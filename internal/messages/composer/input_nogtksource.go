//go:build nogtksource || windows

package composer

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

func initializeInput() initializedInput {
	view := gtk.NewTextView()
	return initializedInput{
		View:   view,
		Buffer: view.Buffer(),
	}
}
