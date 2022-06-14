package message

import (
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/pkg/errors"
)

var showBlockedMessages = prefs.NewBool(false, prefs.PropMeta{
	Name:        "Show Blocked Messages",
	Section:     "Discord",
	Description: "Show messages from blocked users as dimmed instead of completely hidden.",
})

func init() {
	prefs.RegisterProp((*blockedUsersPrefs)(nil))
	prefs.Order((*blockedUsersPrefs)(nil), showBlockedMessages)
}

var _ = cssutil.WriteCSS(`
	.message-blockedusers-expander {
		margin-top: 4px;
	}
	.message-blockedusers-expander expander {
		min-width:  16px;
		min-height: 16px;
		padding: 4px;
	}
	.message-blockedusers {
		font-size: 0.95em;
		margin-left: 24px;
	}
	.message-blockedusers > *:not(:first-child) {
		margin-top: 4px;
	}
	.message-blockedusers button {
		padding: 4px 8px;
	}
`)

type blockedUsersPrefs struct{}

func (*blockedUsersPrefs) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

func (*blockedUsersPrefs) UnmarshalJSON(b []byte) error {
	if string(b) != "null" {
		return fmt.Errorf("unexpected %q, expecting null", b)
	}
	return nil
}

func (*blockedUsersPrefs) Meta() prefs.PropMeta {
	return prefs.PropMeta{
		Name:        "Blocked Users",
		Section:     "Discord",
		Description: "List of users whose messages you won't see.",
	}
}

// Pubsubber panics. Do not call this method.
func (*blockedUsersPrefs) Pubsubber() *prefs.Pubsub {
	panic("BUG: accidental call to Pubsubber")
}

func (*blockedUsersPrefs) CreateWidget(ctx context.Context, _ func()) gtk.Widgetter {
	state := gtkcord.FromContext(ctx)
	if state == nil {
		panic("BUG: context not passed down properly to prefui")
	}

	var blockedList *gtk.Box

	expander := gtk.NewExpander("Show")
	expander.AddCSSClass("message-blockedusers-expander")
	expander.SetResizeToplevel(true)
	expander.SetExpanded(false)
	expander.NotifyProperty("expanded", func() {
		if !expander.Expanded() {
			expander.SetLabel("Show")
			blockedList.Unparent()
			blockedList = nil
			return
		}

		blockedList = gtk.NewBox(gtk.OrientationVertical, 0)
		blockedList.AddCSSClass("message-blockedusers")

		for _, user := range state.RelationshipState.BlockedUsers() {
			user := user

			unblock := gtk.NewButtonWithLabel("Unblock")
			name := gtk.NewLabel(user.Tag())
			name.SetHExpand(true)
			name.SetXAlign(0)
			name.SetSelectable(true)

			box := gtk.NewBox(gtk.OrientationHorizontal, 0)
			box.Append(name)
			box.Append(unblock)

			blockedList.Append(box)

			unblock.ConnectClicked(func() {
				// Ensure that the user is still blocked.
				if !state.RelationshipState.IsBlocked(user.ID) {
					return
				}

				box.SetSensitive(false)
				gtkutil.Async(ctx, func() func() {
					err := state.DeleteRelationship(user.ID)

					return func() {
						if err != nil {
							box.SetSensitive(true)
							app.Error(ctx, errors.Wrapf(err, "cannot unblock user %s", user.Tag()))
						} else {
							blockedList.Remove(box)
						}
					}
				})
			})
		}

		expander.SetLabel("Hide")
		expander.SetChild(blockedList)
	})

	return expander
}

func (*blockedUsersPrefs) WidgetIsLarge() bool {
	return true
}
