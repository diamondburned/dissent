package messages

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/embed"
	"github.com/diamondburned/chatkit/md/mdrender"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/ningen/v3/discordmd"
	"github.com/dustin/go-humanize"
	"libdb.so/dissent/internal/gtkcord"
	"libdb.so/dissent/internal/mediaviewer"
)

// TODO: allow disable fetching videos.

var trustedCDNHosts = []string{
	"cdn.discordapp.com",
}

// embedFieldColumns is the number of inline embed fields in a row.
const embedFieldColumns = 3

// maxRichEmbedWidth is the maximum width of an embed with a body, which is the
// same as the official client's.
const maxRichEmbedWidth = 520

var defaultEmbedOpts = embed.Opts{
	Provider:    imgutil.HTTPProvider,
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
		if slices.Contains(trustedCDNHosts, direct.Host) {
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
	.message-embed-spoiler .onlineimage {
		filter: blur(45px);
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
	return newAttachmentSized(ctx, attachment, maxEmbedWidth.Value(), maxImageHeight.Value())
}

// newAttachmentSized is like newAttachment, but images and videos are at most
// maxW by maxH.
func newAttachmentSized(ctx context.Context, attachment *discord.Attachment, maxW, maxH int) gtk.Widgetter {
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

		src := attachment.Proxy
		if mimeType == "image" && attachment.Width > 0 && attachment.Height > 0 {
			// Fetch the image at the size that it is shown at, since large
			// images tank performance on Cairo renderers.
			w, h := imgutil.MaxSize(int(attachment.Width), int(attachment.Height), maxW, maxH)
			scale := gtkutil.ScaleFactor()
			src = resizeURL(attachment.URL, attachment.Proxy, w*scale, h*scale)
		}

		image, frame := newEmbedImage(ctx, opts, src,
			int(attachment.Width), int(attachment.Height), maxW, maxH)
		image.SetName(name)
		image.SetOpenURL(func() {
			openViewer(ctx, attachment.URL, opts)
		})

		if strings.HasPrefix(attachment.Filename, "SPOILER_") {
			image.AddCSSClass("message-embed-spoiler")
		}

		frame.AddCSSClass("message-richframe")
		return frame
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
	@define-color dissent_embed_background alpha(@theme_fg_color, 0.05);

	.message-normalembed {
		border: none;
		border-radius: 8px;
		padding: 10px;
		background-color: @dissent_embed_background;
	}
	.message-normalembed-body > *:not(:last-child) {
		margin-bottom: 0.5em;
	}
	.message-embed-author,
	.message-embed-description {
		font-size: 0.9em;
	}
	.message-embed-author-icon,
	.message-embed-footer-icon {
		margin-right: 0.5em;
	}
	.message-embed-footer {
		opacity: 0.5;
		font-size: 0.8em;
	}
`)

const embedColorCSSf = `
	.message-normalembed {
		padding-left: 14px;
		background: linear-gradient(to right,
			%s 4px,
			@dissent_embed_background 0px,
			@dissent_embed_background 100%%
		);
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
			img := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, 18)
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
		// Boxes are used instead of a grid, which asks for the height of its
		// wrapped labels at the grid's minimum width.
		fields := gtk.NewBox(gtk.OrientationVertical, 7)
		fields.AddCSSClass("message-embed-fields")

		bodyBox.Append(fields)
		hasBody = true

		var row *gtk.Box
		var col int

		for _, field := range msgEmbed.Fields {
			text := gtk.NewLabel("")
			text.SetWrap(true)
			text.SetWrapMode(pango.WrapWordChar)
			text.SetXAlign(0.0)
			text.SetVAlign(gtk.AlignStart)
			text.SetMarkup(fmt.Sprintf(
				`<span weight="heavy">%s</span>`+"\n"+`<span weight="light">%s</span>`,
				html.EscapeString(field.Name),
				html.EscapeString(field.Value),
			))
			text.SetTooltipText(field.Name + "\n" + field.Value)

			// Inline fields share rows of up to 3, and other fields take up a
			// whole row.
			if !field.Inline {
				fields.Append(text)
				row = nil
				continue
			}
			if row == nil || col == embedFieldColumns {
				row = gtk.NewBox(gtk.OrientationHorizontal, 14)
				row.SetHomogeneous(true)
				fields.Append(row)
				col = 0
			}
			row.Append(text)
			col++
		}
	}

	if msgEmbed.Footer != nil || msgEmbed.Timestamp.IsValid() {
		footer := gtk.NewBox(gtk.OrientationHorizontal, 0)
		footer.AddCSSClass("message-embed-footer")

		if msgEmbed.Footer != nil {
			if msgEmbed.Footer.ProxyIcon != "" {
				img := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, 18)
				img.AddCSSClass("message-embed-footer-icon")

				footer.Append(img)
			}

			if msgEmbed.Footer.Text != "" {
				text := gtk.NewLabel(msgEmbed.Footer.Text)
				text.SetVAlign(gtk.AlignStart)
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

	embedBox := gtk.Widgetter(bodyBox)
	var embedOverlay *gtk.Overlay
	if hasBody {
		// The thumbnail is laid over the body instead of next to it in a
		// horizontal box. Such a box measures the body's width for its height,
		// which makes wrapping labels as wide as their whole text.
		embedOverlay = gtk.NewOverlay()
		embedOverlay.SetHAlign(gtk.AlignStart)
		embedOverlay.SetVAlign(gtk.AlignStart)
		embedOverlay.SetChild(bodyBox)
		normalEmbedCSS(embedOverlay)

		if msgEmbed.Color != discord.NullColor {
			cssutil.Applyf(embedOverlay, embedColorCSSf, msgEmbed.Color.String())
		}

		// Keep long text from making the embed as wide as the message list,
		// like the official client does.
		clamp := adw.NewClamp()
		clamp.SetUnit(adw.LengthUnitPx)
		clamp.SetMaximumSize(maxRichEmbedWidth)
		clamp.SetTighteningThreshold(maxRichEmbedWidth)
		clamp.SetHAlign(gtk.AlignStart)
		clamp.SetVAlign(gtk.AlignStart)
		clamp.SetChild(embedOverlay)
		embedBox = clamp
	}

	// Big thumbnails are shown below the body, and others are next to it.
	bigThumbnail := msgEmbed.Thumbnail != nil && (!hasBody ||
		msgEmbed.Type == discord.GIFVEmbed ||
		msgEmbed.Type == discord.ImageEmbed ||
		msgEmbed.Type == discord.VideoEmbed ||
		msgEmbed.Type == discord.ArticleEmbed)

	if msgEmbed.Thumbnail != nil {
		thumb := msgEmbed.Thumbnail
		big := bigThumbnail

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

		image, frame := newEmbedImage(ctx, opts,
			resizeURL(thumb.URL, thumb.Proxy, int(thumb.Width), int(thumb.Height)),
			int(thumb.Width), int(thumb.Height), maxW, maxH)

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
					return
				}

				switch opts.Type {
				case embed.EmbedTypeGIFV:
					// Play GIFVs in the embed. The image library will handle
					// rendering the GIFV like a GIF.
					image.SetFromURL(msgEmbed.Video.Proxy)
					image.ActivateDefault()
					// Override the next click to open the video in the viewer.
					image.SetOpenURL(func() {
						openViewer(ctx, msgEmbed.Video.Proxy, opts)
					})
				case embed.EmbedTypeVideo:
					// Play videos in the viewer.
					openViewer(ctx, msgEmbed.Video.Proxy, opts)
				}
			})
		default:
			image.SetOpenURL(func() {
				openViewer(ctx, msgEmbed.Thumbnail.Proxy, opts)
			})
		}

		if big {
			bodyBox.Append(frame)
		} else {
			frame.SetHAlign(gtk.AlignEnd)
			bodyBox.SetMarginEnd(maxW + 8)
			embedOverlay.AddOverlay(frame)
			embedOverlay.SetMeasureOverlay(frame, true)
		}
	}

	// Show the image unless the thumbnail already took its place, such as for
	// videos and links to images.
	if !bigThumbnail && (msgEmbed.Image != nil || (msgEmbed.Thumbnail == nil && msgEmbed.Video != nil)) {
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

		src := img.Proxy
		if msgEmbed.Image != nil {
			// The server can only resize images.
			src = resizeURL(img.URL, img.Proxy, int(img.Width), int(img.Height))
		}

		image, frame := newEmbedImage(ctx, opts, src,
			int(img.Width), int(img.Height), maxEmbedWidth.Value(), maxImageHeight.Value())
		image.SetOpenURL(func() {
			openViewer(ctx, img.URL, opts)
		})

		bodyBox.Append(frame)
	}

	gtk.BaseWidget(embedBox).AddCSSClass("message-richframe")
	return embedBox
}

// newEmbedImage creates an embed that shows src with the size of w by h, fitted
// into maxW by maxH. The returned frame holds the embed and is what should be
// added. It shrinks with the window while keeping its aspect ratio. The size is
// taken from the image once it loads if w or h is unknown.
func newEmbedImage(ctx context.Context, opts embed.Opts, src string, w, h, maxW, maxH int) (*embed.Embed, *aspectFrame) {
	image := embed.New(ctx, maxW, maxH, opts)
	image.SetFromURL(src)

	frame := newAspectFrame(image, 0, 0)
	if w > 0 && h > 0 {
		frame.SetSize(imgutil.MaxSize(w, h, maxW, maxH))
	} else {
		// The frame has no size to draw the image in until it is fetched.
		image.Image.FetchOnMap(frame)
	}

	image.SetHAlign(gtk.AlignFill)
	image.SetVAlign(gtk.AlignFill)
	image.Bin.SetSizeRequest(-1, -1)
	image.NotifyImage(func() {
		// The frame sizes the image instead.
		image.Bin.SetSizeRequest(-1, -1)
		if iw, ih := image.ImageSize(); iw > 0 && ih > 0 && (w <= 0 || h <= 0) {
			frame.SetSize(imgutil.MaxSize(iw, ih, maxW, maxH))
		}
	})

	return image, frame
}

func openViewer(ctx context.Context, uri string, opts embed.Opts) {
	mediaviewer.Open(ctx, uri, opts)
}

func fixNatWrap(label *gtk.Label) {
	if err := gtk.CheckVersion(4, 6, 0); err == "" {
		label.SetObjectProperty("natural-wrap-mode", 1) // NaturalWrapNone
	}
}
