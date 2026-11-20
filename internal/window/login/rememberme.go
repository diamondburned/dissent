package login

import (
	"context"

	"github.com/diamondburned/chatkit/components/secretdialog"
	"github.com/diamondburned/chatkit/kits/secret"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type rememberMeBox struct {
	action *gio.SimpleAction
	driver secret.Driver
}

func newRememberMeBox(ctx context.Context, target gtk.Widgetter) *rememberMeBox {
	b := rememberMeBox{}
	keyring := secret.KeyringDriver(ctx)

	initial := keyring.IsAvailable()
	if initial {
		b.driver = keyring
	}

	b.action = gio.NewSimpleActionStateful("remember", nil, glib.NewVariantBoolean(initial))
	b.action.ConnectChangeState(func(value *glib.Variant) {
		active := value != nil && value.Boolean()
		b.action.SetState(glib.NewVariantBoolean(active))

		if !active {
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
					// User didn't want this, so untick the checkbox.
					b.action.SetState(glib.NewVariantBoolean(false))
					b.driver = nil
				} else {
					b.driver = enc
				}
			},
		)
	})

	group := gio.NewSimpleActionGroup()
	group.AddAction(b.action)
	gtk.BaseWidget(target).InsertActionGroup("login", group)

	return &b
}

// SecretDriver returns the secret driver if the user opts to remember the
// session.
func (b *rememberMeBox) SecretDriver() secret.Driver {
	return b.driver
}
