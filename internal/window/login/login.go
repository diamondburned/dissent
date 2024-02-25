package login

import (
	"context"
	"log"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/chatkit/kits/secret"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/pkg/errors"
	"libdb.so/dissent/internal/gtkcord"
)

// LoginController is the parent controller that Page controls.
type LoginController interface {
	// Hook is called before the state is opened and before Ready is called. It
	// is meant to be called for hooking the handlers.
	Hook(*gtkcord.State)
	// Ready is called once the user has fully logged in. The session given to
	// the controller will have already been opened and have received the Ready
	// event.
	Ready(*gtkcord.State)
	// PromptLogin is called by the login page if the user needs to log in
	// again, either because their credentials are wrong or Discord returns a
	// server error.
	PromptLogin()
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

// LoadKeyring loads the session from the keyring.
func (p *Page) LoadKeyring() {
	p.asyncLoadFromSecrets(secret.KeyringDriver(p.ctx))
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
			log.Println("note: account not found from driver:", err)
			return done
		}

		return func() {
			done()
			p.asyncUseToken(string(b))
		}
	})
}

// asyncUseToken connects with the given token. If driver != nil, then the token
// is stored.
func (p *Page) asyncUseToken(token string) {
	state := gtkcord.Wrap(state.New(token))
	p.ctrl.Hook(state)

	gtkutil.Async(p.ctx, func() func() {
		if err := state.Open(p.ctx); err != nil {
			return func() {
				p.ctrl.PromptLogin()
				p.Login.ShowError(errors.Wrap(err, "cannot open session"))
			}
		}

		return func() {
			p.ctrl.Ready(state)
		}
	})
}
