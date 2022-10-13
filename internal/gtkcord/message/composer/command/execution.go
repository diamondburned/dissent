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

	destroy func()
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
	c.commander.Buffer.SetText("")

	// Write the command into the buffer.
	prefix := "/" + cmd.Name()

	// Make a new prefix marker.
	prefixMark := newImmutableMark(commander.Buffer.StartIter(), func() {
		// Finish destroying the command.
		c.destroy()
	})
	prefixMark.SetContent(prefix)

	c.errorPopover = newErrorPopover()
	c.errorPopover.SetPosition(gtk.PosTop)
	c.errorPopover.SetParent(commander.Input)

	bufferHandle := commander.Buffer.ConnectChanged(func() {
		// Hide the error popover before we do anything.
		c.errorPopover.Hide()

		prefixMark.Validate()
		// Validate all the immutableMarks.
		for i, arg := range c.inputtedArgs {
			if arg.mark.Validate() {
				continue
			}

			// Remove the argument.
			c.inputtedArgs = append(c.inputtedArgs[:i], c.inputtedArgs[i+1:]...)
		}

		// TODO: validate all the arguments.
		for _, arg := range c.inputtedArgs {
			_, err := arg.value()
			if err != nil {
				c.errorPopover.Show(err)
			}
		}
	})

	// Move the cursor to the end of the inserted piece.
	commander.Buffer.PlaceCursor(commander.Buffer.EndIter())

	// Color the command to indicate that we're entering one.
	cmdTag := commandTextTag().FromTable(commander.Buffer.TagTable(), "__command_prefix")
	cmdStart, cmdEnd := prefixMark.Iters()
	commander.Buffer.ApplyTag(cmdTag, cmdStart, cmdEnd)

	// Hook the arguments autocompleter. We'll unhook it once we're done.
	c.argsSearcher = newArgumentsAutocompleter(c.Args, argumentsAutocompleterOpts{
		inhibitors: []func() bool{
			// Inhibit if the cursor is still in the middle of the argument
			// value.
			func() bool { return isInShellWord(cursorText(commander.Buffer)) },
		},
	})
	commander.Autocompleter.Use(c.argsSearcher)

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

		c.commander.Buffer.BeginUserAction()
		defer c.commander.Buffer.EndUserAction()

		// Delete the current text bits.
		c.commander.Buffer.Delete(selection.Bounds[0], selection.Bounds[1])

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
		icon *gtk.Image
		body *gtk.Label
	}
}

var errorPopoverCSS = cssutil.Applier(`command-error-popover`, `

`)

func newErrorPopover() *errorPopover {
	p := errorPopover{}

	p.child.icon = gtk.NewImageFromIconName("dialog-error-symbolic")
	p.child.icon.AddCSSClass("error")
	p.child.icon.SetIconSize(gtk.IconSizeNormal)
	p.child.icon.SetVAlign(gtk.AlignStart)

	p.child.body = gtk.NewLabel("")
	p.child.body.AddCSSClass("command-error-popover-body")
	p.child.body.SetXAlign(0)
	p.child.body.SetWrap(true)
	p.child.body.SetWrapMode(pango.WrapWordChar)

	p.child.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	p.child.Box.Append(p.child.icon)
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

	a.mark = newImmutableMark(iterAt, a.delete)
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
