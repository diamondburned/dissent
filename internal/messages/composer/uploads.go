package composer

import (
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/dustin/go-humanize"
)

func mimeIsText(mime string) bool {
	// How is utf8_string a valid MIME type? GTK, what the fuck?
	return strings.HasPrefix(mime, "text") || mime == "utf8_string"
}

// UploadTray is the tray holding files to be uploaded.
type UploadTray struct {
	*gtk.Box
	files []uploadFile
}

type uploadFile struct {
	*gtk.Box
	icon *gtk.Image
	name *gtk.Label
	del  *gtk.Button

	file File
}

var uploadTrayCSS = cssutil.Applier("composer-upload-tray", `
	.composer-upload-tray {
	}
	.composer-upload-item > image {
		margin-bottom: 1px;
		margin-right:  6px;
	}
`)

// NewUploadTray creates a new UploadTray.
func NewUploadTray() *UploadTray {
	t := UploadTray{}
	t.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	uploadTrayCSS(t.Box)
	return &t
}

// AddFile adds a file into the tray.
func (t *UploadTray) AddFile(file File) {
	f := uploadFile{file: file}

	f.icon = gtk.NewImageFromIconName(mimeIcon(file.Type))

	f.name = gtk.NewLabel(file.Name)
	f.name.SetEllipsize(pango.EllipsizeMiddle)
	f.name.SetXAlign(0)
	f.name.SetHExpand(true)

	if file.Size > 0 {
		f.name.SetMarkup(fmt.Sprintf(
			`%s <span size="small" alpha="85%%">%s</span>`,
			html.EscapeString(file.Name), humanize.Bytes(uint64(file.Size)),
		))
	}

	f.del = gtk.NewButtonFromIconName("edit-clear-all-symbolic")
	f.del.SetHasFrame(false)
	f.del.SetTooltipText(locale.Get("Remove File"))

	// TODO: hover to preview?
	f.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	f.Box.AddCSSClass("composer-upload-item")
	f.Box.SetHExpand(true)
	f.Box.Append(f.icon)
	f.Box.Append(f.name)
	f.Box.Append(f.del)

	t.Box.Append(f)
	t.files = append(t.files, f)

	f.del.ConnectClicked(t.bindDelete(f))
}

func mimeIcon(mime string) string {
	if mime == "" {
		return "text-x-generic-symbolic"
	}

	switch strings.SplitN(mime, "/", 2)[0] {
	case "image":
		return "image-x-generic-symbolic"
	case "video":
		return "video-x-generic-symbolic"
	case "audio":
		return "audio-x-generic-symbolic"
	default:
		return "text-x-generic-symbolic"
	}
}

func (t *UploadTray) bindDelete(this uploadFile) func() {
	return func() {
		for i, f := range t.files {
			if f.Box == this.Box {
				t.Box.Remove(t.files[i])
				t.files = append(t.files[:i], t.files[i+1:]...)
				return
			}
		}
	}
}

// Files returns the list of files in the tray.
func (t *UploadTray) Files() []File {
	files := make([]File, len(t.files))
	for i, file := range t.files {
		files[i] = file.file
	}
	return files
}

// Clear clears the tray and returns the list of paths that it held.
func (t *UploadTray) Clear() []File {
	files := make([]File, len(t.files))
	for i, file := range t.files {
		files[i] = file.file
		t.Remove(file)
	}

	t.files = nil
	return files
}
