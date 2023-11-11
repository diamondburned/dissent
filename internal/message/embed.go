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

var trustedCDNHosts = map[string]struct{}{
	"cdn.discordapp.com": {},
}

var defaultEmbedOpts = embed.Opts{
	IgnoreWidth: true,
}

func resizeURL(directURL, proxyURL string, w, h int) string {
	if w == 0 || h == 0 {
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

	u, err := url.Parse(proxyURL)
	if err != nil {
		return proxyURL
	}

	if direct, err := url.Parse(directURL); err == nil {
		// Special-case: sometimes, the URL is already a Discord CDN URL. In
		// that case, we'll just use it directly.
		if _, ok := trustedCDNHosts[direct.Host]; ok {
			u = direct
		}
	}

	q := u.Query()
	// Do we have a size parameter already? We might if the URL is one crafted
	// by us to fetch an emoji.
	if q.Has("size") {
		// If we even have a size, then we can just assume that the size is
		// the larger dimension.
		if w > h {
			q.Set("size", strconv.Itoa(w*scale))
		} else {
			q.Set("size", strconv.Itoa(h*scale))
		}
	} else {
		q.Set("width", strconv.Itoa(w*scale))
		q.Set("height", strconv.Itoa(h*scale))
	}

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

		// TODO: this is always round because we're using a GtkFrame. What the
		// heck? How does this shit even work?!
		image := embed.New(ctx, gtkcord.StickerSize, gtkcord.StickerSize, defaultEmbedOpts)
		image.SetName(sticker.Name)
		image.SetHAlign(gtk.AlignStart)
		image.SetSizeRequest(gtkcord.StickerSize, gtkcord.StickerSize)
		image.SetFromURL(url)
		image.SetOpenURL(func() { app.OpenURI(ctx, url) }) // TODO: Add sticker info popover
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

var messageAttachmentCSS = cssutil.Applier("message-attachment", `
	.message-attachment-filename {
		padding-left: 0.35em;
		padding-right: 0.35em;
	}
	.message-attachment-filesize {
		color: alpha(@theme_fg_color, 0.75);
	}
`)

func newAttachment(ctx context.Context, attachment *discord.Attachment) gtk.Widgetter {
	var mimeType string
	if attachment.ContentType != "" {
		mimeType, _, _ = strings.Cut(attachment.ContentType, "/")
	}

	switch mimeType {
	case "image", "video":
		// Make this attachment like an image embed.
		opts := defaultEmbedOpts

		switch {
		case attachment.ContentType == "image/gif":
			opts.Type = embed.EmbedTypeGIF
		case mimeType == "image":
			opts.Type = embed.EmbedTypeImage
		case mimeType == "video":
			opts.Type = embed.EmbedTypeVideo
			// Use FFmpeg for video so we can get the thumbnail.
			opts.Provider = imgutil.FFmpegProvider
		}

		name := fmt.Sprintf(
			"%s (%s)",
			attachment.Filename,
			humanize.Bytes(attachment.Size),
		)

		image := embed.New(ctx, maxEmbedWidth.Value(), maxImageHeight.Value(), opts)
		image.SetLayoutManager(gtk.NewBinLayout())
		image.AddCSSClass("message-richframe")
		image.SetHAlign(gtk.AlignStart)
		image.SetHExpand(false)
		image.SetName(name)

		image.SetOpenURL(func() {
			openViewer(ctx, attachment.URL, opts)
		})

		if attachment.Width > 0 && attachment.Height > 0 {
			origW := int(attachment.Width)
			origH := int(attachment.Height)

			// Work around to prevent GTK from rendering the image at its
			// original size, which tanks performance on Cairo renderers.
			w, h := imgutil.MaxSize(
				origW, origH,
				maxEmbedWidth.Value(), maxImageHeight.Value(),
			)

			image.SetSizeRequest(-1, h)
			image.Thumbnail.SetSizeRequest(-1, h)
			if mimeType == "image" {
				scale := gtkutil.ScaleFactor()
				w *= scale
				h *= scale

				image.SetFromURL(resizeURL(
					attachment.URL,
					attachment.Proxy,
					w, h,
				))
			} else {
				image.SetFromURL(attachment.Proxy)
			}
		} else {
			image.SetFromURL(attachment.Proxy)
		}

		return image
	default:
		icon := gtk.NewImageFromIconName(mimeIcon(mimeType))
		icon.AddCSSClass("message-attachment-icon")
		icon.SetIconSize(gtk.IconSizeNormal)

		filename := gtk.NewLabel("")
		filename.AddCSSClass("message-attachment-filename")
		filename.SetMarkup(fmt.Sprintf(
			`<a href="%s">%s</a>`,
			html.EscapeString(attachment.URL),
			html.EscapeString(attachment.Filename),
		))
		filename.SetEllipsize(pango.EllipsizeEnd)
		filename.SetXAlign(0)

		filesize := gtk.NewLabel(humanize.Bytes(attachment.Size))
		filesize.AddCSSClass("message-attachment-filesize")
		filesize.SetXAlign(0)

		box := gtk.NewBox(gtk.OrientationHorizontal, 0)
		box.SetTooltipText(attachment.Filename)
		box.Append(icon)
		box.Append(filename)
		box.Append(filesize)
		messageAttachmentCSS(box)

		return box
	}
}

func mimeIcon(mimePrefix string) string {
	switch mimePrefix {
	case "audio":
		return "audio-x-generic-symbolic"
	case "image":
		return "image-x-generic-symbolic"
	case "video":
		return "video-x-generic-symbolic"
	case "application":
		return "application-x-executable-symbolic"
	default:
		return "text-x-generic-symbolic"
	}
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
	bodyBox.SetHAlign(gtk.AlignFill)
	bodyBox.SetHExpand(true)
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
			time := locale.TimeAgo(msgEmbed.Timestamp.Time())

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
		embedBox.SetHAlign(gtk.AlignFill)
		embedBox.Append(bodyBox)
		normalEmbedCSS(embedBox)

		if msgEmbed.Color != discord.NullColor {
			cssutil.Applyf(embedBox, embedColorCSSf, msgEmbed.Color.String())
		}
	}

	if msgEmbed.Thumbnail != nil {
		thumb := msgEmbed.Thumbnail
		big := !hasBody ||
			msgEmbed.Type == discord.GIFVEmbed ||
			msgEmbed.Type == discord.ImageEmbed ||
			msgEmbed.Type == discord.VideoEmbed ||
			msgEmbed.Type == discord.ArticleEmbed

		maxW := 80
		maxH := 80
		if big {
			maxW = maxEmbedWidth.Value()
			maxH = maxImageHeight.Value()
		}

		opts := defaultEmbedOpts
		switch msgEmbed.Type {
		case discord.NormalEmbed, discord.ImageEmbed:
			opts.Type = embed.TypeFromURL(thumb.Proxy)
		case discord.VideoEmbed:
			opts.Type = embed.EmbedTypeVideo
		case discord.GIFVEmbed:
			opts.Type = embed.EmbedTypeGIFV
		}

		image := embed.New(ctx, maxW, maxH, opts)
		image.SetVAlign(gtk.AlignStart)
		if thumb.Width > 0 && thumb.Height > 0 {
			// Enforce this image's own dimensions if possible.
			image.ShrinkMaxSize(int(thumb.Width), int(thumb.Height))
			image.SetSizeRequest(int(thumb.Width), int(thumb.Height))
		}

		image.SetFromURL(resizeURL(
			thumb.URL,
			thumb.Proxy,
			int(thumb.Width),
			int(thumb.Height),
		))

		switch {
		case msgEmbed.Image != nil:
			image.SetName(path.Base(msgEmbed.Image.URL))
		case msgEmbed.Video != nil:
			image.SetName(path.Base(msgEmbed.Video.URL))
		default:
			image.SetName(path.Base(thumb.URL))
		}

		switch {
		case msgEmbed.Image != nil:
			// Open the Image proxy instead of the Thumbnail proxy. Honestly
			// have no idea what the difference is.
			image.SetOpenURL(func() {
				openViewer(ctx, msgEmbed.Image.Proxy, opts)
			})
		case msgEmbed.Video != nil:
			image.SetOpenURL(func() {
				// Some video URLs don't have direct links, like YouTube.
				if msgEmbed.Video.Proxy == "" {
					app.OpenURI(ctx, msgEmbed.Video.URL)
				} else {
					image.SetFromURL(msgEmbed.Video.Proxy)
					image.ActivateDefault()
				}
			})
		default:
			image.SetOpenURL(func() {
				openViewer(ctx, msgEmbed.Thumbnail.Proxy, opts)
			})
		}

		if big {
			image.SetHAlign(gtk.AlignStart)
			bodyBox.Append(image)
		} else {
			image.SetHAlign(gtk.AlignEnd)
			embedBox.Append(image)
		}
	}

	if msgEmbed.Thumbnail == nil && (msgEmbed.Image != nil || msgEmbed.Video != nil) {
		opts := defaultEmbedOpts
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

		image := embed.New(ctx, maxEmbedWidth.Value(), maxImageHeight.Value(), opts)
		image.SetSizeRequest(int(img.Width), int(img.Height))

		image.SetOpenURL(func() {
			openViewer(ctx, msgEmbed.URL, opts)
		})

		if msgEmbed.Image != nil {
			// The server can only resize images.
			image.SetFromURL(resizeURL(
				img.URL,
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

func openViewer(ctx context.Context, uri string, opts embed.Opts) {
	embedViewer, err := embed.NewViewer(ctx, uri, opts)
	if err != nil {
		app.Error(ctx, err)
		return
	}

	embedViewer.Show()
}

func fixNatWrap(label *gtk.Label) {
	if err := gtk.CheckVersion(4, 6, 0); err == "" {
		label.SetObjectProperty("natural-wrap-mode", 1) // NaturalWrapNone
	}
}
