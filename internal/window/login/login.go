package login

import (
	"context"
	"log/slog"

	"github.com/diamondburned/chatkit/kits/secret"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// LoginController is the parent controller that Page controls.
type LoginController interface {
	// Login logs in using the given token. The controller is responsible for
	// switching away from the login page or showing errors.
	Login(token string)
}

// Page is the page containing the login forms.
type Page struct {
	*gtk.Box
	Header *gtk.HeaderBar
	Login  *Component

	ctx  context.Context
	ctrl LoginController
}

var pageCSS = cssutil.Applier("login-page", ``)

// NewPage creates a new Page.
func NewPage(ctx context.Context, ctrl LoginController) *Page {
	p := Page{
		ctx:  ctx,
		ctrl: ctrl,
	}

	p.Header = gtk.NewHeaderBar()
	p.Header.AddCSSClass("login-page-header")
	p.Header.SetShowTitleButtons(true)

	p.Login = NewComponent(ctx, &p)
	p.Login.SetVExpand(true)
	p.Login.SetHExpand(true)

	p.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	p.Box.Append(p.Header)
	p.Box.Append(p.Login)
	pageCSS(p)

	return &p
}

func (p *Page) asyncLoadFromSecrets(driver secret.Driver) {
	p.Login.Loading.Show()
	p.Login.SetSensitive(false)

	done := func() {
		p.Login.Loading.Hide()
		p.Login.SetSensitive(true)
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
