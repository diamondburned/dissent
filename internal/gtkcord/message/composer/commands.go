package composer

import (
	"context"

	"github.com/diamondburned/gtkcord4/internal/gtkcord/message/composer/command"
	"github.com/pkg/errors"
)

var commands = command.NewRegistry([]command.Command{
	helloCommand{},
	meCommand{},
})

type helloCommand struct{}

var _ command.Command = helloCommand{}

func (c helloCommand) Name() string             { return "hello" }
func (c helloCommand) Desc() string             { return "Send a Hello world message." }
func (c helloCommand) Args() []command.Argument { return nil }

func (c helloCommand) Exec(ctx context.Context, args command.ArgValues) error {
	state := command.RequireChannelState(ctx)

	_, err := state.SendMessage(state.ChannelID, "Hello, 世界!")
	if err != nil {
		return errors.Wrap(err, "failed to send message")
	}

	return nil
}

type meCommand struct{}

var _ command.Command = meCommand{}

func (c meCommand) Name() string { return "me" }
func (c meCommand) Desc() string { return "It's (not) like /me, but on Discord!" }

func (c meCommand) Args() []command.Argument {
	return []command.Argument{
		command.StringArg("message", "the message to send", command.RequiredArg),
	}
}

func (c meCommand) Exec(ctx context.Context, args command.ArgValues) error {
	state := command.RequireChannelState(ctx)

	_, err := state.SendMessage(state.ChannelID, args["message"])
	if err != nil {
		return errors.Wrap(err, "failed to send message")
	}

	return nil
}
