package command

import (
	"context"

	"github.com/diamondburned/chatkit/components/autocomplete"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// InputCommander is an instance that hooks into an input to help executing
// commands.
type InputCommander struct {
	*Registry
	ctx context.Context

	Input         *gtk.TextView
	Buffer        *gtk.TextBuffer
	Autocompleter *autocomplete.Autocompleter

	// current is not nil if we're currently executing a command.
	current *CommandExecution
}

// NewInputCommander creates an InputCommander for the given input. The input is
// hooked into the returned commander to autocomplete commands and its
// arguments.
func NewInputCommander(ctx context.Context, registry *Registry, input *gtk.TextView, autocompleter *autocomplete.Autocompleter) *InputCommander {
	c := InputCommander{
		Registry:      registry,
		ctx:           ctx,
		Input:         input,
		Buffer:        input.Buffer(),
		Autocompleter: autocompleter,
	}

	c.Autocompleter.AddSelectedFunc(c.onAutocompleted)
	c.Autocompleter.Use(newCommandSearcher(c.commands))

	return &c
}

// ConnectKeyController hooks the given keyboard controller to the input.
func (c *InputCommander) ConnectKeyController(controller *gtk.EventControllerKey) {
	controller.ConnectKeyPressed(func(val, _ uint, state gdk.ModifierType) bool {
		switch val {
		case gdk.KEY_Return, gdk.KEY_KP_Enter:
			if c.Autocompleter.Select() {
				return true
			}

			if c.current != nil {
				c.current.exec()
				return true
			}
		}

		return false
	})
}

// Context returns the context that was passed to NewInputCommander.
func (c *InputCommander) Context() context.Context {
	return c.ctx
}

func (c *InputCommander) onAutocompleted(selection autocomplete.SelectedData) bool {
	switch data := selection.Data.(type) {
	case autocompletedCommandData:
		// Just in case.
		c.Buffer.BeginIrreversibleAction()
		defer c.Buffer.EndIrreversibleAction()

		if c.current != nil {
			c.current.Destroy()
		}

		c.current = newCommandExecution(c, data.cmd)
		c.current.OnDestroyed(func() { c.current = nil })

		// Trigger an autocompletion update to show the arguments.
		glib.IdleAdd(func() { c.Autocompleter.Autocomplete() })
	}

	if c.current != nil {
		return c.current.onAutocompleted(selection)
	}

	return false
}
