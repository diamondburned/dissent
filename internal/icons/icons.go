// Package icons generates and loads icon Gresource.
package icons

import (
	_ "embed"
	"log"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

//go:generate glib-compile-resources gtkcord4.gresource.xml

//go:embed gtkcord4.gresource
var Resources []byte

func LoadResources() {
	resources, err := gio.NewResourceFromData(glib.NewBytesWithGo(Resources))
	if err != nil {
		log.Panicln("Failed to create resources: ", err)
	}

	gio.ResourcesRegister(resources)
}
