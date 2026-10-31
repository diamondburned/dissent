package messages

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/embed"
	"github.com/diamondburned/chatkit/md/mdrender"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/ningen/v3/discordmd"
	"github.com/yuin/goldmark/ast"
	"libdb.so/ctxt"
	"libdb.so/dissent/internal/gtkcord"
)

var (
	//go:embed component_button.ui
	componentButtonUI string
	//go:embed component_section.ui
	componentSectionUI string
	//go:embed component_container.ui
	componentContainerUI string
	//go:embed component_gallery.ui
	componentGalleryUI string
	//go:embed component_gallery_item.ui
	componentGalleryItemUI string
	//go:embed component.css
	componentCSS string
)

var _ = cssutil.WriteCSS(componentCSS)

// componentAccentCSSf draws a container's accent color as its left border.
const componentAccentCSSf = `
	.message-component-container {
		border-left: 4px solid %s;
	}
`

// thumbnailSize is the maximum size of a section's thumbnail.
const thumbnailSize = 96

// messageComponents renders the components of a message.
type messageComponents struct {
	ctx  context.Context
	view *View
	msg  *discord.Message
}

func newMessageComponents(ctx context.Context, v *View, m *discord.Message) gtk.Widgetter {
	c := messageComponents{ctx, v, m}

	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("message-components")
	for _, component := range m.Components {
		box.Append(c.render(component))
	}
	return box
}

func (c messageComponents) render(component discord.Component) gtk.Widgetter {
	switch component := component.(type) {
	case *discord.ActionRowComponent:
		return c.actionRow(*component)
	case *discord.ButtonComponent:
		return c.button(component)
	case *discord.StringSelectComponent,
		*discord.UserSelectComponent,
		*discord.RoleSelectComponent,
		*discord.MentionableSelectComponent,
		*discord.ChannelSelectComponent:
		return c.selectMenu(component.(discord.InteractiveComponent))
	case *discord.SectionComponent:
		return c.section(component)
	case *discord.TextDisplayComponent:
		return newComponentText(c.ctx, c.msg, component.Content)
	case *discord.ThumbnailComponent:
		return newComponentMedia(c.ctx, component.Media, component.Spoiler, component.Description, thumbnailSize)
	case *discord.MediaGalleryComponent:
		return c.mediaGallery(component)
	case *discord.FileComponent:
		return newComponentFile(c.ctx, component)
	case *discord.SeparatorComponent:
		return newComponentSeparator(component)
	case *discord.ContainerComponent:
		return c.container(component)
	default:
		return newUnsupportedComponent(component)
	}
}

// send sends an interaction for one of the message's components.
func (c messageComponents) send(data api.ComponentInteractionData, done func(error)) {
	target := messageInteractionTarget(c.msg)
	c.view.sendInteraction(target, discord.ComponentInteractionType, data, func(err error) {
		if err != nil {
			c.view.interactionToast(err)
		}
		done(err)
	})
}

func (c messageComponents) actionRow(row discord.ActionRowComponent) gtk.Widgetter {
	flow := gtk.NewFlowBox()
	flow.AddCSSClass("message-component-row")
	flow.SetSelectionMode(gtk.SelectionNone)
	flow.SetHAlign(gtk.AlignStart)
	flow.SetColumnSpacing(6)
	flow.SetRowSpacing(6)
	flow.SetMaxChildrenPerLine(5)
	for i, component := range row {
		flow.Append(c.render(component))
		flow.ChildAtIndex(i).SetFocusable(false)
	}
	return flow
}

func (c messageComponents) button(b *discord.ButtonComponent) gtk.Widgetter {
	ui := gtk.NewBuilderFromString(componentButtonUI)
	button := ui.GetObject("rootContent").Cast().(*gtk.Button)
	stack := ui.GetObject("stack").Cast().(*gtk.Stack)
	emoji := ui.GetObject("emoji").Cast().(*gtk.Box)
	label := ui.GetObject("label").Cast().(*gtk.Label)
	link := ui.GetObject("link").Cast().(*gtk.Image)

	label.SetText(b.Label)
	label.SetVisible(b.Label != "")
	setComponentEmoji(c.ctx, emoji, b.Emoji)
	button.SetSensitive(!b.Disabled)

	switch b.Style {
	case discord.PrimaryButtonStyle():
		button.AddCSSClass("suggested-action")
	case discord.SuccessButtonStyle():
		button.AddCSSClass("success")
	case discord.DangerButtonStyle():
		button.AddCSSClass("destructive-action")
	}

	if url := b.LinkURL(); url != "" {
		link.SetVisible(true)
		button.SetTooltipText(url)
		button.ConnectClicked(func() { app.OpenURI(c.ctx, url) })
		return button
	}

	button.ConnectClicked(func() {
		stack.SetVisibleChildName("busy")
		button.SetSensitive(false)

		c.send(api.ComponentInteractionData{
			ComponentType: discord.ButtonComponentType,
			CustomID:      b.CustomID,
		}, func(error) {
			stack.SetVisibleChildName("content")
			button.SetSensitive(true)
		})
	})

	return button
}

func (c messageComponents) selectMenu(component discord.InteractiveComponent) gtk.Widgetter {
	s := newComponentSelect(c.ctx, c.view.guildID, component)
	s.onChange = func(values []string, done func(error)) {
		c.send(api.ComponentInteractionData{
			ComponentType: component.Type(),
			CustomID:      component.ID(),
			Values:        values,
		}, done)
	}
	return s
}

func (c messageComponents) section(section *discord.SectionComponent) gtk.Widgetter {
	ui := gtk.NewBuilderFromString(componentSectionUI)
	box := ui.GetObject("rootContent").Cast().(*gtk.Box)
	texts := ui.GetObject("texts").Cast().(*gtk.Box)
	accessory := ui.GetObject("accessory").Cast().(*gtk.Box)

	for _, component := range section.Components {
		texts.Append(c.render(component))
	}

	if section.Accessory != nil {
		accessory.Append(c.render(section.Accessory))
	} else {
		accessory.SetVisible(false)
	}

	return box
}

func (c messageComponents) mediaGallery(gallery *discord.MediaGalleryComponent) gtk.Widgetter {
	ui := gtk.NewBuilderFromString(componentGalleryUI)
	grid := ui.GetObject("rootContent").Cast().(*gtk.Grid)

	// cells are the positions of the items as {column, row, width, height}
	// in a grid of columns by rows.
	var cells [][4]int
	var columns, rows int
	width := maxEmbedWidth.Value()
	height := width * 2 / 3

	switch len(gallery.Items) {
	case 1:
		cells = [][4]int{{0, 0, 1, 1}}
		columns, rows = 1, 1
		if media := gallery.Items[0].Media; media.Width > 0 && media.Height > 0 {
			width, height = imgutil.MaxSize(media.Width, media.Height, width, maxImageHeight.Value())
		}
	case 2:
		cells = [][4]int{{0, 0, 1, 1}, {1, 0, 1, 1}}
		columns, rows = 2, 1
		height = width / 2
	case 3:
		// One large item on the left and two stacked on the right.
		cells = [][4]int{{0, 0, 2, 2}, {2, 0, 1, 1}, {2, 1, 1, 1}}
		columns, rows = 3, 2
	default:
		cells = [][4]int{{0, 0, 1, 1}, {1, 0, 1, 1}, {0, 1, 1, 1}, {1, 1, 1, 1}}
		columns, rows = 2, 2
	}

	const spacing = 4
	unitW := (width - (columns-1)*spacing) / columns
	unitH := (height - (rows-1)*spacing) / rows

	for i, cell := range cells {
		w := cell[2]*unitW + (cell[2]-1)*spacing
		h := cell[3]*unitH + (cell[3]-1)*spacing
		item := newGalleryItem(c.ctx, gallery.Items[i], w, h)

		// Cover the last cell with the number of items that are not shown.
		if i == len(cells)-1 && len(gallery.Items) > len(cells) {
			more := item.more
			more.SetText(fmt.Sprintf("%d+", len(gallery.Items)-len(cells)))
			more.SetVisible(true)
		}

		// Only round the corners on the outside of the collage.
		if cell[1] == 0 {
			item.AddCSSClass("top")
		}
		if cell[1]+cell[3] == rows {
			item.AddCSSClass("bottom")
		}
		if cell[0] == 0 {
			item.AddCSSClass("left")
		}
		if cell[0]+cell[2] == columns {
			item.AddCSSClass("right")
		}

		grid.Attach(item, cell[0], cell[1], cell[2], cell[3])
	}

	// Scale the collage down with the window instead of requesting its full
	// size, so that it never stops the window from getting narrower.
	return newAspectFrame(grid, width, height)
}

type galleryItem struct {
	*gtk.Button
	more *gtk.Label
}

// newGalleryItem creates an image of a media gallery that is cropped to fit.
// The image is fetched at w by h.
func newGalleryItem(ctx context.Context, item discord.MediaGalleryComponentItem, w, h int) galleryItem {
	ui := gtk.NewBuilderFromString(componentGalleryItemUI)
	button := ui.GetObject("rootContent").Cast().(*gtk.Button)
	overlay := ui.GetObject("overlay").Cast().(*gtk.Overlay)
	more := ui.GetObject("more").Cast().(*gtk.Label)

	opts := defaultEmbedOpts
	opts.Type = embed.EmbedTypeImage
	provider := imgutil.HTTPProvider
	if strings.HasPrefix(item.Media.ContentType, "video/") {
		opts.Type = embed.EmbedTypeVideo
		provider = imgutil.FFmpegProvider
	}

	paintable := onlineimage.NewPaintable(ctx, provider)

	picture := gtk.NewPictureForPaintable(paintable)
	picture.SetContentFit(gtk.ContentFitCover)
	picture.SetCanShrink(true)
	overlay.SetChild(picture)

	proxy := item.Media.ProxyURL
	if proxy == "" {
		proxy = item.Media.URL
	}
	if opts.Type == embed.EmbedTypeImage {
		size := max(w, h) * gtkutil.ScaleFactor()
		paintable.SetURL(resizeURL(item.Media.URL, proxy, size, size))
	} else {
		paintable.SetURL(proxy)
	}

	if item.Description != "" {
		button.SetTooltipText(item.Description)
		ui.GetObject("alt").Cast().(*gtk.Label).SetVisible(true)
	}
	if item.Spoiler {
		button.AddCSSClass("message-embed-spoiler")
	}

	button.ConnectClicked(func() {
		// Reveal a spoiler before opening it.
		if button.HasCSSClass("message-embed-spoiler") {
			button.RemoveCSSClass("message-embed-spoiler")
			return
		}
		openViewer(ctx, item.Media.URL, opts)
	})

	return galleryItem{button, more}
}

func (c messageComponents) container(container *discord.ContainerComponent) gtk.Widgetter {
	ui := gtk.NewBuilderFromString(componentContainerUI)
	stack := ui.GetObject("rootContent").Cast().(*gtk.Stack)
	content := ui.GetObject("content").Cast().(*gtk.Box)
	spoiler := ui.GetObject("spoiler").Cast().(*gtk.Button)

	for _, component := range container.Components {
		content.Append(c.render(component))
	}

	if container.AccentColor != discord.NullColor && container.AccentColor != 0 {
		cssutil.Applyf(content, componentAccentCSSf, container.AccentColor.String())
	}

	if container.Spoiler {
		stack.SetVisibleChildName("spoiler")
		spoiler.ConnectClicked(func() { stack.SetVisibleChildName("content") })
	} else {
		stack.SetVisibleChildName("content")
	}

	return stack
}

// newComponentText renders the Markdown of a text display. The message is
// used to resolve mentions and may be nil.
func newComponentText(ctx context.Context, m *discord.Message, text string) gtk.Widgetter {
	src := []byte(text)

	var node ast.Node
	if m != nil {
		state := gtkcord.FromContext(ctx)
		node = discordmd.ParseWithMessage(src, *state.Cabinet, m, true)
	} else {
		node = discordmd.Parse(src)
	}

	return mdrender.NewMarkdownViewer(ctxt.With(ctx, newMarkdownState()), src, node, renderers...)
}

// newComponentMedia renders an image or video of a thumbnail or media
// gallery, at most size by size.
func newComponentMedia(
	ctx context.Context, media discord.UnfurledMediaitem,
	spoiler bool, description string, size int) gtk.Widgetter {

	attachment := mediaAttachment(media, "", spoiler)
	attachment.Description = description
	if attachment.ContentType == "" {
		// Thumbnails and galleries only allow images and videos.
		attachment.ContentType = "image/*"
	}

	return newAttachmentSized(ctx, &attachment, size, size)
}

func newComponentFile(ctx context.Context, file *discord.FileComponent) gtk.Widgetter {
	attachment := mediaAttachment(file.File, file.Name, file.Spoiler)
	attachment.Size = uint64(file.Size)
	if contentType, _, _ := strings.Cut(attachment.ContentType, "/"); contentType == "image" || contentType == "video" {
		// Files are never shown inline.
		attachment.ContentType = ""
	}
	return newAttachment(ctx, &attachment)
}

// mediaAttachment returns an attachment for media so that it can be rendered
// with newAttachment. If name is empty, the name is taken from the URL.
func mediaAttachment(media discord.UnfurledMediaitem, name string, spoiler bool) discord.Attachment {
	if name == "" {
		if u, err := url.Parse(media.URL); err == nil {
			name = path.Base(u.Path)
		}
	}
	if spoiler {
		name = "SPOILER_" + name
	}

	proxy := media.ProxyURL
	if proxy == "" {
		proxy = media.URL
	}

	return discord.Attachment{
		ID:          media.AttachmentID,
		Filename:    name,
		ContentType: media.ContentType,
		URL:         media.URL,
		Proxy:       proxy,
		Width:       uint(media.Width),
		Height:      uint(media.Height),
	}
}

func newComponentSeparator(separator *discord.SeparatorComponent) gtk.Widgetter {
	margin := 4
	if separator.Spacing == discord.SeparatorComponentSpacingLargePadding {
		margin = 12
	}

	sep := gtk.NewSeparator(gtk.OrientationHorizontal)
	sep.AddCSSClass("message-component-separator")
	if separator.Divider {
		sep.AddCSSClass("divider")
	}
	sep.SetMarginTop(margin)
	sep.SetMarginBottom(margin)
	return sep
}

func newUnsupportedComponent(component discord.Component) gtk.Widgetter {
	label := gtk.NewLabel(locale.Sprintf("Unsupported component: %s", component.Type()))
	label.AddCSSClass("dim-label")
	label.SetXAlign(0)
	label.SetWrap(true)
	return label
}

// setComponentEmoji shows the emoji in box, or hides box if there is none.
func setComponentEmoji(ctx context.Context, box *gtk.Box, e *discord.ComponentEmoji) {
	if e == nil {
		box.SetVisible(false)
		return
	}

	if e.ID.IsValid() {
		emoji := discord.Emoji{ID: e.ID, Name: e.Name, Animated: e.Animated}
		paintable := onlineimage.NewPaintable(ctx, imgutil.HTTPProvider)
		paintable.SetURL(emoji.EmojiURL())

		image := gtk.NewImageFromPaintable(paintable)
		image.SetPixelSize(gtkcord.InlineEmojiSize)
		box.Append(image)
	} else {
		box.Append(gtk.NewLabel(e.Name))
	}
}
