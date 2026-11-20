package login

import (
	"context"
	_ "embed"
	"strings"

	"github.com/diamondburned/chatkit/components/secretdialog"
	"github.com/diamondburned/chatkit/kits/secret"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/pkg/errors"
)

//go:embed token_dialog.ui
var tokenDialogUI string

// Component is the main greeter component in the login page.
type Component struct {
	*adw.StatusPage
	Remember *rememberMeBox

	errorBanner        *adw.Banner
	loginDiscordButton *gtk.Button
	loginButtonStack   *gtk.Stack
	tokenButton        *gtk.Button
	decryptGroup       *adw.PreferencesGroup
	decryptRow         *adw.ActionRow

	ctx  context.Context
	page *Page
}

func newComponent(ctx context.Context, p *Page, get func(string) glib.Objector) *Component {
	c := Component{
		StatusPage:         get("statusPage").(*adw.StatusPage),
		errorBanner:        get("errorBanner").(*adw.Banner),
		loginDiscordButton: get("loginDiscordButton").(*gtk.Button),
		loginButtonStack:   get("loginButtonStack").(*gtk.Stack),
		tokenButton:        get("tokenButton").(*gtk.Button),
		decryptGroup:       get("decryptGroup").(*adw.PreferencesGroup),
		decryptRow:         get("decryptRow").(*adw.ActionRow),
		ctx:                ctx,
		page:               p,
	}

	c.Remember = newRememberMeBox(ctx, p.NavigationView)
	c.errorBanner.ConnectButtonClicked(c.HideError)
	c.loginDiscordButton.ConnectClicked(c.loginBrowser)
	c.tokenButton.ConnectClicked(func() { c.openTokenDialog() })
	c.decryptRow.ConnectActivated(c.askDecrypt)

	if !browserLoginSupported {
		c.loginDiscordButton.SetVisible(false)
		c.tokenButton.AddCSSClass("suggested-action")
		c.StatusPage.SetDescription("Input your Discord account token to sign in.")
	}

	gtkutil.Async(ctx, func() func() {
		if secret.IsEncrypted(ctx) {
			return func() { c.decryptGroup.SetVisible(true) }
		}
		return nil
	})

	return &c
}

// ShowError reveals the error banner and shows the error to the user.
func (c *Component) ShowError(err error) {
	msg := err.Error()
	c.errorBanner.SetTitle(msg)
	c.errorBanner.SetTooltipText(msg)
	c.errorBanner.SetRevealed(true)
}

// HideError hides the error banner.
func (c *Component) HideError() {
	c.errorBanner.SetRevealed(false)
}

func (c *Component) SetBusy() {
	c.SetSensitive(false)
	c.loginButtonStack.SetVisibleChildName("busy")
}

func (c *Component) SetDone() {
	c.SetSensitive(true)
	c.loginButtonStack.SetVisibleChildName("label")
}

func (c *Component) openTokenDialog() *adw.Dialog {
	ui := gtk.NewBuilderFromString(tokenDialogUI)
	get := func(name string) glib.Objector { return ui.GetObject(name).Cast() }

	dlg := get("rootContent").(*adw.Dialog)
	cancelButton := get("cancelButton").(*gtk.Button)
	submitButton := get("submitButton").(*gtk.Button)
	tokenEntry := get("tokenEntry").(*adw.PasswordEntryRow)

	dlg.SetDefaultWidget(submitButton)

	trimmedToken := func() string {
		return strings.TrimSpace(strings.Trim(tokenEntry.Text(), `"`))
	}

	submit := func() {
		token := trimmedToken()
		if token == "" {
			return
		}
		c.HideError()
		dlg.Close()
		c.loginToken(token)
	}

	tokenEntry.ConnectChanged(func() {
		submitButton.SetSensitive(trimmedToken() != "")
	})
	tokenEntry.ConnectEntryActivated(submit)
	submitButton.ConnectClicked(submit)
	cancelButton.ConnectClicked(func() { dlg.Close() })

	dlg.Present(c)
	tokenEntry.GrabFocus()
	return dlg
}

func (c *Component) loginToken(token string) {
	go func() {
		driver := c.Remember.SecretDriver()
		if driver == nil {
			return
		}

		if err := driver.Set("account", []byte(token)); err != nil {
			app.Error(c.ctx, errors.Wrap(err, "cannot store account as secret"))
		}
	}()

	c.page.asyncUseToken(token)
}

func (c *Component) askDecrypt() {
	secretdialog.PromptPassword(
		c.ctx, secretdialog.PromptDecrypt,
		func(ok bool, enc *secret.EncryptedFile) {
			if ok {
				c.page.asyncLoadFromSecrets(enc)
			}
		},
	)
}
