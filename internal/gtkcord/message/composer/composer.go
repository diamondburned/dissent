package composer

import (
	"context"
	"io"
	"os"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/kits/mediautil"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/pkg/errors"
)

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

type View struct {
	*gtk.Box
	Action struct {
		*gtk.Button
		current func()
	}
	Input      *Input
	UploadTray *UploadTray
	Send       *gtk.Button

	ctx  context.Context
	ctrl Controller
	chID discord.ChannelID

	state struct {
		id       discord.MessageID
		editing  bool
		replying bool
	}
}

var viewCSS = cssutil.Applier("composer-view", `
	.composer-action {
		margin:  0 11px;
		padding: 6px;
	}
	.composer-send {
		margin:   0px;
		padding: 10px;
		border-radius: 0;
		min-height: 0;
		min-width:  0;
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
	v := View{
		ctx:  ctx,
		ctrl: ctrl,
		chID: chID,
	}

	v.Input = NewInput(ctx, inputControllerView{&v}, chID)
	v.Input.SetVExpand(true)

	v.UploadTray = NewUploadTray()

	middle := gtk.NewBox(gtk.OrientationVertical, 0)
	middle.Append(v.Input)
	middle.Append(v.UploadTray)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetMaxContentHeight(450)
	scroll.SetMinContentHeight(50)
	scroll.SetPropagateNaturalWidth(true)
	scroll.SetPropagateNaturalHeight(true)
	scroll.SetChild(middle)

	v.Input.SetVAdjustment(scroll.VAdjustment())
	v.Input.SetHAdjustment(scroll.HAdjustment())

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
	v.Box.Append(v.Action)
	v.Box.Append(scroll)
	v.Box.Append(v.Send)

	viewCSS(v)
	return &v
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
}

// StopReplying undoes the start call.
func (v *View) StopReplying() {
	if !v.state.replying {
		return
	}

	v.state.id = 0
	v.state.replying = false

	v.Send.SetIconName(sendIcon)
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
