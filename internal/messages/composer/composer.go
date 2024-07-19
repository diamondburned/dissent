package composer

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/mediautil"
	"github.com/pkg/errors"
	"libdb.so/dissent/internal/gtkcord"
)

var showAllEmojis = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Show All Emojis",
	Section:     "Composer",
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
	Content      string
	Files        []File
	ReplyingTo   discord.MessageID
	ReplyMention bool
}

// Controller is the parent Controller for a View.
type Controller interface {
	SendMessage(SendingMessage)
	StopEditing()
	StopReplying()
	EditLastMessage() bool
	AddReaction(discord.MessageID, discord.APIEmoji)
	AddToast(*adw.Toast)
}

type typer struct {
	Markup string
	UserID discord.UserID
	Time   discord.UnixTimestamp
}

func findTyper(typers []typer, userID discord.UserID) *typer {
	for i, t := range typers {
		if t.UserID == userID {
			return &typers[i]
		}
	}
	return nil
}

const typerTimeout = 10 * time.Second

type replyingState uint8

const (
	notReplying replyingState = iota
	replyingMention
	replyingNoMention
)

type View struct {
	*gtk.Box
	Input        *Input
	Placeholder  *gtk.Label
	UploadTray   *UploadTray
	EmojiChooser *gtk.EmojiChooser

	ctx  context.Context
	ctrl Controller
	chID discord.ChannelID

	rightBox    *gtk.Box
	emojiButton *gtk.MenuButton
	sendButton  *gtk.Button

	leftBox      *gtk.Box
	uploadButton *gtk.Button

	chooser *gtk.FileChooserNative

	state struct {
		id       discord.MessageID
		editing  bool
		replying replyingState
	}
}

var viewCSS = cssutil.Applier("composer-view", `
	.composer-view * {
		/* Fix spacing for certain GTK themes such as stock Adwaita. */
		min-height: 0;
	}
	.composer-left-actions {
		margin: 0 4px 0 11px;
	}
	.composer-left-actions > *:not(:first-child) {
		margin-right: 4px;
	}
	.composer-right-actions button.toggle:checked {
		background-color: alpha(@accent_color, 0.25);
		color: @accent_color;
	}
	.composer-right-actions {
		margin: 0 11px 0 0;
	}
	.composer-right-actions > *:not(:first-child) {
		margin-left: 4px;
	}
	.composer-placeholder {
		padding: 12px 2px;
		color: alpha(@theme_fg_color, 0.65);
	}
`)

const (
	sendIcon   = "paper-plane-symbolic"
	emojiIcon  = "sentiment-satisfied-symbolic"
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
	scroll.SetMaxContentHeight(1000)
	scroll.SetChild(v.Input)

	v.Placeholder = gtk.NewLabel("")
	v.Placeholder.AddCSSClass("composer-placeholder")
	v.Placeholder.SetVAlign(gtk.AlignStart)
	v.Placeholder.SetHAlign(gtk.AlignFill)
	v.Placeholder.SetXAlign(0)
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
	updatePlaceholderVisibility := func() {
		start, end := v.Input.Buffer.Bounds()
		// Reveal if the buffer has 0 length.
		revealer.SetRevealChild(start.Offset() == end.Offset())
	}
	v.Input.Buffer.ConnectChanged(updatePlaceholderVisibility)
	updatePlaceholderVisibility()

	v.UploadTray = NewUploadTray()

	middle := gtk.NewBox(gtk.OrientationVertical, 0)
	middle.Append(overlay)
	middle.Append(v.UploadTray)

	v.uploadButton = newActionButton(actionButtonData{
		Name: "Upload File",
		Icon: uploadIcon,
		Func: v.upload,
	})

	v.leftBox = gtk.NewBox(gtk.OrientationHorizontal, 0)
	v.leftBox.AddCSSClass("composer-left-actions")

	v.EmojiChooser = gtk.NewEmojiChooser()
	v.EmojiChooser.ConnectEmojiPicked(func(emoji string) { v.insertEmoji(emoji) })

	v.emojiButton = gtk.NewMenuButton()
	v.emojiButton.SetIconName(emojiIcon)
	v.emojiButton.AddCSSClass("flat")
	v.emojiButton.SetVAlign(gtk.AlignCenter)
	v.emojiButton.SetTooltipText("Choose Emoji")
	v.emojiButton.SetPopover(v.EmojiChooser)

	v.sendButton = gtk.NewButtonFromIconName(sendIcon)
	v.sendButton.AddCSSClass("composer-send")
	v.sendButton.SetVAlign(gtk.AlignCenter)
	v.sendButton.SetTooltipText("Send Message")
	v.sendButton.SetHasFrame(false)
	v.sendButton.ConnectClicked(v.send)

	v.rightBox = gtk.NewBox(gtk.OrientationHorizontal, 0)
	v.rightBox.AddCSSClass("composer-right-actions")
	v.rightBox.SetHAlign(gtk.AlignEnd)

	v.resetAction()

	v.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	v.Box.SetVAlign(gtk.AlignEnd)
	v.Box.Append(v.leftBox)
	v.Box.Append(middle)
	v.Box.Append(v.rightBox)

	v.SetPlaceholderMarkup("")

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
	v.Placeholder.SetText("Message " + gtkcord.ChannelNameFromID(v.ctx, v.chID))
}

// actionButton is a button that is used in the composer bar.
type actionButton interface {
	newButton() gtk.Widgetter
}

// existingActionButton is a button that already exists in the composer bar.
type existingActionButton struct{ gtk.Widgetter }

func (a existingActionButton) newButton() gtk.Widgetter { return a }

// actionButtonData is the data that the action button in the composer bar is
// currently doing.
type actionButtonData struct {
	Name locale.Localized
	Icon string
	Func func()
}

func newActionButton(a actionButtonData) *gtk.Button {
	button := gtk.NewButton()
	button.AddCSSClass("composer-action")
	button.SetHasFrame(false)
	button.SetHAlign(gtk.AlignCenter)
	button.SetVAlign(gtk.AlignCenter)
	button.SetSensitive(a.Func != nil)
	button.SetIconName(a.Icon)
	button.SetTooltipText(a.Name.String())
	button.ConnectClicked(func() { a.Func() })

	return button
}

func (a actionButtonData) newButton() gtk.Widgetter {
	return newActionButton(a)
}

type actions struct {
	left  []actionButton
	right []actionButton
}

// setAction sets the action of the button in the composer.
func (v *View) setActions(actions actions) {
	gtkutil.RemoveChildren(v.leftBox)
	gtkutil.RemoveChildren(v.rightBox)

	for _, a := range actions.left {
		v.leftBox.Append(a.newButton())
	}
	for _, a := range actions.right {
		v.rightBox.Append(a.newButton())
	}
}

func (v *View) resetAction() {
	v.setActions(actions{
		left:  []actionButton{existingActionButton{v.uploadButton}},
		right: []actionButton{existingActionButton{v.emojiButton}, existingActionButton{v.sendButton}},
	})
}

func (v *View) upload() {
	// From GTK's documentation:
	//   Note that unlike GtkDialog, GtkNativeDialog objects are not toplevel
	//   widgets, and GTK does not keep them alive. It is your responsibility to
	//   keep a reference until you are done with the object.
	v.chooser = gtk.NewFileChooserNative(
		"Upload Files",
		app.GTKWindowFromContext(v.ctx),
		gtk.FileChooserActionOpen,
		"Upload", "Cancel",
	)
	v.chooser.SetSelectMultiple(true)
	v.chooser.SetModal(true)
	v.chooser.ConnectResponse(func(resp int) {
		if resp == int(gtk.ResponseAccept) {
			v.addFiles(v.chooser.Files())
		}
		v.chooser.Destroy()
		v.chooser = nil
	})
	v.chooser.Show()
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

func (v *View) peekContent() (string, []File) {
	start, end := v.Input.Buffer.Bounds()
	text := v.Input.Buffer.Text(start, end, false)
	files := v.UploadTray.Files()
	return text, files
}

func (v *View) commitContent() (string, []File) {
	start, end := v.Input.Buffer.Bounds()
	text := v.Input.Buffer.Text(start, end, false)
	v.Input.Buffer.Delete(start, end)
	files := v.UploadTray.Clear()
	return text, files
}

func (v *View) insertEmoji(emoji string) {
	endIter := v.Input.Buffer.EndIter()
	v.Input.Buffer.Insert(endIter, emoji)
}

func (v *View) send() {
	if v.state.editing {
		v.edit()
		return
	}

	text, files := v.commitContent()
	if text == "" && len(files) == 0 {
		return
	}

	if len(files) == 0 && textBufferIsReaction(text) {
		state := gtkcord.FromContext(v.ctx).Online()

		var targetMessageID discord.MessageID
		if v.state.replying != notReplying {
			targetMessageID = v.state.id
		} else {
			msgs, _ := state.Cabinet.Messages(v.chID)
			if len(msgs) > 0 {
				targetMessageID = msgs[0].ID
			}
		}

		if targetMessageID.IsValid() {
			text = strings.TrimPrefix(text, "+")
			text = strings.TrimSpace(text)
			text = strings.Trim(text, "<>")

			state := gtkcord.FromContext(v.ctx).Online()
			emoji := discord.APIEmoji(text)
			chID := v.chID
			go func() {
				if err := state.React(chID, targetMessageID, emoji); err != nil {
					slog.Error(
						"cannot react to message",
						"channel", chID,
						"message", targetMessageID,
						"emoji", emoji,
						"err", err)
					app.Error(v.ctx, errors.Wrap(err, "cannot react to message"))
				}
			}()

			v.ctrl.StopReplying()
			return
		}
	}

	v.ctrl.SendMessage(SendingMessage{
		Content:      text,
		Files:        files,
		ReplyingTo:   v.state.id,
		ReplyMention: v.state.replying == replyingMention,
	})

	if v.state.replying != notReplying {
		v.ctrl.StopReplying()
	}
}

// textBufferIsReaction returns whether the text buffer is for adding a reaction.
// It is true if the input matches something like "+<emoji>".
func textBufferIsReaction(buffer string) bool {
	buffer = strings.TrimRightFunc(buffer, unicode.IsSpace)
	return strings.HasPrefix(buffer, "+") && !strings.ContainsFunc(buffer, unicode.IsSpace)
}

func (v *View) edit() {
	editingID := v.state.id
	text, _ := v.commitContent()

	state := gtkcord.FromContext(v.ctx).Online()

	gtkutil.Async(v.ctx, func() func() {
		_, err := state.EditMessage(v.chID, editingID, text)
		if err != nil {
			err = errors.Wrap(err, "cannot edit message")
			log.Println()
			return func() {
				toast := adw.NewToast(locale.Get("Cannot edit message"))
				toast.SetTimeout(0)
				toast.SetButtonLabel(locale.Get("Logs"))
				toast.SetActionName("app.logs")
				v.ctrl.AddToast(toast)
			}
		}
		return nil
	})

	v.ctrl.StopEditing()
}

// StartEditing starts editing the given message. The message is edited once the
// user hits send.
func (v *View) StartEditing(msg *discord.Message) {
	v.restart()

	v.state.id = msg.ID
	v.state.editing = true

	v.Input.Buffer.SetText(msg.Content)
	v.SetPlaceholderMarkup(locale.Get("Editing message"))
	v.AddCSSClass("composer-editing")
	v.setActions(actions{
		left: []actionButton{
			actionButtonData{
				Name: "Stop Editing",
				Icon: stopIcon,
				Func: v.ctrl.StopEditing,
			},
		},
		right: []actionButton{
			actionButtonData{
				Name: "Edit",
				Icon: editIcon,
				Func: v.edit,
			},
		},
	})
}

// StopEditing stops editing.
func (v *View) StopEditing() {
	if !v.state.editing {
		return
	}

	v.state.id = 0
	v.state.editing = false
	start, end := v.Input.Buffer.Bounds()
	v.Input.Buffer.Delete(start, end)

	v.SetPlaceholderMarkup("")
	v.RemoveCSSClass("composer-editing")
	v.resetAction()
}

// StartReplyingTo starts replying to the given message. Visually, there is no
// difference except for the send button being different.
func (v *View) StartReplyingTo(msg *discord.Message) {
	v.restart()

	v.state.id = msg.ID
	v.state.replying = replyingMention

	v.AddCSSClass("composer-replying")

	state := gtkcord.FromContext(v.ctx)
	v.SetPlaceholderMarkup(fmt.Sprintf(
		"Replying to %s",
		state.AuthorMarkup(&gateway.MessageCreateEvent{Message: *msg}),
	))

	mentionToggle := gtk.NewToggleButton()
	mentionToggle.AddCSSClass("composer-mention-toggle")
	mentionToggle.SetIconName("online-symbolic")
	mentionToggle.SetHasFrame(false)
	mentionToggle.SetActive(true)
	mentionToggle.SetHAlign(gtk.AlignCenter)
	mentionToggle.SetVAlign(gtk.AlignCenter)
	mentionToggle.ConnectToggled(func() {
		if mentionToggle.Active() {
			v.state.replying = replyingMention
		} else {
			v.state.replying = replyingNoMention
		}
	})

	v.setActions(actions{
		left: []actionButton{
			existingActionButton{v.uploadButton},
		},
		right: []actionButton{
			existingActionButton{v.emojiButton},
			existingActionButton{mentionToggle},
			actionButtonData{
				Name: "Reply",
				Icon: replyIcon,
				Func: v.send,
			},
		},
	})
}

// StopReplying undoes the start call.
func (v *View) StopReplying() {
	if v.state.replying == 0 {
		return
	}

	v.state.id = 0
	v.state.replying = 0

	v.SetPlaceholderMarkup("")
	v.RemoveCSSClass("composer-replying")
	v.resetAction()
}

func (v *View) restart() bool {
	state := v.state

	if v.state.editing {
		v.ctrl.StopEditing()
	}
	if v.state.replying != notReplying {
		v.ctrl.StopReplying()
	}

	return state.editing || state.replying != notReplying
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
