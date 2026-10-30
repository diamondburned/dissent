package session

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
)

// Autostart starts Dissent in the background when the user logs in.
var Autostart = prefs.NewBool(false, prefs.PropMeta{
	Name:        "Start on Login",
	Section:     "Discord",
	Description: "Start Dissent in the background when logging in. Requires Run in Background.",
})

const (
	portalName = "org.freedesktop.portal.Desktop"
	portalPath = "/org/freedesktop/portal/desktop"
)

// sandboxed is true if Dissent is running inside Flatpak.
var sandboxed = func() bool {
	_, err := os.Stat("/.flatpak-info")
	return err == nil
}()

// StartBackgroundPortal tells the Background portal whether Dissent runs in
// the background and starts on login. Desktops like GNOME use this to list
// Dissent as a background app. It should be called once preferences are
// loaded.
func (s *Session) StartBackgroundPortal() {
	RunInBackground.Subscribe(s.requestBackground)
	Autostart.Subscribe(s.requestBackground)
}

// requestBackground calls RequestBackground with the current preferences.
// Only one request runs at a time, since the portal may show a dialog for
// each.
func (s *Session) requestBackground() {
	if s.portalBusy {
		s.portalDirty = true
		return
	}
	s.portalBusy = true

	appID := s.app.ID()
	autostart := RunInBackground.Value() && Autostart.Value()

	go func() {
		background, autostarted, err := requestBackground(appID, autostart)
		glib.IdleAdd(func() {
			s.portalBusy = false

			switch {
			case err != nil:
				slog.Warn(
					"cannot request to run in the background",
					"err", err)
			case !background:
				slog.Warn("not allowed to run in the background")
			case autostart != autostarted:
				slog.Warn(
					"cannot change autostart",
					"autostart", autostart)
			}

			if s.portalDirty {
				s.portalDirty = false
				s.requestBackground()
			}
		})
	}()
}

// updatePortalStatus shows the session status in the desktop's list of
// background apps. The portal only allows this for sandboxed apps.
func (s *Session) updatePortalStatus() {
	if !sandboxed {
		return
	}

	var message string
	switch s.status {
	case LoggedOut:
		message = "Logged out"
	case Connecting:
		message = "Connecting"
	case Connected:
		message = "Connected"
	}

	go func() {
		conn, err := portalConn()
		if err == nil {
			err = conn.Object(portalName, portalPath).Call(
				"org.freedesktop.portal.Background.SetStatus", 0,
				map[string]dbus.Variant{"message": dbus.MakeVariant(message)},
			).Err
		}
		if err != nil {
			slog.Warn(
				"cannot set background status",
				"err", err)
		}
	}()
}

// portalConn returns a private session bus connection for the portal. It is
// private so that we can register our app ID before any other portal call on
// it, which the portal requires.
var portalConn = sync.OnceValues(func() (*dbus.Conn, error) {
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to session bus")
	}

	if err := conn.Auth(nil); err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "cannot authenticate to session bus")
	}

	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "cannot greet session bus")
	}

	return conn, nil
})

var registerOnce sync.Once

func requestBackground(appID string, autostart bool) (background, autostarted bool, err error) {
	conn, err := portalConn()
	if err != nil {
		return false, false, err
	}

	// Outside of Flatpak, the portal only knows our app ID if the desktop
	// launched us. Tell it ourselves in case we were started from a terminal.
	if !sandboxed {
		registerOnce.Do(func() {
			err := conn.Object(portalName, portalPath).Call(
				"org.freedesktop.host.portal.Registry.Register", 0,
				appID, map[string]dbus.Variant{},
			).Err
			if err != nil {
				slog.Debug(
					"cannot register app ID with the portal",
					"err", err)
			}
		})
	}

	token := fmt.Sprintf("dissent%d", rand.Uint32())
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	handle := dbus.ObjectPath(portalPath + "/request/" + sender + "/" + token)

	match := []dbus.MatchOption{
		dbus.WithMatchObjectPath(handle),
		dbus.WithMatchInterface("org.freedesktop.portal.Request"),
		dbus.WithMatchMember("Response"),
	}
	if err := conn.AddMatchSignal(match...); err != nil {
		return false, false, errors.Wrap(err, "cannot watch for the portal response")
	}
	defer conn.RemoveMatchSignal(match...)

	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	err = conn.Object(portalName, portalPath).Call(
		"org.freedesktop.portal.Background.RequestBackground", 0,
		"", map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(token),
			"reason":       dbus.MakeVariant("Stay connected to Discord and show notifications."),
			"autostart":    dbus.MakeVariant(autostart),
			"commandline":  dbus.MakeVariant([]string{"dissent", "--gapplication-service"}),
		},
	).Err
	if err != nil {
		return false, false, errors.Wrap(err, "cannot call RequestBackground")
	}

	for sig := range signals {
		if sig.Path != handle || len(sig.Body) != 2 {
			continue
		}

		// A response other than 0 means that the user denied the request.
		response, _ := sig.Body[0].(uint32)
		results, _ := sig.Body[1].(map[string]dbus.Variant)
		if response != 0 {
			return false, false, nil
		}

		background, _ = results["background"].Value().(bool)
		autostarted, _ = results["autostart"].Value().(bool)
		return background, autostarted, nil
	}

	return false, false, errors.New("session bus disconnected")
}
