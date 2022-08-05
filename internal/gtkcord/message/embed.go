package message

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/embed"
	"github.com/diamondburned/chatkit/md/mdrender"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3/discordmd"
	"github.com/dustin/go-humanize"
)

// TODO: allow disable fetching videos.

func resizeURL(image *embed.Embed, proxyURL string, w, h int) string {
	imgW, imgH := image.Size()
	if imgW == 0 || imgH == 0 {
		return proxyURL
	}

	// Grab the maximum scale factor that we've ever seen. Plugging in another
	// monitor while we've already rendered will not update this, but it's good
	// enough. We just don't want to abuse bandwidth for 1x or 2x people.
	scale := gtkutil.ScaleFactor()
	if scale == 1 {
		// Fetching 2x shouldn't be too bad, though.
		scale = 2
	}

	imgW *= scale
	imgH *= scale

	if imgW > w || imgH > h {
		return proxyURL
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return proxyURL
	}

	w = imgW
	h = imgH

	q := u.Query()
	q.Set("width", strconv.Itoa(w))
	q.Set("height", strconv.Itoa(h))
	u.RawQuery = q.Encode()

	return u.String()
}

var stickerCSS = cssutil.Applier("message-sticker", `
	.message-sticker {
		border-radius: 0;
	}
	.message-sticker picture.thumbnail-embed-image {
		background-color: transparent;
	}
`)

func newSticker(ctx context.Context, sticker *discord.StickerItem) gtk.Widgetter {
	switch sticker.FormatType {
	case discord.StickerFormatAPNG, discord.StickerFormatPNG:
		url := sticker.StickerURLWithType(discord.PNGImage)

		image := embed.New(ctx, gtkcord.StickerSize, gtkcord.StickerSize, embed.Opts{})
		image.SetName(sticker.Name)
		image.SetSizeRequest(gtkcord.StickerSize, gtkcord.StickerSize)
		image.SetFromURL(url)
		image.SetOpenURL(func() { app.OpenURI(ctx, url) })
		stickerCSS(image)
		return image
	default:
		// Fuck Lottie, whatever the fuck that is.
		msg := gtk.NewLabel(fmt.Sprintf("[Lottie sticker: %s]", sticker.Name))
		msg.SetXAlign(0)
		systemContentCSS(msg)
		fixNatWrap(msg)
		return msg
	}
}

var _ = cssutil.WriteCSS(`
	.message-richframe:not(:first-child) {
		margin-top: 4px;
	}
`)

func newAttachment(ctx context.Context, attachment *discord.Attachment) gtk.Widgetter {
	if attachment.ContentType != "" {
		typ := strings.SplitN(attachment.ContentType, "/", 2)[0]
		if typ == "image" || typ == "video" {
			// Make this attachment like an image embed.
			opts := embed.Opts{}

			switch {
			case attachment.ContentType == "image/gif":
				opts.Type = embed.EmbedTypeGIF
			case typ == "image":
				opts.Type = embed.EmbedTypeImage
			case typ == "video":
				opts.Type = embed.EmbedTypeVideo
				// Use FFmpeg for video so we can get the thumbnail.
				opts.Provider = imgutil.FFmpegProvider
			}

			name := fmt.Sprintf(
				"%s (%s)",
				attachment.Filename,
				humanize.Bytes(attachment.Size),
			)

			image := embed.New(ctx, gtkcord.EmbedMaxWidth, gtkcord.EmbedImgHeight, opts)
			image.SetLayoutManager(gtk.NewBinLayout())
			image.AddCSSClass("message-richframe")
			image.SetHAlign(gtk.AlignStart)
			image.SetHExpand(false)
			image.SetName(name)
			image.SetOpenURL(func() {
				switch opts.Type {
				case embed.EmbedTypeVideo:
					image.ActivateDefault()
				default:
					app.OpenURI(ctx, attachment.URL)
				}
			})

			if attachment.Width > 0 && attachment.Height > 0 {
				image.SetSizeRequest(int(attachment.Width), int(attachment.Height))
				if typ == "image" {
					image.SetFromURL(resizeURL(
						image,
						attachment.Proxy,
						int(attachment.Width),
						int(attachment.Height),
					))
				} else {
					image.SetFromURL(attachment.Proxy)
				}
			} else {
				image.SetFromURL(attachment.Proxy)
			}

			return image
		}
	}

	return gtk.NewBox(gtk.OrientationVertical, 0)
}

var normalEmbedCSS = cssutil.Applier("message-normalembed", `
	.message-normalembed {
		border-left: 3px solid;
		padding: 5px 10px;
		background-color: mix(@theme_bg_color, @theme_fg_color, 0.10);
	}
	.message-normalembed-body > *:not(:last-child) {
		margin-bottom: 4px;
	}
	.message-embed-author,
	.message-embed-description {
		font-size: 0.9em;
	}
`)

const embedColorCSSf = `
	.message-normalembed {
		border-color: %s;
	}
`

func newEmbed(ctx context.Context, msg *discord.Message, embed *discord.Embed) gtk.Widgetter {
	return newNormalEmbed(ctx, msg, embed)
}

func newNormalEmbed(ctx context.Context, msg *discord.Message, msgEmbed *discord.Embed) gtk.Widgetter {
	bodyBox := gtk.NewBox(gtk.OrientationVertical, 0)
	bodyBox.SetHAlign(gtk.AlignStart)
	bodyBox.SetHExpand(false)
	bodyBox.AddCSSClass("message-normalembed-body")

	// Track whether or not we have an embed body. An embed body should have any
	// kind of text in it. If we don't have a body but do have a thumbnail, then
	// the thumbnail should be big and on its own.
	hasBody := false

	if msgEmbed.Author != nil {
		box := gtk.NewBox(gtk.OrientationHorizontal, 0)
		box.AddCSSClass("message-embed-author")

		if msgEmbed.Author.ProxyIcon != "" {
			img := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, 24)
			img.AddCSSClass("message-embed-author-icon")
			img.SetFromURL(msgEmbed.Author.ProxyIcon)

			box.Append(img)
		}

		if msgEmbed.Author.Name != "" {
			author := gtk.NewLabel(msgEmbed.Author.Name)
			author.SetUseMarkup(true)
			author.SetSingleLineMode(true)
			author.SetEllipsize(pango.EllipsizeEnd)
			author.SetTooltipText(msgEmbed.Author.Name)
			author.SetXAlign(0)

			if msgEmbed.Author.URL != "" {
				author.SetMarkup(fmt.Sprintf(
					`<a href="%s">%s</a>`,
					html.EscapeString(msgEmbed.Author.URL), html.EscapeString(msgEmbed.Author.Name),
				))
			}

			box.Append(author)
		}

		bodyBox.Append(box)
		hasBody = true
	}

	if msgEmbed.Title != "" {
		title := `<span weight="heavy">` + html.EscapeString(msgEmbed.Title) + `</span>`
		if msgEmbed.URL != "" {
			title = fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(msgEmbed.URL), title)
		}

		label := gtk.NewLabel("")
		label.AddCSSClass("message-embed-title")
		label.SetWrap(true)
		label.SetWrapMode(pango.WrapWordChar)
		label.SetXAlign(0)
		label.SetMarkup(title)
		fixNatWrap(label)

		bodyBox.Append(label)
		hasBody = true
	}

	if msgEmbed.Description != "" {
		state := gtkcord.FromContext(ctx)
		edesc := []byte(msgEmbed.Description)
		mnode := discordmd.ParseWithMessage(edesc, *state.Cabinet, msg, false)

		v := mdrender.NewMarkdownViewer(ctx, edesc, mnode)
		v.AddCSSClass("message-embed-description")
		v.SetHExpand(false)

		bodyBox.Append(v)
		hasBody = true
	}

	if len(msgEmbed.Fields) > 0 {
		fields := gtk.NewGrid()
		fields.AddCSSClass("message-embed-fields")
		fields.SetRowSpacing(uint(7))
		fields.SetColumnSpacing(uint(14))

		bodyBox.Append(fields)
		hasBody = true

		col, row := 0, 0

		for _, field := range msgEmbed.Fields {
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

	if msgEmbed.Footer != nil || msgEmbed.Timestamp.IsValid() {
		footer := gtk.NewBox(gtk.OrientationHorizontal, 0)
		footer.AddCSSClass("message-embed-footer")

		if msgEmbed.Footer != nil {
			if msgEmbed.Footer.ProxyIcon != "" {
				img := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, 24)
				img.AddCSSClass("message-embed-footer-icon")

				footer.Append(img)
			}

			if msgEmbed.Footer.Text != "" {
				text := gtk.NewLabel(msgEmbed.Footer.Text)
				text.SetVAlign(gtk.AlignStart)
				text.SetOpacity(0.65)
				text.SetSingleLineMode(true)
				text.SetEllipsize(pango.EllipsizeEnd)
				text.SetTooltipText(msgEmbed.Footer.Text)
				text.SetXAlign(0)

				footer.Append(text)
			}
		}

		if msgEmbed.Timestamp.IsValid() {
			time := locale.TimeAgo(ctx, msgEmbed.Timestamp.Time())

			text := gtk.NewLabel(time)
			text.AddCSSClass("message-embed-timestamp")
			if msgEmbed.Footer != nil {
				text.SetText(" - " + time)
			}

			footer.Append(text)
		}

		bodyBox.Append(footer)
		hasBody = true
	}

	embedBox := bodyBox
	if hasBody {
		// bodyBox.SetHAlign(gtk.AlignFill)
		// bodyBox.SetHExpand(false)

		embedBox = gtk.NewBox(gtk.OrientationHorizontal, 0)
		embedBox.SetHExpand(false)
		embedBox.SetHAlign(gtk.AlignStart)
		embedBox.Append(bodyBox)
		normalEmbedCSS(embedBox)

		if msgEmbed.Color != discord.NullColor {
			cssutil.Applyf(embedBox, embedColorCSSf, msgEmbed.Color.String())
		}
	}

	if msgEmbed.Thumbnail != nil {
		big := !hasBody ||
			msgEmbed.Type == discord.GIFVEmbed ||
			msgEmbed.Type == discord.ImageEmbed ||
			msgEmbed.Type == discord.VideoEmbed ||
			msgEmbed.Type == discord.ArticleEmbed

		maxW := 80
		maxH := 80
		if big {
			maxW = gtkcord.EmbedMaxWidth
			maxH = gtkcord.EmbedImgHeight
		}

		var opts embed.Opts
		switch msgEmbed.Type {
		case discord.NormalEmbed, discord.ImageEmbed:
			opts.Type = embed.TypeFromURL(msgEmbed.Thumbnail.Proxy)
		case discord.VideoEmbed:
			opts.Type = embed.EmbedTypeVideo
		case discord.GIFVEmbed:
			opts.Type = embed.EmbedTypeGIFV
		}

		image := embed.New(ctx, maxW, maxH, opts)
		image.SetVAlign(gtk.AlignStart)
		image.SetSizeRequest(int(msgEmbed.Thumbnail.Width), int(msgEmbed.Thumbnail.Height))
		image.SetFromURL(resizeURL(
			image,
			msgEmbed.Thumbnail.Proxy,
			int(msgEmbed.Thumbnail.Width),
			int(msgEmbed.Thumbnail.Height),
		))

		switch {
		case msgEmbed.Image != nil:
			image.SetName(path.Base(msgEmbed.Image.URL))
		case msgEmbed.Video != nil:
			image.SetName(path.Base(msgEmbed.Video.URL))
		default:
			image.SetName(path.Base(msgEmbed.Thumbnail.URL))
		}

		image.SetOpenURL(func() {
			// See if we have either an Image or a Video. If we do, then use
			// that instead.
			switch {
			case msgEmbed.Image != nil:
				// Open the Image proxy instead of the Thumbnail proxy. Honestly
				// have no idea what the difference is.
				app.OpenURI(ctx, msgEmbed.Image.Proxy)
				return
			case msgEmbed.Video != nil:
				// Video doesn't have resizing, so we use the proxy URL
				// directly.
				image.SetFromURL(msgEmbed.Video.Proxy)
				image.ActivateDefault()
			}
		})

		if big {
			image.SetHAlign(gtk.AlignStart)
			bodyBox.Append(image)
		} else {
			image.SetHAlign(gtk.AlignEnd)
			embedBox.Append(image)
		}
	}

	if msgEmbed.Thumbnail == nil && (msgEmbed.Image != nil || msgEmbed.Video != nil) {
		var opts embed.Opts
		var img discord.EmbedImage

		switch {
		case msgEmbed.Image != nil:
			img = *msgEmbed.Image
			opts.Type = embed.TypeFromURL(msgEmbed.Image.URL)

		case msgEmbed.Video != nil:
			img = (discord.EmbedImage)(*msgEmbed.Video)
			opts.Type = embed.EmbedTypeVideo
			opts.Provider = imgutil.FFmpegProvider
		}

		image := embed.New(ctx, gtkcord.EmbedMaxWidth, gtkcord.EmbedImgHeight, opts)
		image.SetSizeRequest(int(img.Width), int(img.Height))
		image.SetOpenURL(func() { app.OpenURI(ctx, msgEmbed.URL) })

		if msgEmbed.Image != nil {
			// The server can only resize images.
			image.SetFromURL(resizeURL(
				image,
				img.Proxy,
				int(img.Width),
				int(img.Height),
			))
		} else {
			image.SetFromURL(img.Proxy)
		}

		bodyBox.Append(image)
	}

	embedBox.AddCSSClass("message-richframe")
	return embedBox
}

func fixNatWrap(label *gtk.Label) {
	if err := gtk.CheckVersion(4, 6, 0); err == "" {
		label.SetObjectProperty("natural-wrap-mode", 1) // NaturalWrapNone
	}
}
