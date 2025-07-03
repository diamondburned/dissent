package composer

import (
	"context"
	"io"
	"mime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/autocomplete"
	"github.com/diamondburned/chatkit/md/mdrender"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/utils/osutil"
	"github.com/pkg/errors"
	"libdb.so/dissent/internal/gtkcord"
)

var persistInput = prefs.NewBool(true, prefs.PropMeta{
	Name:    "Persist Input",
	Section: "Composer",
	Description: "Persist the input message between sessions (to disk). " +
		"If disabled, the input is only persisted for the current session on memory.",
})

// InputController is the parent controller that Input controls.
type InputController interface {
	// Send sends or edits everything in the current message buffer state.
	Send()
	// Escape is called when the Escape key is pressed. It is meant to stop any
	// ongoing action and return true, or false if no action.
	Escape() bool
	// EditLastMessage is called when the user wants to edit their last message.
	// False is returned if no messages can be found.
	EditLastMessage() bool
	// PasteClipboardFile is called everytime the user pastes a file from their
	// clipboard. The file is usually (but not always) an image.
	PasteClipboardFile(*File)
	// UpdateMessageLength updates the message length counter.
	UpdateMessageLength(int)
}

// Input is the text field of the composer.
type Input struct {
	*gtk.TextView
	Buffer *gtk.TextBuffer
	ac     *autocomplete.Autocompleter

	ctx     context.Context
	ctrl    InputController
	chID    discord.ChannelID
	guildID discord.GuildID
}

var inputCSS = cssutil.Applier("composer-input", `
	.composer-input,
	.composer-input text {
		background-color: inherit;
	}
	.composer-input {
		padding: 12px 2px;
		margin-top: 0px;
	}
	.composer-input .autocomplete-row label {
		margin: 0;
	}
`)

var inputWYSIWYG = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Rich Preview",
	Section:     "Composer",
	Description: "Enable a semi-WYSIWYG feature that decorates the input Markdown text.",
})

// inputStateKey is the app state that stores the last input message.
var inputStateKey = app.NewStateKey[string]("input-state")

var inputStateMemory sync.Map // map[discord.ChannelID]string

// initializedInput contains a subset of Input.
// This stays here for as long as the dynexport cap on Windows is an issue,
// which should be fixed by Go 1.24.
type initializedInput struct {
	View   *gtk.TextView
	Buffer *gtk.TextBuffer
}

// NewInput creates a new Input widget.
func NewInput(ctx context.Context, ctrl InputController, chID discord.ChannelID) *Input {
	i := Input{
		ctx:  ctx,
		ctrl: ctrl,
		chID: chID,
	}

	inputState := inputStateKey.Acquire(ctx)
	input := initializeInput()

	input.Buffer.ConnectChanged(func() {
		// Do rough WYSIWYG rendering.
		if inputWYSIWYG.Value() {
			mdrender.RenderWYSIWYG(ctx, input.Buffer)
		}

		// Check for message length limit.
		ctrl.UpdateMessageLength(input.Buffer.CharCount())

		// Handle autocompletion.
		i.ac.Autocomplete()

		start, end := i.Buffer.Bounds()

		// Persist input.
		if end.Offset() == 0 {
			if persistInput.Value() {
				inputState.Delete(chID.String())
			} else {
				inputStateMemory.Delete(chID)
			}
		} else {
			text := i.Buffer.Text(start, end, false)
			if persistInput.Value() {
				inputState.Set(chID.String(), text)
			} else {
				inputStateMemory.Store(chID, text)
			}
		}
	})

	i.Buffer = input.Buffer

	i.TextView = input.View
	i.TextView.SetWrapMode(gtk.WrapWordChar)
	i.TextView.SetAcceptsTab(true)
	i.TextView.SetHExpand(true)
	i.TextView.ConnectPasteClipboard(i.readClipboard)
	i.TextView.SetInputHints(0 |
		gtk.InputHintEmoji |
		gtk.InputHintSpellcheck |
		gtk.InputHintWordCompletion |
		gtk.InputHintUppercaseSentences,
	)
	// textutil.SetTabSize(i.TextView)
	inputCSS(i)

	i.ac = autocomplete.New(ctx, i.TextView)
	i.ac.AddSelectedFunc(i.onAutocompleted)
	i.ac.SetCancelOnChange(false)
	i.ac.SetMinLength(2)
	i.ac.SetTimeout(time.Second)

	state := gtkcord.FromContext(ctx)
	if ch, err := state.Cabinet.Channel(chID); err == nil {
		i.guildID = ch.GuildID
		i.ac.Use(
			NewEmojiCompleter(i.guildID), // :
			NewMemberCompleter(chID),     // @
		)
	}

	enterKeyer := gtk.NewEventControllerKey()
	enterKeyer.ConnectKeyPressed(i.onKey)
	i.AddController(enterKeyer)

	inputState.Get(chID.String(), func(text string) {
		i.Buffer.SetText(text)
	})

	return &i
}

// ChannelID returns the channel ID of the channel that this input is in.
func (i *Input) ChannelID() discord.ChannelID {
	return i.chID
}

// GuildID returns the guild ID of the channel that this input is in.
func (i *Input) GuildID() discord.GuildID {
	return i.guildID
}

func (i *Input) onAutocompleted(row autocomplete.SelectedData) bool {
	i.Buffer.BeginUserAction()
	defer i.Buffer.EndUserAction()

	i.Buffer.Delete(row.Bounds[0], row.Bounds[1])

	switch data := row.Data.(type) {
	case EmojiData:
		state := gtkcord.FromContext(i.ctx)
		start, end := i.Buffer.Bounds()

		canUseEmoji := false ||
			// has Nitro so can use anything
			state.EmojiState.HasNitro() ||
			// unicode emoji
			!data.Emoji.ID.IsValid() ||
			// same guild, not animated
			(data.GuildID == i.guildID && !data.Emoji.Animated) ||
			// adding a reaction, so we can't even use URL
			textBufferIsReaction(i.Buffer.Text(start, end, false))

		var content string
		if canUseEmoji {
			// Use the default emoji format. This string is subject to
			// server-side validation.
			content = data.Emoji.String()
		} else {
			// Use the emoji URL instead of the emoji code to allow
			// non-Nitro users to send emojis by sending the image URL.
			content = gtkcord.InjectSizeUnscaled(data.Emoji.EmojiURL(), gtkcord.LargeEmojiSize)
		}

		i.Buffer.Insert(row.Bounds[1], content)
		return true
	case MemberData:
		i.Buffer.Insert(row.Bounds[1], discord.Member(data).Mention())
		return true
	}

	return false
}

var sendOnEnter = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Send Message on Enter",
	Section:     "Composer",
	Description: "Send the message when the user hits the Enter key. Disable this for mobile.",
})

func (i *Input) onKey(val, _ uint, state gdk.ModifierType) bool {
	switch val {
	case gdk.KEY_Return:
		if i.ac.Select() {
			return true
		}

		// TODO: find a better way to do this. goldmark won't try to
		// parse an incomplete codeblock (I think), but the changed
		// signal will be fired after this signal.
		//
		// Perhaps we could use the FindChar method to avoid allocating
		// a new string (twice) on each keypress.
		head := i.Buffer.StartIter()
		tail := i.Buffer.IterAtMark(i.Buffer.GetInsert())
		uinput := i.Buffer.Text(head, tail, false)

		// Check if the number of triple backticks is odd. If it is, then we're
		// in one.
		withinCodeblock := strings.Count(uinput, "```")%2 != 0

		// Enter (without holding Shift) sends the message.
		if sendOnEnter.Value() && !state.Has(gdk.ShiftMask) && !withinCodeblock {
			i.ctrl.Send()
			return true
		}
	case gdk.KEY_Tab:
		return i.ac.Select()
	case gdk.KEY_Escape:
		return i.ctrl.Escape()
	case gdk.KEY_Up:
		if i.ac.MoveUp() {
			return true
		}
		if i.Buffer.CharCount() == 0 {
			return i.ctrl.EditLastMessage()
		}
	case gdk.KEY_Down:
		return i.ac.MoveDown()
	}

	return false
}

func (i *Input) readClipboard() {
	display := gdk.DisplayGetDefault()

	clipboard := display.Clipboard()
	mimeTypes := clipboard.Formats().MIMETypes()

	// Ignore anything text.
	for _, mime := range mimeTypes {
		if mimeIsText(mime) {
			return
		}
	}

	clipboard.ReadAsync(i.ctx, mimeTypes, int(glib.PriorityDefault), func(res gio.AsyncResulter) {
		typ, streamer, err := clipboard.ReadFinish(res)
		if err != nil {
			app.Error(i.ctx, errors.Wrap(err, "failed to read clipboard"))
			return
		}

		gtkutil.Async(i.ctx, func() func() {
			stream := gio.BaseInputStream(streamer)
			reader := gioutil.Reader(i.ctx, stream)
			defer reader.Close()

			f, err := osutil.Consume(reader)
			if err != nil {
				app.Error(i.ctx, errors.Wrap(err, "cannot clone clipboard"))
				return nil
			}

			s, err := f.Stat()
			if err != nil {
				app.Error(i.ctx, errors.Wrap(err, "cannot stat clipboard file"))
				return nil
			}

			// We're too lazy to do reference-counting, so just forbid Open from
			// being called more than once.
			var openedOnce atomic.Bool

			file := &File{
				Name: "clipboard",
				Type: typ,
				Size: s.Size(),
				Open: func() (io.ReadCloser, error) {
					if openedOnce.CompareAndSwap(false, true) {
						return f, nil
					}
					return nil, errors.New("Open called more than once on TempFile")
				},
			}

			if exts, _ := mime.ExtensionsByType(typ); len(exts) > 0 {
				file.Name += exts[0]
			}

			return func() { i.ctrl.PasteClipboardFile(file) }
		})
	})
}
