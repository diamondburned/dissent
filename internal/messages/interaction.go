package messages

import (
	"errors"
	"log/slog"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil"
	"libdb.so/dissent/internal/gtkcord"
)

// interactionFailedError is returned when Discord reports that an interaction
// failed.
type interactionFailedError struct {
	reason gateway.InteractionFailureReason
}

func (err interactionFailedError) Error() string {
	return "interaction failed: " + err.reason.String()
}

// interactionFailureMessage returns the message to show for err if it is an
// interactionFailedError.
func interactionFailureMessage(err error) (string, bool) {
	var failed interactionFailedError
	if !errors.As(err, &failed) {
		return "", false
	}
	switch failed.reason {
	case gateway.InteractionFailureTimeout:
		return locale.Get("The application did not respond"), true
	case gateway.InteractionFailureUnknown:
		return locale.Get("Interaction failed"), true
	default:
		return failed.reason.String(), true
	}
}

// interactionTarget describes where an interaction happens.
type interactionTarget struct {
	AppID        discord.AppID
	ChannelID    discord.ChannelID
	MessageID    discord.MessageID
	MessageFlags discord.MessageFlags
}

// messageInteractionTarget returns the target for components on the message.
func messageInteractionTarget(m *discord.Message) interactionTarget {
	appID := m.ApplicationID
	if !appID.IsValid() && m.Author.Bot {
		// Bots sending messages without an interaction have no application
		// ID, but their user ID is the same as their application ID.
		appID = discord.AppID(m.Author.ID)
	}
	return interactionTarget{
		AppID:        appID,
		ChannelID:    m.ChannelID,
		MessageID:    m.ID,
		MessageFlags: m.Flags,
	}
}

// sendInteraction sends an interaction to an application. done is called
// with nil once the application acknowledges it or with an error if it
// fails. It is called on the main thread.
func (v *View) sendInteraction(
	target interactionTarget, typ discord.InteractionDataType, data any, done func(error)) {

	if v.interactions == nil {
		v.interactions = make(map[string]func(error))
	}

	nonce := discord.NewSnowflake(time.Now()).String()
	v.interactions[nonce] = done

	state := gtkcord.FromContext(v.ctx)
	send := api.SendInteractionData{
		Type:         typ,
		Nonce:        nonce,
		SessionID:    state.Ready().SessionID,
		AppID:        target.AppID,
		GuildID:      v.guildID,
		ChannelID:    target.ChannelID,
		MessageID:    target.MessageID,
		MessageFlags: target.MessageFlags,
		Data:         data,
	}

	gtkutil.Async(v.ctx, func() func() {
		err := state.SendInteraction(send)
		if err == nil {
			return nil // wait for the gateway
		}
		return func() {
			slog.Error(
				"cannot send interaction",
				"channel_id", target.ChannelID,
				"err", err)
			v.finishInteraction(nonce, err)
		}
	})
}

// finishInteraction calls the callback of the pending interaction with the
// given nonce, if any.
func (v *View) finishInteraction(nonce string, err error) bool {
	done, ok := v.interactions[nonce]
	if ok {
		delete(v.interactions, nonce)
		done(err)
	}
	return ok
}

// handleInteractionEvent handles the gateway events for interactions sent by
// this view.
func (v *View) handleInteractionEvent(ev gateway.Event) {
	switch ev := ev.(type) {
	case *gateway.InteractionSuccessEvent:
		v.finishInteraction(ev.Nonce, nil)
	case *gateway.InteractionFailureEvent:
		v.finishInteraction(ev.Nonce, interactionFailedError{ev.ReasonCode})
	case *gateway.InteractionModalCreateEvent:
		if v.finishInteraction(ev.Nonce, nil) {
			newComponentModal(v, ev).Present(v)
		}
	}
}

// interactionToast shows an error toast for a failed interaction.
func (v *View) interactionToast(err error) {
	msg, ok := interactionFailureMessage(err)
	if !ok {
		msg = locale.Get("Interaction failed")
	}
	toast := adw.NewToast(msg)
	toast.SetTimeout(5)
	v.AddToast(toast)
}
