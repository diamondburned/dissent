package window

import (
	"log/slog"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/prefs"
	"libdb.so/dissent/internal/gtkcord"
)

var afkTimeout = prefs.NewInt(10, prefs.IntMeta{
	Name:    "Away Timeout",
	Section: "Discord",
	Description: "Minutes without using Dissent before Discord sends notifications " +
		"to your other devices instead. 0 disables this.",
	Min: 0,
	Max: 60,
})

// afkCheckInterval is how often the tracker checks whether the user has been
// away for long enough. It only checks while the user may still be around.
const afkCheckInterval = 60 // seconds

// afkTracker tells Discord whether the user is away from Dissent. Discord only
// sends push notifications to other devices, such as phones, while every
// desktop session is AFK. The user's status and activities are left alone.
type afkTracker struct {
	w *Window

	state  *gtkcord.State
	unbind func()

	// sessionID is the ID of our gateway session.
	sessionID string
	// status and activities are our session's presence from before we went
	// AFK, which are sent back as they are.
	status     discord.Status
	activities []discord.Activity

	// lastActive is when the user last used the window.
	lastActive time.Time
	// afk is whether Discord was last told that we're AFK.
	afk bool
	// screensaver is whether the screensaver is active.
	screensaver bool
	// checkTimer runs check while it is needed.
	checkTimer glib.SourceHandle

	// sendMu is held while sending, so that the presences are sent one at a
	// time and the last one sent is always the latest.
	sendMu  sync.Mutex
	mu      sync.Mutex
	pending *gateway.UpdatePresenceCommand
}

func newAFKTracker(w *Window) *afkTracker {
	t := &afkTracker{
		w:          w,
		lastActive: time.Now(),
	}

	// Any input on the window counts as using it.
	keys := gtk.NewEventControllerKey()
	keys.SetPropagationPhase(gtk.PhaseCapture)
	keys.ConnectKeyPressed(func(_, _ uint, _ gdk.ModifierType) bool {
		t.markActive("key pressed")
		return false
	})
	w.AddController(keys)

	motion := gtk.NewEventControllerMotion()
	motion.ConnectMotion(func(_, _ float64) {
		t.markActive("pointer moved")
	})
	w.AddController(motion)

	// Focusing or leaving the window also counts, so that the timeout starts
	// counting from when the user switches away.
	w.NotifyProperty("is-active", func() {
		t.markActive("window active changed")
	})

	// Locking the screen means the user is away right away.
	application := w.Application()
	application.NotifyProperty("screensaver-active", func() {
		t.screensaver = application.ObjectProperty("screensaver-active").(bool)
		slog.Debug(
			"afk: screensaver state changed",
			"screensaver_active", t.screensaver)
		if t.screensaver {
			t.setAFK(true, "screensaver active")
		} else {
			t.markActive("screensaver inactive")
		}
		t.updateCheckTimer()
	})

	t.updateCheckTimer()
	return t
}

// updateCheckTimer runs check periodically only while the user may still be
// around, so that nothing wakes up while they are away.
func (t *afkTracker) updateCheckTimer() {
	needed := !t.afk && !t.screensaver
	if needed == (t.checkTimer != 0) {
		return
	}

	slog.Debug(
		"afk: toggling idle check timer",
		"running", needed)

	if needed {
		t.checkTimer = glib.TimeoutSecondsAdd(afkCheckInterval, func() bool {
			t.check()
			return true
		})
	} else {
		glib.SourceRemove(t.checkTimer)
		t.checkTimer = 0
	}
}

// bind keeps track of our session's presence in the given state.
func (t *afkTracker) bind(state *gtkcord.State) {
	if t.unbind != nil {
		t.unbind()
	}

	t.state = state
	t.status = ""
	t.activities = nil

	// We're only bound once connected, so READY has already arrived.
	ready := state.Ready()
	t.sessionID = ready.SessionID
	t.updateSessions(ready.Sessions)

	t.unbind = state.AddHandler(
		func(ev *gateway.ReadyEvent) {
			t.sessionID = ev.SessionID
			t.updateSessions(ev.Sessions)
			// Reconnecting resets whether we're AFK.
			if t.afk {
				t.send("reconnected")
			}
		},
		func(ev *gateway.SessionsReplaceEvent) {
			t.updateSessions(*ev)
		},
		// The AFK state may have been sent on a connection that was already
		// dead, in which case it was lost.
		func(ev *gateway.ResumedEvent) {
			if t.afk {
				t.send("resumed")
			}
		},
	)
}

func (t *afkTracker) updateSessions(sessions []gateway.UserSession) {
	for _, s := range sessions {
		ours := s.SessionID == t.sessionID
		slog.Debug(
			"afk: session presence",
			"ours", ours,
			"client", s.ClientInfo.Client,
			"os", s.ClientInfo.OS,
			"status", s.Status,
			"active", s.Active,
			"activities", len(s.Activities))

		// Discord may change our presence while we're AFK, so only keep the
		// one from before.
		if ours && !t.afk {
			t.status = s.Status
			t.activities = s.Activities
		}
	}
}

// markActive marks the user as using the window now.
func (t *afkTracker) markActive(reason string) {
	t.lastActive = time.Now()
	if t.afk {
		t.setAFK(false, reason)
	}
}

// check sets the user as AFK if they have not used the window for longer than
// the timeout.
func (t *afkTracker) check() {
	if t.afk {
		return
	}

	timeout := time.Duration(afkTimeout.Value()) * time.Minute
	if timeout == 0 {
		return
	}

	idle := time.Since(t.lastActive)
	slog.Debug(
		"afk: checking idle time",
		"idle", idle.Round(time.Second),
		"timeout", timeout,
		"window_active", t.w.IsActive())

	if idle >= timeout {
		t.setAFK(true, "idle timeout")
	}
}

func (t *afkTracker) setAFK(afk bool, reason string) {
	if afk && afkTimeout.Value() == 0 {
		return
	}
	if t.afk == afk {
		return
	}
	t.afk = afk
	t.updateCheckTimer()
	t.send(reason)
}

// send sends whether we're AFK to Discord.
func (t *afkTracker) send(reason string) {
	if t.state == nil || t.status == "" {
		slog.Debug(
			"afk: own session presence unknown, not sending AFK state",
			"afk", t.afk,
			"reason", reason)
		return
	}

	cmd := &gateway.UpdatePresenceCommand{
		Status:     t.status,
		Activities: t.activities,
		AFK:        t.afk,
	}
	if cmd.Activities == nil {
		cmd.Activities = []discord.Activity{}
	}
	if t.afk {
		cmd.Since = discord.TimeToMilliseconds(t.lastActive)
	}

	slog.Debug(
		"afk: sending AFK state",
		"afk", cmd.AFK,
		"since", cmd.Since,
		"status", cmd.Status,
		"activities", len(cmd.Activities),
		"reason", reason)

	t.mu.Lock()
	t.pending = cmd
	t.mu.Unlock()

	state := t.state
	go func() {
		t.sendMu.Lock()
		defer t.sendMu.Unlock()

		t.mu.Lock()
		cmd := t.pending
		t.pending = nil
		t.mu.Unlock()

		// A later call already sent this.
		if cmd == nil {
			return
		}

		if err := state.SendGateway(state.Context(), cmd); err != nil {
			slog.Warn(
				"afk: cannot send AFK state",
				"afk", cmd.AFK,
				"err", err)
		}
	}()
}
