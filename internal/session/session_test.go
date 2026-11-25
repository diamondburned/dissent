package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/diamondburned/arikawa/v3/utils/ws"
	"github.com/diamondburned/chatkit/kits/secret"
	"github.com/diamondburned/ningen/v3"
)

func TestIsAuthFailedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "close 4004",
			err:  &ws.CloseEvent{Code: 4004, Err: errors.New("4004 Authentication failed.")},
			want: true,
		},
		{
			name: "wrapped close 4004",
			err:  fmt.Errorf("open: %w", &ws.CloseEvent{Code: 4004, Err: errors.New("4004 Authentication failed.")}),
			want: true,
		},
		{
			name: "disconnected 4004",
			err:  &ningen.DisconnectedEvent{CloseEvent: ws.CloseEvent{Code: 4004, Err: errors.New("4004 Authentication failed.")}},
			want: true,
		},
		{
			name: "close 4000",
			err:  &ws.CloseEvent{Code: 4000, Err: errors.New("4000 Unknown error.")},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAuthFailedError(tc.err); got != tc.want {
				t.Fatalf("isAuthFailedError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestForgetStoredAccountRemovesEncryptedFile(t *testing.T) {
	dir := t.TempDir()
	ctx := secret.WithEncryptedFilePath(context.Background(), dir)

	drv := secret.EncryptedFileDriver(ctx, "test-pass")
	if err := drv.Set("account", []byte("secret-token")); err != nil {
		t.Fatalf("drv.Set: %v", err)
	}
	if _, err := drv.Get("account"); err != nil {
		t.Fatalf("drv.Get before forget: %v", err)
	}

	forgetStoredAccount("", dir)

	if _, err := drv.Get("account"); !errors.Is(err, secret.ErrNotFound) {
		t.Fatalf("drv.Get after forget = %v, want %v", err, secret.ErrNotFound)
	}
	if _, err := os.Stat(filepath.Join(dir, ".hash")); err != nil {
		t.Fatalf("expected .hash to remain intact: %v", err)
	}
}

func TestSessionLogoutResetsStatusAndAdvancesGeneration(t *testing.T) {
	s := New(context.Background())

	var updates []Status
	unsub := s.Subscribe(func() {
		updates = append(updates, s.Status())
	})
	defer unsub()

	s.gen = 3
	s.setStatus(Connecting, errors.New("stale error"))
	prevGen := s.gen

	s.Logout()

	if s.Status() != LoggedOut {
		t.Fatalf("Status() = %v, want %v", s.Status(), LoggedOut)
	}
	if s.Err() != nil {
		t.Fatalf("Err() = %v, want nil", s.Err())
	}
	if s.State() != nil {
		t.Fatalf("State() = %v, want nil", s.State())
	}
	if s.gen <= prevGen {
		t.Fatalf("gen = %d, want > %d", s.gen, prevGen)
	}
	if len(updates) != 2 || updates[0] != Connecting || updates[1] != LoggedOut {
		t.Fatalf("subscriber updates = %v, want [Connecting LoggedOut]", updates)
	}
}
