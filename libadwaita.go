//go:build !nolibadwaita

package main

import (
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotkit/app"
)

func init() {
	app.Hook(func(app *app.Application) {
		app.ConnectActivate(adw.Init)
	})
}
