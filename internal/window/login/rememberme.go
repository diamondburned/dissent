package login

import (
	"context"

	"github.com/diamondburned/chatkit/components/secretdialog"
	"github.com/diamondburned/chatkit/kits/secret"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

type rememberMeBox struct {
	*gtk.CheckButton
	driver secret.Driver
}

var rememberMeCSS = cssutil.Applier("login-rememberme", `
	.login-rememberme {
		margin-bottom: 4px;
	}
`)

func newRememberMeBox(ctx context.Context, checkbox *gtk.CheckButton) *rememberMeBox {
	b := rememberMeBox{CheckButton: checkbox}

	keyring := secret.KeyringDriver(ctx)
	b.CheckButton.ConnectToggled(func() {
		if !b.Active() {
			b.driver = nil
			return
		}

		if keyring.IsAvailable() {
			b.driver = keyring
			return
		}

		secretdialog.PromptPassword(
			ctx, secretdialog.PromptEncrypt,
			func(ok bool, enc *secret.EncryptedFile) {
				if !ok {
					// User didn't want this, so untick the box.
					b.SetActive(false)
				} else {
					b.driver = enc
				}
			},
		)
	})

	return &b
}

// SecretDriver returns the secret driver if the user opts to remember the
// session.
func (b *rememberMeBox) SecretDriver() secret.Driver {
	return b.driver
}
