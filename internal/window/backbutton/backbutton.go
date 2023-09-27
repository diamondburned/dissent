package backbutton

import (
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// BackButtonIcon is the default icon name for a fold reveal button.
const BackButtonIcon = "sidebar-show-symbolic"

// BackButton is a button that toggles whether or not the fold's sidebar
// should be revealed.
type BackButton struct {
	*gtk.Revealer
	Button *gtk.ToggleButton
}

// New creates a new fold reveal button. The button is hidden by default until a
// sidebar is connected to it.
func New() *BackButton {
	button := gtk.NewToggleButton()
	button.SetIconName(BackButtonIcon)
	button.SetSensitive(false)
	button.SetHAlign(gtk.AlignCenter)
	button.SetVAlign(gtk.AlignCenter)

	revealer := gtk.NewRevealer()
	revealer.AddCSSClass("adaptive-sidebar-reveal-button")
	revealer.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	revealer.SetChild(button)
	revealer.SetRevealChild(false)

	return &BackButton{
		Revealer: revealer,
		Button:   button,
	}
}

// SetIconName sets the reveal button's icon name.
func (b *BackButton) SetIconName(icon string) {
	b.Button.SetIconName(icon)
}

// ConnectFold connects the current sidebar reveal button to the given
// sidebar.
func (b *BackButton) ConnectFlap(flap *adw.Flap) {
	b.Button.ConnectClicked(func() {
		flap.SetRevealFlap(b.Button.Active())
	})

	flap.NotifyProperty("reveal-flap", func() {
		b.Button.SetActive(flap.RevealFlap())
	})

	flap.NotifyProperty("folded", func() {
		folded := flap.Folded()
		b.SetRevealChild(folded)
		b.Button.SetActive(flap.RevealFlap())
		b.Button.SetSensitive(folded)
	})
}
