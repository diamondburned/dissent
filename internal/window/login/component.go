package login

import (
	"context"
	//"strings"

	//"github.com/diamondburned/adaptive"
	"github.com/diamondburned/arikawa/v3/session"
	"github.com/diamondburned/chatkit/components/secretdialog"
	"github.com/diamondburned/chatkit/kits/secret"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/pkg/errors"

	//"libdb.so/dissent/internal/window/login/loading"
	"libdb.so/dissent/internal/gresources"
)

// LoginComponent is the main component in the login page.
type Component struct {
	*adw.ViewStack
	Inner *gtk.Box

	//	Loading  *loading.PulsatingBar
	Methods       *Methods
	Bottom        *gtk.Box
	Remember      *rememberMeBox
	ErrorRev      *adw.Banner
	LogIn         *gtk.Button
	LoadInfoLabel *gtk.Label

	loginCarousel         *adw.Carousel
	greeterStack          *adw.ViewStack
	loginCarouselTarget   *adw.ToolbarView
	uiFile                *gresources.UiFile
	pageNameBeforeLoading string
	ctx                   context.Context
	page                  *Page
}

var componentCSS = cssutil.Applier("login-component", `
	.login-component {
		background: mix(@theme_bg_color, @theme_fg_color, 0.05);
		border-radius: 12px;
		min-width: 250px;
		margin:  12px;
		padding: 0;
	}
	.login-component > *:not(.osd) {
		margin: 0 8px;
	}
	.login-component > *:nth-child(2) {
		margin-top: 6px;
	}
	.login-component > *:first-child {
		margin-top: 8px;
	}
	.login-component > *:not(:first-child) {
		margin-bottom: 4px;
	}
	.login-component > *:last-child {
		margin-bottom: 8px;
	}
	.login-component > notebook {
		background: none;
	}
	.login-component .adaptive-errorlabel {
		margin-bottom: 8px;
	}
	.login-button {
		background-color: #7289DA;
		color: #FFFFFF;
	}
	.login-with {
		font-weight: bold;
		margin-bottom: 2px;
	}
	.login-decrypt-button {
		margin-left: 4px;
	}
`)

// const decryptMsg = `You've previously chosen to remember the token and may have
// used a password to encrypt it. This button unlocks that encrypted token and logs
// in using it.`

// NewComponent creates a new login Component.
func NewComponent(ctx context.Context, p *Page) *Component {
	c := Component{
		ctx:  ctx,
		page: p,
	}

	c.uiFile = gresources.New("login_component.ui")
	c.ViewStack = c.uiFile.GetRoot().(*adw.ViewStack)
	c.greeterStack = c.uiFile.GetComponent("GreeterStack").(*adw.ViewStack)
	c.LoadInfoLabel = c.uiFile.GetComponent("LoaderLabel").(*gtk.Label)

	//c.Loading = loading.NewPulsatingBar(loading.PulseFast | loading.PulseBarOSD)
	// loginWith := gtk.NewLabel("Login using:")
	// loginWith.AddCSSClass("login-with")
	// loginWith.SetXAlign(0)

	c.Methods = NewMethods(&c)

	c.Remember = newRememberMeBox(ctx, c.uiFile.GetComponent("Remember").(*gtk.CheckButton))

	// c.ErrorRev = gtk.NewRevealer()
	// c.ErrorRev.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	// c.ErrorRev.SetRevealChild(false)
	c.ErrorRev = c.uiFile.GetComponent("ErrorRev").(*adw.Banner)

	c.LogIn = c.uiFile.GetComponent("LogIn").(*gtk.Button)
	// c.LogIn = gtk.NewButtonWithLabel("Log In")
	// c.LogIn.AddCSSClass("suggested-action")
	// c.LogIn.AddCSSClass("login-button")
	// c.LogIn.SetHExpand(true)
	c.LogIn.ConnectClicked(c.login)

	decrypt := c.uiFile.GetComponent("Decrypt").(*gtk.Button)
	// decrypt.AddCSSClass("login-decrypt-button")
	// decrypt.SetSensitive(false)
	// decrypt.SetTooltipText(strings.ReplaceAll(decryptMsg, "\n", " "))
	decrypt.ConnectClicked(c.askDecrypt)

	// buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	// buttonBox.Append(c.LogIn)
	// buttonBox.Append(decrypt)

	gtkutil.Async(ctx, func() func() {
		if secret.IsEncrypted(ctx) {
			return func() { c.greeterStack.SetVisibleChildName("encryptedPrompt") }
		} else {
			return func() { c.greeterStack.SetVisibleChildName("welcomePage") }
		}
	})

	// c.Inner = gtk.NewBox(gtk.OrientationVertical, 0)
	// c.Inner.Append(loginWith)
	// c.Inner.Append(c.Methods)
	// c.Inner.Append(c.Remember)
	// c.Inner.Append(c.ErrorRev)
	// c.Inner.Append(buttonBox)
	// componentCSS(c.Inner)

	// c.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	// c.Box.AddCSSClass("login-component-outer")
	// c.Box.SetHAlign(gtk.AlignCenter)
	// c.Box.SetVAlign(gtk.AlignCenter)
	// c.Box.Append(c.Loading)
	// c.Box.Append(c.Inner)

	c.loginCarousel = c.uiFile.GetComponent("LoginCarousel").(*adw.Carousel)
	c.loginCarouselTarget = c.uiFile.GetComponent("LoginCarouselTarget").(*adw.ToolbarView)

	return &c
}

func (c *Component) ShowLoginPage() {
	c.loginCarousel.ScrollTo(c.loginCarouselTarget, true)
}

// ShowError reveals the error label and shows it to the user.
func (c *Component) ShowError(err error) {
	// errLabel := adaptive.NewErrorLabel(err)
	// c.ErrorRev.SetChild(errLabel)
	// c.ErrorRev.SetRevealChild(true)
	c.ErrorRev.SetTitle(err.Error())
	c.ErrorRev.SetRevealed(true)
}

// HideError hides the error label.
func (c *Component) HideError() {
	// c.ErrorRev.SetRevealChild(false)
	c.ErrorRev.SetRevealed(false)
}

// Login presses the Login button.
func (c *Component) Login() {
	c.LogIn.Activate()
}

func (c *Component) login() {
	switch {
	case c.Methods.IsEmail():
		c.loginEmail(
			c.Methods.Email.Email.Text(),
			c.Methods.Email.Password.Text(),
			c.Methods.Email.TOTP.Text(),
		)
	case c.Methods.IsToken():
		c.loginToken(
			c.Methods.Token.Token.Text(),
		)
	}
}

func (c *Component) SetBusy(loadInfo ...string) {
	c.pageNameBeforeLoading = c.VisibleChildName()
	if len(loadInfo) > 0 {
		c.LoadInfoLabel.SetText(loadInfo[0])
		c.LoadInfoLabel.SetVisible(true)
	}
	c.SetVisibleChildName("loader")
}

func (c *Component) SetDone() {
	c.LoadInfoLabel.SetVisible(false)
	c.SetVisibleChildName(c.pageNameBeforeLoading)
}

func (c *Component) loginEmail(email, password, totp string) {
	c.SetBusy()

	gtkutil.Async(c.ctx, func() func() {
		u, err := session.Login(c.ctx, email, password, totp)
		if err != nil {
			return func() {
				c.ShowError(errors.Wrap(err, "cannot login"))
				c.SetDone()
			}
		}

		return func() {
			c.loginToken(u.Token)
			c.SetDone()
		}
	})
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

// Methods is the notebook containing entries for different login methods.
type Methods struct {
	Email struct { // Username and Password
		*gtk.Box
		Email    *adw.EntryRow
		Password *adw.PasswordEntryRow
		TOTP     *adw.EntryRow
	}
	Token struct { // Token
		*gtk.Box
		Token *adw.PasswordEntryRow
	}
	AuthTabs *adw.ToggleGroup
}

var methodsCSS = cssutil.Applier("login-methods", `
	.login-methods > * {
		margin: 0;
	}
	.login-methods > header > tabs > tab {
		min-width: 0;
		padding-left:  8px;
		padding-right: 8px;
	}
	.login-methods > stack {
		padding: 0 4px;
	}
	.login-methods .login-formentry {
		margin-top: 8px;
	}
	.login-methods header tab:checked {
		background-color: @accent_color;
	}
	.login-form-2fa {
		margin-left: 6px;
	}
	.login-form-2fa entry {
		font-family: monospace;
	}
`)

// NewMethods creates a new Methods widget.
func NewMethods(c *Component) *Methods {
	m := Methods{}

	m.Email.Email = c.uiFile.GetComponent("Email").(*adw.EntryRow)
	// m.Email.Email.AddCSSClass("login-form-email")
	// m.Email.Email.ConnectActivate(m.FocusNext)
	// m.Email.Email.Entry.SetInputPurpose(gtk.InputPurposeEmail)

	m.Email.Password = c.uiFile.GetComponent("Password").(*adw.PasswordEntryRow)
	// m.Email.Password.AddCSSClass("login-form-password")
	// m.Email.Password.SetHExpand(true)
	// m.Email.Password.FocusNextOnActivate()
	// m.Email.Password.Entry.SetInputPurpose(gtk.InputPurposePassword)
	// m.Email.Password.Entry.SetVisibility(false)

	m.Email.TOTP = c.uiFile.GetComponent("TOTP").(*adw.EntryRow)
	// m.Email.TOTP.AddCSSClass("login-form-2fa")
	m.Email.TOTP.ConnectActivate(c.Login)
	// m.Email.TOTP.Entry.SetInputPurpose(gtk.InputPurposePIN)
	// m.Email.TOTP.Entry.SetPlaceholderText("000000")
	// m.Email.TOTP.Entry.SetMaxLength(6)
	// m.Email.TOTP.Entry.SetWidthChars(6)

	// Hack to collapse the TOTP entry.
	// if text, ok := m.Email.TOTP.Entry.FirstChild().(*gtk.Text); ok {
	// 	text.SetPropagateTextWidth(true)
	// }

	//   [  0  |  1  |  2  |  3  ]
	// 0 [ Email                 ]
	// 1 [ Password        ][TOTP]
	// passwordBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	// passwordBox.Append(m.Email.Password)
	// passwordBox.Append(m.Email.TOTP)

	// m.Email.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	// m.Email.Append(m.Email.Email)
	// m.Email.Append(passwordBox)

	m.Token.Token = c.uiFile.GetComponent("Token").(*adw.PasswordEntryRow)
	m.Token.Token.ConnectActivate(c.Login)
	// m.Token.Token.Entry.SetInputPurpose(gtk.InputPurposePassword)
	// m.Token.Token.Entry.SetVisibility(false)

	// m.Token.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	// m.Token.SetVAlign(gtk.AlignStart)
	// m.Token.Append(m.Token.Token)
	m.AuthTabs = c.uiFile.GetComponent("AuthTabs").(*adw.ToggleGroup)

	// m.Notebook = gtk.NewNotebook()
	// m.Notebook.SetShowBorder(false)
	// m.Notebook.AppendPage(m.Token, gtk.NewLabel("Token"))
	// m.Notebook.AppendPage(m.Email, gtk.NewLabel("Email"))
	// m.Notebook.SetCurrentPage(0)

	// if stack, ok := m.Notebook.LastChild().(*gtk.Stack); ok {
	// 	stack.SetTransitionType(gtk.StackTransitionTypeSlideLeftRight)
	// }

	// methodsCSS(m)
	return &m
}

func (m *Methods) IsEmail() bool { return m.AuthTabs.Active() == 1 }
func (m *Methods) IsToken() bool { return m.AuthTabs.Active() == 0 }

// FormEntry is a widget containing a label and an entry.
// type FormEntry struct {
// 	*gtk.Box
// 	Label *gtk.Label
// 	Entry *gtk.Entry
// }

// var formEntryCSS = cssutil.Applier("login-formentry", ``)

// NewFormEntry creates a new FormEntry.
// func NewFormEntry(label string) *FormEntry {
// 	e := FormEntry{}
// 	e.Label = gtk.NewLabel(label)
// 	e.Label.SetXAlign(0)

// 	e.Entry = gtk.NewEntry()
// 	e.Entry.SetVExpand(true)
// 	e.Entry.SetHasFrame(true)

// 	e.Box = gtk.NewBox(gtk.OrientationVertical, 0)
// 	e.Box.Append(e.Label)
// 	e.Box.Append(e.Entry)
// 	formEntryCSS(e)

// 	return &e
// }

// Text gets the value entry.
// func (e *FormEntry) Text() string { return e.Entry.Text() }

// FocusNext navigates to the next widget.
// func (e *FormEntry) FocusNext() {
// 	e.Entry.Emit("move-focus", gtk.DirTabForward)
// }

// FocusNextOnActivate binds Enter to navigate to the next widget when it's
// pressed.
// func (e *FormEntry) FocusNextOnActivate() {
// 	e.Entry.ConnectActivate(e.FocusNext)
// }

// ConnectActivate connects the activate signal hanlder to the Entry.
// func (e *FormEntry) ConnectActivate(f func()) {
// 	e.Entry.ConnectActivate(f)
// }
