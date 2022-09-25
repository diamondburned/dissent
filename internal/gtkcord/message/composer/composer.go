package composer

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/chatkit/components/author"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/mediautil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/pkg/errors"
)

var showAllEmojis = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Show All Emojis",
	Section:     "Discord",
	Description: "Show (and autocomplete) all emojis even if the user doesn't have Nitro.",
})

// File contains the filename and a callback to open the file that's called
// asynchronously.
type File struct {
	Name string
	Type string // MIME type
	Size int64
	Open func() (io.ReadCloser, error)
}

// SendingMessage is the message created to be sent.
type SendingMessage struct {
	Content    string
	Files      []File
	ReplyingTo discord.MessageID
}

// Controller is the parent Controller for a View.
type Controller interface {
	SendMessage(SendingMessage)
	StopEditing()
	StopReplying()
	EditLastMessage() bool
}

type typer struct {
	Markup string
	UserID discord.UserID
	Time   discord.UnixTimestamp
}

const typerTimeout = 10 * time.Second

type View struct {
	*gtk.Box
	Action struct {
		*gtk.Button
		current func()
	}
	Input       *Input
	Placeholder *gtk.Label
	UploadTray  *UploadTray
	Send        *gtk.Button

	ctx  context.Context
	ctrl Controller
	chID discord.ChannelID

	typers        []typer
	typingHandler glib.SourceHandle

	state struct {
		id       discord.MessageID
		editing  bool
		replying bool
	}
}

var viewCSS = cssutil.Applier("composer-view", `
	.composer-action {
		border:  none;
		margin:  0 11px;
		padding: 6px;
	}
	.composer-send {
		margin:  0px;
		padding: 0px 10px;
		border-radius: 0;
		min-height: 0;
		min-width:  0;
	}
	.composer-placeholder {
		padding: 16px 2px;
		color: alpha(@theme_fg_color, 0.65);
	}
`)

const (
	sendIcon   = "document-send-symbolic"
	editIcon   = "document-edit-symbolic"
	stopIcon   = "edit-clear-all-symbolic"
	replyIcon  = "mail-reply-sender-symbolic"
	uploadIcon = "list-add-symbolic"
)

func NewView(ctx context.Context, ctrl Controller, chID discord.ChannelID) *View {
	v := &View{
		ctx:  ctx,
		ctrl: ctrl,
		chID: chID,
	}

	v.Input = NewInput(ctx, inputControllerView{v}, chID)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetPropagateNaturalHeight(true)
	scroll.SetMaxContentHeight(500)
	scroll.SetChild(v.Input)

	v.Placeholder = gtk.NewLabel("")
	v.Placeholder.AddCSSClass("composer-placeholder")
	v.Placeholder.SetVAlign(gtk.AlignStart)
	v.Placeholder.SetHAlign(gtk.AlignStart)
	v.Placeholder.SetEllipsize(pango.EllipsizeEnd)

	revealer := gtk.NewRevealer()
	revealer.SetChild(v.Placeholder)
	revealer.SetCanTarget(false)
	revealer.SetRevealChild(true)
	revealer.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	revealer.SetTransitionDuration(75)

	overlay := gtk.NewOverlay()
	overlay.AddCSSClass("composer-placeholder-overlay")
	overlay.SetChild(scroll)
	overlay.AddOverlay(revealer)
	overlay.SetClipOverlay(revealer, true)

	// Show or hide the placeholder when the buffer is empty or not.
	v.Input.Buffer.ConnectChanged(func() {
		start, end := v.Input.Buffer.Bounds()
		// Reveal if the buffer has 0 length.
		revealer.SetRevealChild(start.Offset() == end.Offset())
	})

	v.UploadTray = NewUploadTray()

	middle := gtk.NewBox(gtk.OrientationVertical, 0)
	middle.Append(overlay)
	middle.Append(v.UploadTray)

	v.Action.Button = gtk.NewButton()
	v.Action.AddCSSClass("composer-action")
	v.Action.SetHasFrame(false)
	v.Action.SetHAlign(gtk.AlignCenter)
	v.Action.SetVAlign(gtk.AlignCenter)

	v.Action.ConnectClicked(func() { v.Action.current() })
	v.resetAction()

	v.Send = gtk.NewButtonFromIconName(sendIcon)
	v.Send.AddCSSClass("composer-send")
	v.Send.SetHasFrame(false)
	v.Send.ConnectClicked(v.send)

	v.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	v.Box.SetVAlign(gtk.AlignEnd)
	v.Box.Append(v.Action)
	v.Box.Append(middle)
	v.Box.Append(v.Send)

	v.SetPlaceholderMarkup("")

	state := gtkcord.FromContext(ctx)
	state.BindHandler(gtkutil.WithVisibility(ctx, v),
		func(ev gateway.Event) {
			switch ev := ev.(type) {
			case *gateway.TypingStartEvent:
				if ev.ChannelID == chID {
					v.addTyper(ev)
				}
			case *gateway.MessageCreateEvent:
				if ev.ChannelID == chID {
					v.removeTyper(ev.Author.ID)
				}
			}
		},
		(*gateway.TypingStartEvent)(nil),
		(*gateway.MessageCreateEvent)(nil),
	)

	viewCSS(v)
	return v
}

// SetPlaceholder sets the composer's placeholder. The default is used if an
// empty string is given.
func (v *View) SetPlaceholderMarkup(markup string) {
	if markup == "" {
		v.ResetPlaceholder()
		return
	}

	v.Placeholder.SetMarkup(markup)
}

func (v *View) ResetPlaceholder() {
	if len(v.typers) == 0 {
		v.Placeholder.SetText("Message " + gtkcord.ChannelNameFromID(v.ctx, v.chID))
		return
	}

	var typers string
	switch len(v.typers) {
	case 1:
		typers = v.typers[0].Markup + " is typing..."
	case 2:
		typers = v.typers[0].Markup + " and " +
			v.typers[1].Markup + " are typing..."
	case 3:
		typers = v.typers[0].Markup + ", " +
			v.typers[1].Markup + " and " +
			v.typers[2].Markup + " are typing..."
	default:
		typers = "Several people are typing..."
	}
	v.Placeholder.SetMarkup(typers)
}

// actionData is the data that the action button in the composer bar is
// currently doing.
type actionData struct {
	Name string
	Icon string
	Func func()
}

// setAction sets the action of the button in the composer.
func (v *View) setAction(action actionData) {
	v.Action.SetSensitive(action.Func != nil)
	v.Action.SetIconName(action.Icon)
	v.Action.SetTooltipText(action.Name)
	v.Action.current = action.Func
}

func (v *View) resetAction() {
	v.setAction(actionData{
		Name: "Upload File",
		Icon: uploadIcon,
		Func: v.upload,
	})
}

func (v *View) upload() {
	chooser := gtk.NewFileChooserNative(
		"Upload Files",
		app.GTKWindowFromContext(v.ctx),
		gtk.FileChooserActionOpen,
		"Upload", "Cancel",
	)
	chooser.SetSelectMultiple(true)
	chooser.ConnectResponse(func(resp int) {
		if resp == int(gtk.ResponseAccept) {
			v.addFiles(chooser.Files())
		}
	})
	chooser.Show()
}

func (v *View) addFiles(list gio.ListModeller) {
	go func() {
		var i uint
		for v.ctx.Err() == nil {
			obj := list.Item(i)
			if obj == nil {
				break
			}

			file := obj.Cast().(gio.Filer)
			path := file.Path()

			f := File{
				Name: file.Basename(),
				Type: mediautil.FileMIME(v.ctx, file),
				Size: mediautil.FileSize(v.ctx, file),
			}

			if path != "" {
				f.Open = func() (io.ReadCloser, error) {
					return os.Open(path)
				}
			} else {
				f.Open = func() (io.ReadCloser, error) {
					r, err := file.Read(v.ctx)
					if err != nil {
						return nil, err
					}
					return gioutil.Reader(v.ctx, r), nil
				}
			}

			glib.IdleAdd(func() { v.UploadTray.AddFile(f) })
			i++
		}
	}()
}

func (v *View) commit() (string, []File) {
	start, end := v.Input.Buffer.Bounds()
	text := v.Input.Buffer.Text(start, end, false)
	v.Input.Buffer.Delete(start, end)

	files := v.UploadTray.Clear()

	return text, files
}

func (v *View) send() {
	if v.state.editing {
		v.edit()
		return
	}

	text, files := v.commit()
	if text == "" && len(files) == 0 {
		return
	}

	v.ctrl.SendMessage(SendingMessage{
		Content:    text,
		Files:      files,
		ReplyingTo: v.state.id,
	})

	if v.state.replying {
		v.ctrl.StopReplying()
	}
}

func (v *View) edit() {
	editingID := v.state.id
	text, _ := v.commit()

	state := gtkcord.FromContext(v.ctx)

	go func() {
		_, err := state.EditMessage(v.chID, editingID, text)
		if err != nil {
			app.Error(v.ctx, errors.Wrap(err, "cannot edit message"))
		}
	}()

	v.ctrl.StopEditing()
}

// StartEditing starts editing the given message. The message is edited once the
// user hits send.
func (v *View) StartEditing(msg *discord.Message) {
	v.restart()

	v.state.id = msg.ID
	v.state.editing = true

	v.Input.Buffer.SetText(msg.Content)
	v.Send.SetIconName(editIcon)
	v.SetPlaceholderMarkup("Editing message")
	v.AddCSSClass("composer-editing")
	v.setAction(actionData{
		Name: "Stop Editing",
		Icon: stopIcon,
		Func: v.ctrl.StopEditing,
	})
}

// StopEditing stops editing.
func (v *View) StopEditing() {
	if !v.state.editing {
		return
	}

	v.state.id = 0
	v.state.editing = false

	v.Send.SetIconName(sendIcon)
	v.SetPlaceholderMarkup("")
	v.RemoveCSSClass("composer-editing")
	v.resetAction()
}

// StartReplyingTo starts replying to the given message. Visually, there is no
// difference except for the send button being different.
func (v *View) StartReplyingTo(msg *discord.Message) {
	v.restart()

	v.state.id = msg.ID
	v.state.replying = true

	v.Send.SetIconName(replyIcon)
	v.AddCSSClass("composer-replying")

	state := gtkcord.FromContext(v.ctx)
	v.SetPlaceholderMarkup(fmt.Sprintf(
		"Replying to %s",
		state.AuthorMarkup(&gateway.MessageCreateEvent{Message: *msg}),
	))
}

// StopReplying undoes the start call.
func (v *View) StopReplying() {
	if !v.state.replying {
		return
	}

	v.state.id = 0
	v.state.replying = false

	v.Send.SetIconName(sendIcon)
	v.SetPlaceholderMarkup("")
	v.RemoveCSSClass("composer-replying")
}

func (v *View) restart() bool {
	state := v.state

	if v.state.editing {
		v.ctrl.StopEditing()
	}

	if v.state.replying {
		v.ctrl.StopReplying()
	}

	return state.editing || state.replying
}

func (v *View) addTyper(ev *gateway.TypingStartEvent) {
	for i, typer := range v.typers {
		if typer.UserID == ev.UserID {
			v.typers[i].Time = ev.Timestamp
			goto update
		}
	}

	{
		state := gtkcord.FromContext(v.ctx)
		mods := []author.MarkupMod{author.WithMinimal()}
		var markup string

		if ev.Member != nil {
			markup = state.MemberMarkup(ev.GuildID, &discord.GuildUser{
				User:   ev.Member.User,
				Member: ev.Member,
			}, mods...)
		} else {
			markup = state.UserIDMarkup(ev.ChannelID, ev.UserID, mods...)
		}

		v.typers = append(v.typers, typer{
			Markup: markup,
			UserID: ev.UserID,
			Time:   ev.Timestamp,
		})
	}

update:
	sort.Slice(v.typers, func(i, j int) bool {
		return v.typers[i].Time < v.typers[j].Time
	})

	v.ResetPlaceholder()

	if v.typingHandler == 0 {
		v.typingHandler = glib.TimeoutSecondsAdd(1, func() bool {
			v.cleanupTypers()

			if len(v.typers) == 0 {
				v.typingHandler = 0
				return false
			}

			return true
		})
	}
}

func (v *View) removeTyper(uID discord.UserID) {
	for i, typer := range v.typers {
		if typer.UserID == uID {
			v.typers = append(v.typers[:i], v.typers[i+1:]...)
			v.ResetPlaceholder()
			return
		}
	}
}

func (v *View) cleanupTypers() {
	createdTime := discord.UnixTimestamp(time.Now().Add(-typerTimeout).Unix())

	typers := v.typers[:0]
	for _, typer := range v.typers {
		if typer.Time > createdTime {
			typers = append(typers, typer)
		}
	}

	v.typers = typers
	v.ResetPlaceholder()
}

// inputControllerView implements InputController.
type inputControllerView struct {
	*View
}

func (v inputControllerView) Send()        { v.send() }
func (v inputControllerView) Escape() bool { return v.restart() }

func (v inputControllerView) EditLastMessage() bool {
	return v.ctrl.EditLastMessage()
}

func (v inputControllerView) PasteClipboardFile(file File) {
	v.UploadTray.AddFile(file)
}
