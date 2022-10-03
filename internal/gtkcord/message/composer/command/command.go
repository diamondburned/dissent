package command

import (
	"context"
	"fmt"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/autocomplete"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

type any = interface{}

type ctxKey uint8

const (
	_ ctxKey = iota
	channelStateCtx
)

// ChannelState is a stateful struct that holds the current channel and guild.
type ChannelState struct {
	*gtkcord.State
	ChannelID discord.ChannelID
}

// RequireChannelState returns a ChannelState or panic if the context doesn't
// have it.
func RequireChannelState(ctx context.Context) *ChannelState {
	state := ChannelStateFromContext(ctx)
	if state == nil {
		panic("BUG: missing channel state")
	}
	return state
}

// ChannelStateFromContext returns the ChannelState from the given context.
func ChannelStateFromContext(ctx context.Context) *ChannelState {
	v, _ := ctx.Value(channelStateCtx).(*ChannelState)
	return v
}

// WithChannelState injects the given ChannelState into the given context.
func WithChannelState(ctx context.Context, state *ChannelState) context.Context {
	return context.WithValue(ctx, channelStateCtx, state)
}

// Command describes a command.
type Command interface {
	Name() string
	Desc() string
	Args() []Argument
	Exec(ctx context.Context, args ArgValues) error
}

// BotCommand: TODO.
type BotCommand struct {
	Command
	BotID   discord.UserID
	GuildID discord.GuildID
}

// ArgValues is a map of argument names to its values.
type ArgValues map[string]string

// AssertArgValues asserts that the given ArgValues satisfies the declared
// arguments.
func AssertArgValues(ctx context.Context, args []Argument, values ArgValues) error {
	for _, arg := range args {
		if arg.Flags().Has(RequiredArg) {
			if _, ok := values[arg.Name()]; !ok {
				return fmt.Errorf("missing argument %q", arg.Name())
			}
		}
	}

	for key, value := range values {
		arg := FindArgument(args, key)
		if arg == nil {
			return fmt.Errorf("unknown argument %q", key)
		}

		if validator, ok := arg.(ArgumentValidator); ok {
			if err := validator.Validate(ctx, value); err != nil {
				return fmt.Errorf("%q: %w", key, err)
			}
		}
	}

	return nil
}

// ArgFlag is a flag that each argument may have.
type ArgFlag uint16

const (
	RequiredArg ArgFlag = 1 << iota
	NumberArg
)

// Has returns true if f has other.
func (f ArgFlag) Has(other ArgFlag) bool {
	return f&other == other
}

// Argument is a single argument declared inside a command.
type Argument interface {
	Name() string
	Desc() string
	Flags() ArgFlag
}

type ArgumentPlaceholder interface {
	Argument
	Placeholder() string
}

type ArgumentAutocompleter interface {
	Argument
	Autocomplete(context.Context, string) []string
}

type ArgumentValidator interface {
	Argument
	Validate(context.Context, string) error
}

// FindArgument searches the list of arguments for one with the given name. Nil
// is returned if none is found.
func FindArgument(args []Argument, name string) Argument {
	for _, arg := range args {
		if arg.Name() == name {
			return arg
		}
	}
	return nil
}

type stringArg struct {
	name  string
	desc  string
	flags ArgFlag
}

// StringArg creates a simple string argument.
func StringArg(name, desc string, flags ...ArgFlag) Argument {
	var flag ArgFlag
	for _, f := range flags {
		flag |= f
	}
	return stringArg{
		name,
		desc,
		flag,
	}
}

func (a stringArg) Name() string   { return a.name }
func (a stringArg) Desc() string   { return a.desc }
func (a stringArg) Flags() ArgFlag { return a.flags }

func (a stringArg) Validate(context.Context, string) error { return nil }

func (a stringArg) Autocomplete(context.Context, string) []string { return nil }

// Registry provides a registry of commands that helps with looking up commands
// and executing them.
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates a new Registry.
func NewRegistry(commands []Command) *Registry {
	r := Registry{commands: make(map[string]Command, len(commands))}
	for _, cmd := range commands {
		r.commands[cmd.Name()] = cmd
	}
	return &r
}

// ForInput creates a new InputCommander instance for a given input and the
// current registry.
func (r *Registry) ForInput(ctx context.Context, input *gtk.TextView, autocompleter *autocomplete.Autocompleter) *InputCommander {
	return NewInputCommander(ctx, r, input, autocompleter)
}
