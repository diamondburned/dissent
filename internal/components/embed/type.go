package embed

import (
	"mime"
	"path"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// EmbedType indicates the type of the Embed being constructed. The type
// determines how it's displayed visually to the user.
type EmbedType uint8

const (
	_ EmbedType = iota // unknown type
	EmbedTypeImage
	EmbedTypeVideo
	EmbedTypeGIF
	EmbedTypeGIFV // video GIF
	EmbedTypeAudio
)

// IsGIF returns true if the URL is a GIF URL.
func IsGIF(url string) bool {
	return path.Ext(url) == ".gif"
}

// TypeFromURL returns the EmbedType from the URL.
func TypeFromURL(url string) EmbedType {
	mime := mime.TypeByExtension(path.Ext(url))
	if mime == "" {
		return 0 // dunno
	}
	if mime == "image/gif" {
		return EmbedTypeGIF
	}

	switch {
	case strings.HasPrefix(mime, "image/"):
		return EmbedTypeImage
	case strings.HasPrefix(mime, "video/"):
		return EmbedTypeVideo
	case strings.HasPrefix(mime, "audio/"):
		return EmbedTypeAudio
	}

	return 0
}

func (t EmbedType) IsLooped() bool {
	return t == EmbedTypeGIF || t == EmbedTypeGIFV
}

func (t EmbedType) IsMuted() bool {
	return t != EmbedTypeVideo && t != EmbedTypeAudio
}

func setTypeLabel(l *gtk.Label, t EmbedType) {
	switch t {
	case EmbedTypeGIF:
		l.SetLabel("GIF")
		l.SetVisible(true)
	case EmbedTypeGIFV:
		l.SetLabel("GIFV")
		l.SetVisible(true)
	default:
		l.SetVisible(false)
	}
}
