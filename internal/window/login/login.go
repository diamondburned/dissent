package login

import (
	"context"
	_ "embed"
	"log/slog"

	"github.com/diamondburned/chatkit/kits/secret"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
)

//go:embed login.ui
var loginUI string

// LoginController is the parent controller that Page controls.
type LoginController interface {
	// Login logs in using the given token. The controller is responsible for
	// switching away from the login page or showing errors.
	Login(token string)
}

// Page is the page containing the login greeter and browser login flow.
type Page struct {
	*adw.NavigationView
	Login *Component

	ctx  context.Context
	ctrl LoginController
}

// NewPage creates a new Page.
func NewPage(ctx context.Context, ctrl LoginController) *Page {
	ui := gtk.NewBuilderFromString(loginUI)
	get := func(name string) glib.Objector { return ui.GetObject(name).Cast() }

	p := Page{
		NavigationView: get("rootContent").(*adw.NavigationView),
		ctx:            ctx,
		ctrl:           ctrl,
	}
	p.Login = newComponent(ctx, &p, get)
	return &p
}

func (p *Page) asyncLoadFromSecrets(driver secret.Driver) {
	p.Login.SetBusy()

	done := func() {
		p.Login.SetDone()
	}

	gtkutil.Async(p.ctx, func() func() {
		b, err := driver.Get("account")
		if err != nil {
			slog.Info(
				"account not found in keyring",
				"err", err)
			return done
		}

		return func() {
			done()
			p.asyncUseToken(string(b))
		}
	})
}

// asyncUseToken logs in using the given token.
func (p *Page) asyncUseToken(token string) {
	p.ctrl.Login(token)
}
