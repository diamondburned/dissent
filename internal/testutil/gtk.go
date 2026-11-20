package testutil

import (
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

var gtkReady = sync.OnceValue(func() bool {
	if !gtk.InitCheck() || gdk.DisplayGetDefault() == nil {
		return false
	}
	adw.Init()
	return true
})

func hasGTKDisplayEnv() bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return true
	}
	return os.Getenv("DISPLAY") != "" ||
		os.Getenv("WAYLAND_DISPLAY") != "" ||
		os.Getenv("BROADWAY_DISPLAY") != "" ||
		os.Getenv("GDK_BACKEND") != ""
}

// RequireGTK initializes GTK and Libadwaita once per test process, or skips the
// test when no GUI/GTK display environment is available.
func RequireGTK(t testing.TB) {
	t.Helper()
	if !hasGTKDisplayEnv() {
		t.Skip("GTK display environment not set")
	}
	if !gtkReady() {
		t.Skip("GTK display not available")
	}
}
