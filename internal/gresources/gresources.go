package gresources

import (
	_ "embed"
	"log"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
)
//go:generate glib-compile-resources --sourcedir=../../uifiles/ --target=uifiles.gresource ../../uifiles/uifiles.gresource.xml

//go:embed uifiles.gresource
var Resources []byte
var resourcePath string = "/so/libdb/dissent"

type UiFile struct {
	*gtk.Builder
}

func (ui *UiFile) GetComponent(cmpName string) glib.Objector {
	return ui.Builder.GetObject(cmpName).Cast()
}

func (ui *UiFile) GetRoot() glib.Objector {
	return ui.GetComponent("rootContent")
}


func Init() bool {
	resources, err := gio.NewResourceFromData(glib.NewBytesWithGo(Resources))
	if err != nil {
		log.Panicln("Failed to create resources: ", err)
		return false
	}
	gio.ResourcesRegister(resources)
	return true
}

func New(filename string) *UiFile {
	return &UiFile{gtk.NewBuilderFromResource(resourcePath + "/" + filename)}
}
