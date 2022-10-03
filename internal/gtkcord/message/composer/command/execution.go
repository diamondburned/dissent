package command

import (
	"context"

	"github.com/diamondburned/chatkit/components/autocomplete"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/pkg/errors"
)

// CommandExecution handles a single execution of a command. It helps construct
// the values needed to execute a command.
type CommandExecution struct {
	Command
	Args      []Argument
	destroyed func()

	argsSearcher *argumentsAutocompleter
	commander    *InputCommander
	errorPopover *errorPopover
	inputtedArgs []*inputtedArgument

	reservedOffset int
	destroy        func()
}

var _commandTextTag textutil.TextTag

func commandTextTag() textutil.TextTag {
	if _commandTextTag == nil {
		// Return the Discord color as a fallback.
		foreground := "#3f51b5"

		accent, ok := textutil.LookupColor("theme_selected_bg_color")
		if ok {
			foreground = accent.String()
		}

		_commandTextTag = textutil.TextTag{
			"foreground":     foreground,
			"foreground-set": true,
		}
	}

	return _commandTextTag
}

// newCommandExecution creates a new CommandExecution.
func newCommandExecution(commander *InputCommander, cmd Command) *CommandExecution {
	c := CommandExecution{
		Command:   cmd,
		Args:      cmd.Args(),
		commander: commander,
	}

	// Clear the buffer.
	// Write the command into the buffer.
	prefix := "/" + cmd.Name()
	commander.Buffer.SetText(prefix + " ")

	// Make a new prefix marker.
	prefixMark := newImmutableMark(commander.Buffer.StartIter(), "prefix", func() {
		// Invalidate the reservedOffset before we actually delete
		// everything. This avoids an infinite recursion.
		c.reservedOffset = -1
		// Finish destroying the command.
		c.destroy()
	})
	prefixMark.SetContent(prefix)

	// Move the cursor to the end of the inserted piece.
	iter := commander.Buffer.EndIter()
	commander.Buffer.PlaceCursor(iter)

	// Mark the command region. Deleting any of this region will null the entire
	// buffer, so we keep track of this.
	iter.BackwardChar()
	c.reservedOffset = iter.Offset()

	// Color the command to indicate that we're entering one.
	cmdTag := commandTextTag().FromTable(commander.Buffer.TagTable(), "__command_prefix")
	commander.Buffer.ApplyTag(cmdTag, commander.Buffer.StartIter(), iter)

	// Hook the arguments autocompleter. We'll unhook it once we're done.
	c.argsSearcher = newArgumentsAutocompleter(c.Args, argumentsAutocompleterOpts{
		inhibitors: []func() bool{
			// Inhibit if the cursor is still in the middle of the argument
			// value.
			func() bool { return isInShellWord(cursorText(commander.Buffer)) },
		},
	})
	commander.Autocompleter.Use(c.argsSearcher)

	c.errorPopover = newErrorPopover()
	c.errorPopover.SetPosition(gtk.PosTop)
	c.errorPopover.SetParent(commander.Input)

	bufferHandle := commander.Buffer.ConnectChanged(func() {
		// Hide the error popover before we do anything.
		c.errorPopover.Hide()

		// Validate all the immutableMarks.
		prefixMark.Validate()
		for _, arg := range c.inputtedArgs {
			arg.mark.Validate()
		}

		// TODO: validate all the arguments.
		for _, arg := range c.inputtedArgs {
			_, err := arg.value()
			if err != nil {
				c.errorPopover.Show(err)
			}
		}
	})

	c.destroy = func() {
		commander.Autocompleter.Unuse(c.argsSearcher)
		commander.Autocompleter.Autocomplete()
		commander.Buffer.HandlerDisconnect(bufferHandle)
		commander.Buffer.SetText("")
		c.errorPopover.Unparent()
		c.destroyed()
	}

	c.destroyed = func() {
		// Prevent future calls of destroy().
		c.destroy = func() {}
	}

	return &c
}

func (c *CommandExecution) Destroy() {
	c.destroy()
}

func (c *CommandExecution) OnDestroyed(f func()) {
	destroyed := c.destroyed
	c.destroyed = func() {
		destroyed()
		f()
	}
}

func (c *CommandExecution) onAutocompleted(selection autocomplete.SelectedData) bool {
	switch data := selection.Data.(type) {
	case autocompletedArgData:
		for _, inputted := range c.inputtedArgs {
			if inputted.arg == data.arg {
				panic("bug: user chose an argument that's already inputted")
			}
		}

		inputtedArg := newInputtedArgument(c, data.arg, c.commander.Buffer.EndIter())
		c.inputtedArgs = append(c.inputtedArgs, inputtedArg)

		c.argsSearcher.setDone(data.arg, true)
		return true
	}

	return false
}

func (c *CommandExecution) exec() {
	if err := c.tryExec(c.commander.Context()); err != nil {
		c.errorPopover.Show(err)
	}
}

func (c *CommandExecution) tryExec(ctx context.Context) error {
	argValues := make(ArgValues, len(c.inputtedArgs))

	for _, inputted := range c.inputtedArgs {
		v, err := inputted.value()
		if err != nil {
			return err
		}

		argValues[inputted.arg.Name()] = v
	}

	if err := AssertArgValues(ctx, c.Args, argValues); err != nil {
		return err
	}

	panic("TODO")
}

func cursorText(buffer *gtk.TextBuffer) string {
	cursorOffset := buffer.ObjectProperty("cursor-position").(int)
	return buffer.Text(buffer.IterAtOffset(cursorOffset), buffer.EndIter(), false)
}

type errorPopover struct {
	*gtk.Popover
	child struct {
		*gtk.Box
		head struct {
			*gtk.Box
			icon *gtk.Image
			text *gtk.Label
		}
		body *gtk.Label
	}
}

var errorPopoverCSS = cssutil.Applier(`command-error-popover`, `

`)

func newErrorPopover() *errorPopover {
	p := errorPopover{}

	p.child.head.icon = gtk.NewImageFromIconName("dialog-error-symbolic")
	p.child.head.icon.AddCSSClass("error")
	p.child.head.icon.SetIconSize(gtk.IconSizeNormal)

	p.child.head.text = gtk.NewLabel("Error!")
	p.child.head.text.AddCSSClass("error")

	p.child.head.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	p.child.head.Box.AddCSSClass("command-error-popover-head")
	p.child.head.Box.Append(p.child.head.icon)
	p.child.head.Box.Append(p.child.head.text)

	p.child.body = gtk.NewLabel("")
	p.child.body.AddCSSClass("command-error-popover-body")
	p.child.body.SetXAlign(0)
	p.child.body.SetWrap(true)
	p.child.body.SetWrapMode(pango.WrapWordChar)

	p.child.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	p.child.Box.Append(p.child.head)
	p.child.Box.Append(p.child.body)

	p.Popover = gtk.NewPopover()
	p.Popover.SetAutohide(false)
	p.Popover.SetChild(p.child)
	p.Popover.SetSizeRequest(300, -1)
	errorPopoverCSS(p)

	return &p
}

func (p *errorPopover) Show(err error) {
	p.child.body.SetText(err.Error())
	p.Popover.Popup()
}

func (p *errorPopover) Hide() {
	p.Popover.Popdown()
}

type inputtedArgument struct {
	arg  Argument
	mark *immutableMark
}

func newInputtedArgument(execution *CommandExecution, arg Argument, iterAt *gtk.TextIter) *inputtedArgument {
	a := inputtedArgument{}
	a.arg = arg

	a.mark = newImmutableMark(iterAt, "arg_"+arg.Name(), a.delete)
	a.mark.SetContent(arg.Name() + ":")

	// Replace the iterator with the one at the end of the mark.
	_, iterAt = a.mark.Iters()

	// Insert the two quotes to assist the user and place the cursor in the
	// middle. This isn't really needed syntax-wise, but it's a nice touch.
	execution.commander.Buffer.Insert(iterAt, `""`)

	// Move the iterator to the middle of the quotes.
	iterAt.BackwardChar()
	// Place the cursor right in the middle.
	execution.commander.Buffer.PlaceCursor(iterAt)

	return &a
}

func (a *inputtedArgument) delete() {}

func (a *inputtedArgument) value() (string, error) {
	// The end of the mark is the start of the value.
	_, start := a.mark.Iters()

	buffer := start.Buffer()
	end := buffer.EndIter()

	// Grab from the start to the end of the buffer.
	text := buffer.Text(start, end, false)

	v, err := parseSingleWord(text)
	if err != nil {
		return v, errors.Wrapf(err, "failed to parse value for %q", a.arg.Name())
	}

	return v, nil
}
