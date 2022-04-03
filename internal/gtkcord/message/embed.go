package message

import (
	"context"
	"fmt"
	"html"
	"path"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/thumbnail"
	"github.com/diamondburned/chatkit/md/mdrender"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3/discordmd"
	"github.com/dustin/go-humanize"
)

func newAttachment(ctx context.Context, attachment *discord.Attachment) gtk.Widgetter {
	if attachment.ContentType != "" {
		typ := strings.SplitN(attachment.ContentType, "/", 2)[0]
		if typ == "image" || typ == "video" {
			// Make this attachment like an image embed.
			var embedType thumbnail.EmbedType

			switch {
			case attachment.ContentType == "image/gif":
				embedType = thumbnail.EmbedTypeGIF
			case typ == "image":
				embedType = thumbnail.EmbedTypeImage
			case typ == "video":
				embedType = thumbnail.EmbedTypeVideo
			}

			w := gtkcord.EmbedMaxWidth
			h := gtkcord.EmbedImgHeight

			if attachment.Width > 0 && attachment.Height > 0 {
				w, h = imgutil.MaxSize(
					int(attachment.Width),
					int(attachment.Height),
					w, h,
				)
			}

			image := thumbnail.NewEmbed(embedType, w, h)
			image.SetName(fmt.Sprintf(
				"%s (%s)",
				attachment.Filename, humanize.Bytes(attachment.Size),
			))
			image.SetFromURL(ctx, attachment.Proxy)
			image.SetOpenURL(func() { app.OpenURI(ctx, attachment.URL) })

			return image
		}
	}

	return gtk.NewBox(gtk.OrientationVertical, 0)
}

func newEmbed(ctx context.Context, msg *discord.Message, embed *discord.Embed) gtk.Widgetter {
	switch embed.Type {
	case discord.ImageEmbed, discord.VideoEmbed, discord.GIFVEmbed:
		return newImageEmbed(ctx, msg, embed)
	case discord.NormalEmbed, discord.ArticleEmbed, discord.LinkEmbed:
		fallthrough
	default:
		return newNormalEmbed(ctx, msg, embed)
	}
}

func newImageEmbed(ctx context.Context, msg *discord.Message, embed *discord.Embed) gtk.Widgetter {
	var typ thumbnail.EmbedType
	var img discord.EmbedImage

	switch {
	case embed.Image != nil:
		img = *embed.Image
	case embed.Thumbnail != nil:
		img = discord.EmbedImage(*embed.Thumbnail)
	default:
		return newNormalEmbed(ctx, msg, embed)
	}

	switch embed.Type {
	case discord.ImageEmbed:
		typ = thumbnail.TypeFromURL(img.URL)
	case discord.VideoEmbed:
		typ = thumbnail.EmbedTypeVideo
	case discord.GIFVEmbed:
		typ = thumbnail.EmbedTypeGIF
	}

	w, h := imgutil.MaxSize(
		int(img.Width),
		int(img.Height),
		gtkcord.EmbedMaxWidth,
		gtkcord.EmbedImgHeight,
	)

	image := thumbnail.NewEmbed(typ, w, h)
	image.SetName(path.Base(img.URL))
	image.SetFromURL(ctx, img.Proxy)
	image.SetOpenURL(func() { app.OpenURI(ctx, img.URL) })

	return image
}

var normalEmbedCSS = cssutil.Applier("message-normalembed", `
	.message-normalembed {
		border-left: 3px solid;
		padding: 5px 10px;
	}
`)

const embedColorCSSf = `
	.message-normalembed {
		border-color: %s;
	}
`

func newNormalEmbed(ctx context.Context, msg *discord.Message, embed *discord.Embed) gtk.Widgetter {
	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.AddCSSClass("message-normalembed-body")
	content.SetHExpand(true)

	// used for calculating requested embed width
	// widthHint := 0

	if embed.Author != nil {
		box := gtk.NewBox(gtk.OrientationHorizontal, 0)
		box.AddCSSClass("message-embed-author")

		if embed.Author.ProxyIcon != "" {
			img := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, 24)
			img.AddCSSClass("message-embed-author-icon")
			img.SetFromURL(embed.Author.ProxyIcon)

			box.Append(img)
		}

		if embed.Author.Name != "" {
			author := gtk.NewLabel(embed.Author.Name)
			author.SetUseMarkup(true)
			author.SetSingleLineMode(true)
			author.SetEllipsize(pango.EllipsizeEnd)
			author.SetTooltipText(embed.Author.Name)
			author.SetXAlign(0)

			if embed.Author.URL != "" {
				author.SetMarkup(fmt.Sprintf(
					`<a href="%s">%s</a>`,
					html.EscapeString(embed.Author.URL), html.EscapeString(embed.Author.Name),
				))
			}

			box.Append(author)
		}

		content.Append(box)
	}

	if embed.Title != "" {
		title := `<span weight="heavy">` + html.EscapeString(embed.Title) + `</span>`
		if embed.URL != "" {
			title = fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(embed.URL), title)
		}

		label := gtk.NewLabel("")
		label.AddCSSClass("message-embed-title")
		label.SetWrap(true)
		label.SetWrapMode(pango.WrapWordChar)
		label.SetXAlign(0)
		label.SetMarkup(title)

		content.Append(label)
	}

	if embed.Description != "" {
		state := gtkcord.FromContext(ctx)
		edesc := []byte(embed.Description)
		mnode := discordmd.ParseWithMessage(edesc, *state.Cabinet, msg, false)

		v := mdrender.NewMarkdownViewer(ctx, edesc, mnode)
		v.AddCSSClass("message-embed-description")

		content.Append(v)
	}

	if len(embed.Fields) > 0 {
		fields := gtk.NewGrid()
		fields.AddCSSClass("message-embed-fields")
		fields.SetRowSpacing(uint(7))
		fields.SetColumnSpacing(uint(14))

		content.Append(fields)

		col, row := 0, 0

		for _, field := range embed.Fields {
			text := gtk.NewLabel("")
			text.SetEllipsize(pango.EllipsizeEnd)
			text.SetXAlign(0.0)
			text.SetMarkup(fmt.Sprintf(
				`<span weight="heavy">%s</span>`+"\n"+`<span weight="light">%s</span>`,
				html.EscapeString(field.Name),
				html.EscapeString(field.Value),
			))
			text.SetTooltipText(field.Name + "\n" + field.Value)

			// I have no idea what this does. It's just improvised.
			if field.Inline && col < 3 {
				fields.Attach(text, col, row, 1, 1)
				col++
			} else {
				if col > 0 {
					row++
				}

				col = 0
				fields.Attach(text, col, row, 1, 1)

				if !field.Inline {
					row++
				} else {
					col++
				}
			}
		}
	}

	if embed.Footer != nil || embed.Timestamp.IsValid() {
		footer := gtk.NewBox(gtk.OrientationHorizontal, 0)
		footer.AddCSSClass("message-embed-footer")

		if embed.Footer != nil {
			if embed.Footer.ProxyIcon != "" {
				img := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, 24)
				img.AddCSSClass("message-embed-footer-icon")

				footer.Append(img)
			}

			if embed.Footer.Text != "" {
				text := gtk.NewLabel(embed.Footer.Text)
				text.SetVAlign(gtk.AlignStart)
				text.SetOpacity(0.65)
				text.SetSingleLineMode(true)
				text.SetEllipsize(pango.EllipsizeEnd)
				text.SetTooltipText(embed.Footer.Text)
				text.SetXAlign(0)

				footer.Append(text)
			}
		}

		if embed.Timestamp.IsValid() {
			time := locale.TimeAgo(ctx, embed.Timestamp.Time())

			text := gtk.NewLabel(time)
			text.AddCSSClass("message-embed-timestamp")
			if embed.Footer != nil {
				text.SetText(" - " + time)
			}

			footer.Append(text)
		}

		content.Append(footer)
	}

	if embed.Image != nil || embed.Video != nil {
		w := gtkcord.EmbedMaxWidth
		h := gtkcord.EmbedImgHeight

		// set width hint to resize embeds accordingly
		// widthHint = w

		var typ thumbnail.EmbedType
		var url discord.URL

		switch {
		case embed.Image != nil:
			w, h = imgutil.MaxSize(
				int(embed.Image.Width),
				int(embed.Image.Height),
				w, h,
			)
			typ = thumbnail.TypeFromURL(embed.Image.URL)
			url = embed.Image.Proxy
		case embed.Video != nil:
			w, h = imgutil.MaxSize(
				int(embed.Video.Width),
				int(embed.Video.Height),
				w, h,
			)
			typ = thumbnail.EmbedTypeVideo
			url = "" // TODO
		}

		img := thumbnail.NewEmbed(typ, w, h)
		img.SetFromURL(ctx, url)
		img.SetOpenURL(func() { app.OpenURI(ctx, embed.Image.URL) })

		content.Append(img)
	}

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.SetHAlign(gtk.AlignStart)
	box.SetSizeRequest(gtkcord.EmbedMaxWidth, -1)
	box.Append(content)
	normalEmbedCSS(box)

	if embed.Color != discord.NullColor {
		cssutil.Applyf(box, embedColorCSSf, embed.Color.String())
	}

	if embed.Thumbnail != nil {
		w, h := imgutil.MaxSize(
			int(embed.Thumbnail.Width),
			int(embed.Thumbnail.Height),
			80, 80,
		)

		img := thumbnail.NewEmbed(thumbnail.EmbedTypeImage, w, h)
		img.SetHAlign(gtk.AlignEnd)
		img.SetFromURL(ctx, embed.Thumbnail.Proxy)
		img.SetOpenURL(func() { app.OpenURI(ctx, embed.Thumbnail.URL) })

		box.Append(img)
	}

	// Calculate the embed width without padding:
	// var w = clampWidth(variables.EmbedMaxWidth)
	// if widthHint > 0 && w > widthHint {
	// 	w = widthHint
	// }

	return box
}
