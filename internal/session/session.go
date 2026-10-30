// Package session owns the Discord session for the whole application. It lives
// outside of any window so that Dissent can stay connected and keep sending
// notifications while no window is open.
package session

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/ws"
	"github.com/diamondburned/chatkit/kits/secret"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/notify"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/ningen/v3"
	"github.com/pkg/errors"
	"libdb.so/dissent/internal/gtkcord"
)

// RunInBackground keeps Dissent connected after its window is closed.
var RunInBackground = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Run in Background",
	Section:     "Discord",
	Description: "Stay connected and keep sending notifications after the window is closed.",
})

var useDiscordColorScheme = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Use Discord's color preference",
	Section:     "Discord",
	Description: "Whether or not to use Discord's dark/light mode preference.",
})

// SetPreferDarkTheme sets whether or not GTK should use a dark theme.
func SetPreferDarkTheme(prefer bool) {
	if !useDiscordColorScheme.Value() {
		return
	}

	scheme := adw.ColorSchemePreferLight
	if prefer {
		scheme = adw.ColorSchemePreferDark
	}

	adwStyles := adw.StyleManagerGetDefault()
	adwStyles.SetColorScheme(scheme)
}

// Status is the connection status of a Session.
type Status int

const (
	// LoggedOut means that the user needs to log in.
	LoggedOut Status = iota
	// Connecting means that the session is logging in or reconnecting.
	Connecting
	// Connected means that the session is connected and ready.
	Connected
)

// Session is the application-wide Discord session. All methods must be called
// from the main thread.
type Session struct {
	app     *app.Application
	baseCtx context.Context
	ctx     context.Context

	state  *gtkcord.State
	unbind func()
	status Status
	err    error

	reconnecting glib.SourceHandle
	held         bool

	portalBusy  bool
	portalDirty bool

	subs  map[int]func()
	subID int
}

// New creates a new logged out Session.
func New(ctx context.Context) *Session {
	s := &Session{
		app:     app.FromContext(ctx),
		baseCtx: ctx,
		ctx:     ctx,
		subs:    make(map[int]func()),
	}
	RunInBackground.Subscribe(s.updateHold)
	return s
}

// State returns the current state, or nil if the session is logged out.
func (s *Session) State() *gtkcord.State { return s.state }

// Status returns the current connection status.
func (s *Session) Status() Status { return s.status }

// Err returns the error that caused the session to be logged out, if any.
func (s *Session) Err() error { return s.err }

// Subscribe calls f every time the status changes. It returns a function that
// unsubscribes f.
func (s *Session) Subscribe(f func()) (unsubscribe func()) {
	id := s.subID
	s.subID++
	s.subs[id] = f
	return func() { delete(s.subs, id) }
}

// LoadKeyring logs in using the token stored in the keyring, if there is one.
func (s *Session) LoadKeyring() {
	s.setStatus(Connecting, nil)

	driver := secret.KeyringDriver(s.baseCtx)
	gtkutil.Async(s.baseCtx, func() func() {
		b, err := driver.Get("account")
		if err != nil {
			slog.Info(
				"account not found in keyring",
				"err", err)
			return func() { s.setStatus(LoggedOut, nil) }
		}

		return func() { s.Login(string(b)) }
	})
}

// Login logs in using the given token, replacing the current session.
func (s *Session) Login(token string) {
	s.closeState(false)

	state := gtkcord.Wrap(state.New(token))
	s.state = state
	s.ctx = gtkcord.InjectState(s.baseCtx, state)
	s.unbind = state.AddHandler(s.handleEvent)
	s.setStatus(Connecting, nil)

	ctx := s.ctx
	gtkutil.Async(ctx, func() func() {
		err := state.Open(ctx)
		return func() {
			if err == nil || s.state != state {
				return
			}
			s.closeState(false)
			s.setStatus(LoggedOut, errors.Wrap(err, "cannot open session"))
		}
	})
}

// Close closes the session and waits for it to disconnect. It is meant to be
// called when the application shuts down.
func (s *Session) Close() {
	s.closeState(true)
}

func (s *Session) closeState(wait bool) {
	if s.state == nil {
		return
	}

	if s.reconnecting != 0 {
		glib.SourceRemove(s.reconnecting)
		s.reconnecting = 0
	}

	state := s.state
	s.unbind()
	s.state = nil
	s.unbind = nil
	s.ctx = s.baseCtx

	close := func() {
		slog.Info("Closing Discord session...")
		if err := state.Close(); err != nil {
			slog.Error("error closing session", "err", err)
		}
	}
	if wait {
		close()
	} else {
		go close()
	}
}

func (s *Session) setStatus(status Status, err error) {
	s.status = status
	s.err = err
	s.updateHold()
	s.updatePortalStatus()

	for _, f := range s.subs {
		f()
	}
}

// updateHold keeps the application running without any windows for as long as
// the session isn't logged out.
func (s *Session) updateHold() {
	hold := s.status != LoggedOut && RunInBackground.Value()
	if hold == s.held {
		return
	}

	s.held = hold
	if hold {
		s.app.Hold()
	} else {
		s.app.Release()
	}
}

func (s *Session) handleEvent(ev gateway.Event) {
	switch ev := ev.(type) {
	case *ningen.ConnectedEvent:
		slog.Info(
			"Discord gateway connected",
			"event", ev.EventType())

		// Cancel the 3s delay if we're already connected during that.
		if s.reconnecting != 0 {
			glib.SourceRemove(s.reconnecting)
			s.reconnecting = 0
		}

		s.setStatus(Connected, nil)

	case *ws.BackgroundErrorEvent:
		slog.Warn(
			"Discord gateway background error",
			"err", ev.Err)

	case *ws.CloseEvent:
		slog.Info(
			"Discord gateway closed",
			"err", ev.Err,
			"code", ev.Code)

	case *ningen.DisconnectedEvent:
		slog.Info(
			"Discord gateway disconnected",
			"err", ev.Err,
			"code", ev.Code)

		if ev.IsLoggedOut() {
			s.closeState(false)
			s.setStatus(LoggedOut, nil)
			return
		}

		// Add a 3s delay in case we have a sudden disruption that
		// immediately recovers.
		if s.reconnecting == 0 {
			s.reconnecting = glib.TimeoutSecondsAdd(3, func() {
				s.reconnecting = 0
				s.setStatus(Connecting, nil)
			})
		}

	case *gateway.ReadyEvent:
		if ev.UserSettings != nil {
			switch ev.UserSettings.Theme {
			case "dark":
				SetPreferDarkTheme(true)
			case "light":
				SetPreferDarkTheme(false)
			}
		}

	case *gateway.MessageCreateEvent:
		s.notifyMention(ev)
	}
}

func (s *Session) notifyMention(ev *gateway.MessageCreateEvent) {
	if s.state.MessageMentions(&ev.Message) == 0 {
		return
	}

	if s.state.Status() == discord.DoNotDisturbStatus {
		return
	}

	avatarURL := gtkcord.InjectAvatarSize(ev.Author.AvatarURL())

	notify.Send(s.ctx, notify.Notification{
		ID: notify.HashID(ev.ChannelID),
		Title: fmt.Sprintf(
			"%s (%s)",
			s.state.AuthorDisplayName(ev),
			gtkcord.ChannelNameFromID(s.ctx, ev.ChannelID),
		),
		Body:  s.state.MessagePreview(&ev.Message),
		Icon:  notify.IconURL(s.ctx, avatarURL, notify.IconName("avatar-default-symbolic")),
		Sound: notify.MessageSound,
		Action: notify.Action{
			ActionID: "app.open-channel",
			Argument: gtkcord.NewChannelIDVariant(ev.ChannelID),
		},
	})
}
