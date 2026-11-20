package login

import (
	"context"
	"errors"
	"testing"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
)

type stubLoginController struct {
	tokens []string
}

func (s *stubLoginController) Login(token string) {
	s.tokens = append(s.tokens, token)
}

func TestLoginUIFiles(t *testing.T) {
	if !gtk.InitCheck() {
		t.Skip("GTK display not available")
	}
	adw.Init()

	ctx := context.Background()
	gtkApp := adw.NewApplication("so.libdb.dissent.test", gio.ApplicationFlagsNone)
	ctx = app.WithApplication(ctx, app.Wrap(ctx, &gtkApp.Application, "Dissent"))
	ctrl := &stubLoginController{}

	page := NewPage(ctx, ctrl)
	if page == nil || page.Login == nil {
		t.Fatal("NewPage returned nil")
	}

	page.Login.ShowError(errors.New("bad token"))
	if !page.Login.errorBanner.Revealed() {
		t.Fatal("expected error banner to be revealed")
	}
	page.Login.HideError()
	if page.Login.errorBanner.Revealed() {
		t.Fatal("expected error banner to be hidden")
	}

	page.Login.SetBusy()
	if page.Login.loginButtonStack.VisibleChildName() != "busy" {
		t.Fatal("expected busy spinner child on button stack")
	}
	page.Login.SetDone()
	if page.Login.loginButtonStack.VisibleChildName() != "label" {
		t.Fatal("expected label child on button stack")
	}

	win := gtk.NewWindow()
	win.SetChild(page)
	dlg := page.Login.openTokenDialog()
	if dlg == nil {
		t.Fatal("openTokenDialog returned nil")
	}
	dlg.ForceClose()
	win.Destroy()

	loading := NewLoadingPage(ctx)
	if loading == nil {
		t.Fatal("NewLoadingPage returned nil")
	}
	loading.SetText("Reconnecting...")
	if loading.statusPage.Title() != "Reconnecting..." {
		t.Fatalf("statusPage.Title() = %q, want %q", loading.statusPage.Title(), "Reconnecting...")
	}

	initial := page.Login.Remember.action.State().Boolean()
	if initial {
		if page.Login.Remember.SecretDriver() == nil {
			t.Fatal("expected non-nil SecretDriver when login.remember starts checked")
		}
		page.Login.Remember.action.ChangeState(glib.NewVariantBoolean(false))
		if page.Login.Remember.action.State().Boolean() || page.Login.Remember.SecretDriver() != nil {
			t.Fatal("expected login.remember action to reset to false")
		}
	} else {
		page.Login.Remember.action.Activate(nil)
		if !page.Login.Remember.action.State().Boolean() {
			t.Fatal("expected login.remember action to toggle true on activate")
		}
		page.Login.Remember.action.ChangeState(glib.NewVariantBoolean(false))
		if page.Login.Remember.action.State().Boolean() || page.Login.Remember.SecretDriver() != nil {
			t.Fatal("expected login.remember action to reset to false")
		}
	}
}
