package testutil

import (
	"runtime"
	"testing"
)

func TestHasGTKDisplayEnv(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		if !hasGTKDisplayEnv() {
			t.Fatal("expected hasGTKDisplayEnv to return true on desktop OS")
		}
		return
	}

	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("BROADWAY_DISPLAY", "")
	t.Setenv("GDK_BACKEND", "")
	if hasGTKDisplayEnv() {
		t.Fatal("expected hasGTKDisplayEnv to return false when all display envs are empty")
	}

	for _, env := range []string{"DISPLAY", "WAYLAND_DISPLAY", "BROADWAY_DISPLAY", "GDK_BACKEND"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv(env, "test")
			if !hasGTKDisplayEnv() {
				t.Fatalf("expected hasGTKDisplayEnv to return true when %s is set", env)
			}
		})
	}
}
