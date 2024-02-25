package messages

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/chatkit/components/author"
	"github.com/diamondburned/chatkit/md"
	"github.com/diamondburned/chatkit/md/mdrender"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/ningen/v3/discordmd"
	"github.com/yuin/goldmark/ast"
	"libdb.so/dissent/internal/gtkcord"
)

// Content is the message content widget.
type Content struct {
	*gtk.Box
	ctx    context.Context
	view   *View
	menu   *gio.Menu
	mdview *mdrender.MarkdownViewer
	react  *contentReactions
	child  []gtk.Widgetter

	chID  discord.ChannelID
	msgID discord.MessageID
}

var contentCSS = cssutil.Applier("message-content-box", `
	.message-content-box {
		margin-right: 4px;
	}
	.message-content-box > *:not(:first-child) {
		margin-top: 4px;
	}
	.message-content-box .thumbnail-embed {
		border-width: 0;
		border-radius: 8px; /* stolen from Discord mobile */
	}
	.message-header-blockquote {
		margin-bottom: 0;
	}
	.message-header-blockquote > *,
	.message-header-blockquote .mauthor-chip,
	.message-reply-content link {
		color: mix(@theme_bg_color, @theme_fg_color, 0.85);
	}
	.message-header-blockquote > * {
		font-size: 0.9em;
	}
	.message-interaction-name {
		margin-left: 0.25em;
		font-family: monospace;
	}
`)

// NewContent creates a new Content widget.
func NewContent(ctx context.Context, v *View) *Content {
	c := Content{
		ctx:   ctx,
		view:  v,
		child: make([]gtk.Widgetter, 0, 2),
		chID:  v.ChannelID(),
	}
	c.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	contentCSS(c.Box)

	return &c
}

// MessageID returns the message ID.
func (c *Content) MessageID() discord.MessageID {
	return c.msgID
}

// ChannelID returns the channel ID.
func (c *Content) ChannelID() discord.ChannelID {
	return c.chID
}

// SetExtraMenu implements ExtraMenuSetter.
func (c *Content) SetExtraMenu(menu gio.MenuModeller) {
	c.menu = gio.NewMenu()
	c.menu.InsertSection(0, locale.Get("Message"), menu)

	if c.mdview != nil {
		c.setMenu()
	}
}

type extraMenuSetter interface{ SetExtraMenu(gio.MenuModeller) }

var (
	_ extraMenuSetter = (*gtk.TextView)(nil)
	_ extraMenuSetter = (*gtk.Label)(nil)
)

func (c *Content) setMenu() {
	var menu gio.MenuModeller
	if c.menu != nil {
		menu = c.menu // because a nil interface{} != nil *T
	}

	for _, child := range c.child {
		// Manually check on child to allow certain widgets to override the
		// method.
		s, ok := child.(extraMenuSetter)
		if ok {
			s.SetExtraMenu(menu)
		}

		gtkutil.WalkWidget(c.Box, func(w gtk.Widgetter) bool {
			s, ok := w.(extraMenuSetter)
			if ok {
				s.SetExtraMenu(menu)
			}
			return false
		})
	}
}

var systemContentCSS = cssutil.Applier("message-system-content", `
	.message-system-content {
		font-style: italic;
		color: alpha(@theme_fg_color, 0.9);
	}
`)

// Update replaces Content with the message.
func (c *Content) Update(m *discord.Message, customs ...gtk.Widgetter) {
	c.msgID = m.ID
	c.clear()

	state := gtkcord.FromContext(c.ctx)

	if m.Reference != nil {
		w := c.newReplyBox(m)
		c.append(w)
	}

	if m.Interaction != nil {
		w := c.newInteractionBox(m)
		c.append(w)
	}

	var messageMarkup string
	switch m.Type {
	case discord.GuildMemberJoinMessage:
		messageMarkup = locale.Get("Joined the server.")
	case discord.CallMessage:
		messageMarkup = locale.Get("Calling you.")
	case discord.ChannelIconChangeMessage:
		messageMarkup = locale.Get("Changed the channel icon.")
	case discord.ChannelNameChangeMessage:
		messageMarkup = locale.Get("Changed the channel name to #%s.", html.EscapeString(m.Content))
	case discord.ChannelPinnedMessage:
		messageMarkup = locale.Get(`Pinned <a href="#message/%d">a message</a>.`, m.ID)
	case discord.RecipientAddMessage, discord.RecipientRemoveMessage:
		mentioned := state.MemberMarkup(m.GuildID, &m.Mentions[0], author.WithMinimal())
		switch m.Type {
		case discord.RecipientAddMessage:
			messageMarkup = locale.Get("Added %s to the group.", mentioned)
		case discord.RecipientRemoveMessage:
			messageMarkup = locale.Get("Removed %s from the group.", mentioned)
		}
	case discord.NitroBoostMessage:
		messageMarkup = locale.Get("Boosted the server!")
	case discord.NitroTier1Message:
		messageMarkup = locale.Get("The server is now Nitro Boosted to Tier 1.")
	case discord.NitroTier2Message:
		messageMarkup = locale.Get("The server is now Nitro Boosted to Tier 2.")
	case discord.NitroTier3Message:
		messageMarkup = locale.Get("The server is now Nitro Boosted to Tier 3.")
	}

	c.mdview = nil

	switch {
	case messageMarkup != "":
		msg := gtk.NewLabel("")
		msg.SetMarkup(messageMarkup)
		msg.SetHExpand(true)
		msg.SetXAlign(0)
		msg.SetWrap(true)
		msg.SetWrapMode(pango.WrapWordChar)
		msg.ConnectActivateLink(func(uri string) bool {
			if !strings.HasPrefix(uri, "#") {
				return false // not our link
			}

			parts := strings.SplitN(uri, "/", 2)
			if len(parts) != 2 {
				return true // pretend we've handled this because of #
			}

			switch strings.TrimPrefix(parts[0], "#") {
			case "message":
				if id, _ := discord.ParseSnowflake(parts[1]); id.IsValid() {
					c.view.ScrollToMessage(discord.MessageID(id))
				}
			}

			return true
		})
		systemContentCSS(msg)
		fixNatWrap(msg)
		c.append(msg)

	// We render a big content if the content itself is literally a Unicode
	// emoji.
	case m.Content != "" && md.IsUnicodeEmoji(m.Content):
		l := gtk.NewLabel(m.Content)
		l.SetAttributes(gtkcord.EmojiAttrs)
		l.SetHExpand(true)
		l.SetXAlign(0)
		l.SetSelectable(true)
		l.SetWrap(true)
		l.SetWrapMode(pango.WrapWordChar)
		c.append(l)

	// We don't render the message content if all it is is the URL to the
	// embedded image, because that's what the official client does.
	case m.Content != "" &&
		(len(m.Embeds) != 1 || m.Embeds[0].Type != discord.ImageEmbed || m.Embeds[0].URL != m.Content):

		src := []byte(m.Content)
		node := discordmd.ParseWithMessage(src, *state.Cabinet, m, true)

		c.mdview = mdrender.NewMarkdownViewer(c.ctx, src, node, renderers...)
		c.append(c.mdview)
	}

	for i := range m.Stickers {
		v := newSticker(c.ctx, &m.Stickers[i])
		c.append(v)
	}

	for i := range m.Attachments {
		v := newAttachment(c.ctx, &m.Attachments[i])
		c.append(v)
	}

	for i := range m.Embeds {
		v := newEmbed(c.ctx, m, &m.Embeds[i])
		c.append(v)
	}

	for _, custom := range customs {
		c.append(custom)
	}

	c.SetReactions(m.Reactions)
	c.setMenu()
}

func (c *Content) newReplyBox(m *discord.Message) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("md-blockquote")
	box.AddCSSClass("message-header-blockquote")
	box.AddCSSClass("message-reply-box")

	state := gtkcord.FromContext(c.ctx)

	referencedMsg := m.ReferencedMessage
	if referencedMsg == nil {
		referencedMsg, _ = state.Cabinet.Message(m.Reference.ChannelID, m.Reference.MessageID)
	}

	if referencedMsg == nil {
		slog.Warn(
			"Cannot display message reference because the message is not found",
			"channel_id", m.ChannelID,
			"guild_id", m.GuildID,
			"id", m.ID,
			"id_reference", m.Reference.MessageID)

		header := gtk.NewLabel("Unknown message.")
		header.AddCSSClass("message-reply-header")
		box.Append(header)

		return box
	}

	if !showBlockedMessages.Value() && state.UserIsBlocked(referencedMsg.Author.ID) {
		header := gtk.NewLabel("Blocked user.")
		header.AddCSSClass("message-reply-header")
		box.Append(header)

		blockedCSS(box)
		return box
	}

	member, _ := state.Cabinet.Member(m.Reference.GuildID, referencedMsg.Author.ID)
	chip := newAuthorChip(c.ctx, m.GuildID, &discord.GuildUser{
		User:   referencedMsg.Author,
		Member: member,
	})
	chip.SetHAlign(gtk.AlignStart)
	chip.Unpad()
	box.Append(chip)

	if preview := state.MessagePreview(referencedMsg); preview != "" {
		// Force single line.
		preview = strings.ReplaceAll(preview, "\n", "  ")
		markup := fmt.Sprintf(
			`<a href="dissent://reply">%s</a>`,
			html.EscapeString(preview),
		)

		reply := gtk.NewLabel(markup)
		reply.AddCSSClass("message-reply-content")
		reply.SetUseMarkup(true)
		reply.SetTooltipText(preview)
		reply.SetEllipsize(pango.EllipsizeEnd)
		reply.SetLines(1)
		reply.SetXAlign(0)
		reply.ConnectActivateLink(func(link string) bool {
			slog.Debug(
				"Activated message reference link",
				"link", link,
				"message_id", m.ID,
				"reference_id", referencedMsg.ID)

			if link != "dissent://reply" {
				return false
			}

			if !c.ActivateAction("messages.scroll-to", gtkcord.NewMessageIDVariant(m.ID)) {
				slog.Error(
					"Failed to activate messages.scroll-to",
					"id", m.ID)
			}

			return true
		})

		box.Append(reply)
	}

	if state.UserIsBlocked(referencedMsg.Author.ID) {
		blockedCSS(box)
	}

	return box
}

func (c *Content) newInteractionBox(m *discord.Message) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("md-blockquote")
	box.AddCSSClass("message-header-blockquote")
	box.AddCSSClass("message-interaction-box")

	state := gtkcord.FromContext(c.ctx)

	if !showBlockedMessages.Value() && state.UserIsBlocked(m.Interaction.User.ID) {
		header := gtk.NewLabel("Blocked user.")
		header.AddCSSClass("message-reply-header")
		box.Append(header)

		blockedCSS(box)
		return box
	}

	chip := newAuthorChip(c.ctx, m.GuildID, &discord.GuildUser{
		User:   m.Interaction.User,
		Member: m.Interaction.Member,
	})
	chip.SetHAlign(gtk.AlignStart)
	chip.Unpad()
	box.Append(chip)

	nameLabel := gtk.NewLabel(m.Interaction.Name)
	nameLabel.AddCSSClass("message-interaction-name")
	if m.Interaction.Type == discord.CommandInteractionType {
		nameLabel.SetText("/" + m.Interaction.Name)
		nameLabel.AddCSSClass("message-interaction-command")
	}
	nameLabel.SetTooltipText(m.Interaction.Name)
	nameLabel.SetEllipsize(pango.EllipsizeEnd)
	nameLabel.SetXAlign(0)
	box.Append(nameLabel)

	if state.UserIsBlocked(m.Interaction.User.ID) {
		blockedCSS(box)
	}

	return box
}

func (c *Content) append(w gtk.Widgetter) {
	c.Box.Append(w)
	c.child = append(c.child, w)
}

func (c *Content) SetCustomChild(child ...gtk.Widgetter) {
	c.clear()
	for _, w := range child {
		c.append(w)
	}
}

func (c *Content) clear() {
	for i, child := range c.child {
		c.Box.Remove(child)
		c.child[i] = nil
	}
	c.child = c.child[:0]
}

var redactedContentCSS = cssutil.Applier("message-redacted-content", `
	.message-redacted-content {
		font-style: italic;
		color: alpha(@theme_fg_color, 0.75);
	}
`)

// Redact clears the content widget.
func (c *Content) Redact() {
	c.clear()

	red := gtk.NewLabel(locale.Get("Redacted."))
	red.SetXAlign(0)
	redactedContentCSS(red)
	c.append(red)
}

// SetReactions sets the reactions inside the message.
func (c *Content) SetReactions(reactions []discord.Reaction) {
	if c.react == nil {
		if len(reactions) == 0 {
			return
		}
		c.react = newContentReactions(c.ctx, c)
		c.append(c.react)
	}
	c.react.SetReactions(reactions)
}

var renderers = []mdrender.OptionFunc{
	mdrender.WithRenderer(discordmd.KindEmoji, renderEmoji),
	mdrender.WithRenderer(discordmd.KindInline, renderInline),
	mdrender.WithRenderer(discordmd.KindMention, renderMention),
}

var inlineEmojiTag = textutil.TextTag{
	"rise":     -5 * pango.SCALE,
	"rise-set": true,
}

func renderEmoji(r *mdrender.Renderer, n ast.Node) ast.WalkStatus {
	emoji := n.(*discordmd.Emoji)
	text := r.State.TextBlock()

	picture := onlineimage.NewPicture(r.State.Context(), imgutil.HTTPProvider)
	picture.EnableAnimation().OnHover()
	picture.SetKeepAspectRatio(true)
	picture.SetTooltipText(emoji.Name)
	picture.SetURL(gtkcord.EmojiURL(emoji.ID, emoji.GIF))

	var inlineImage *md.InlineImage
	makeInlineImage := func(size int) {
		inlineImage = md.InsertCustomImageWidget(text.TextView, text.Buffer.CreateChildAnchor(text.Iter), picture)
		inlineImage.SetSizeRequest(size, size)
	}

	if emoji.Large {
		makeInlineImage(gtkcord.LargeEmojiSize)
	} else {
		tag := inlineEmojiTag.FromTable(text.Buffer.TagTable(), "inline-emoji")
		text.TagBounded(tag, func() { makeInlineImage(gtkcord.InlineEmojiSize) })
	}

	return ast.WalkContinue
}

var htmlTagMap = map[discordmd.Attribute]string{
	discordmd.AttrBold:          "b",
	discordmd.AttrItalics:       "i",
	discordmd.AttrUnderline:     "u",
	discordmd.AttrStrikethrough: "strike",
	discordmd.AttrMonospace:     "code",
}

func renderInline(r *mdrender.Renderer, n ast.Node) ast.WalkStatus {
	text := r.State.TextBlock()
	startIx := text.Iter.Offset()

	// Render everything inside. We'll wrap the whole region with tags.
	r.RenderChildren(n)

	start := text.Buffer.IterAtOffset(startIx)
	end := text.Iter

	inline := n.(*discordmd.Inline)

	for tag, htmltag := range htmlTagMap {
		if inline.Attr.Has(tag) {
			text.Buffer.ApplyTag(text.Tag(htmltag), start, end)
		}
	}

	return ast.WalkSkipChildren
}

// rgba(111, 120, 219, 0.3)
const defaultMentionColor = "#6F78DB"

func mentionTag(r *mdrender.Renderer, color string) *gtk.TextTag {
	tag := textutil.TextTag{"background": color + "76"}
	return tag.FromTable(r.State.TagTable(), tag.Hash())
}

func renderMention(r *mdrender.Renderer, n ast.Node) ast.WalkStatus {
	mention := n.(*discordmd.Mention)

	text := r.State.TextBlock()

	switch {
	case mention.Channel != nil:
		text.TagBounded(mentionTag(r, defaultMentionColor), func() {
			text.Insert(" #" + mention.Channel.Name + " ")
		})

	case mention.GuildRole != nil:
		roleColor := defaultMentionColor
		if mention.GuildRole.Color != discord.NullColor {
			roleColor = mention.GuildRole.Color.String()
		}

		text.TagBounded(mentionTag(r, roleColor), func() {
			text.Insert(" @" + mention.GuildRole.Name + " ")
		})

	case mention.GuildUser != nil:
		chip := newAuthorChip(r.State.Context(), mention.Message.GuildID, mention.GuildUser)
		chip.InsertText(text.TextView, text.Iter)
	}

	return ast.WalkContinue
}

func newAuthorChip(ctx context.Context, guildID discord.GuildID, user *discord.GuildUser) *author.Chip {
	name := user.DisplayOrUsername()
	color := defaultMentionColor

	if user.Member != nil {
		if user.Member.Nick != "" {
			name = user.Member.Nick
		}

		s := gtkcord.FromContext(ctx)
		c, ok := state.MemberColor(user.Member, func(id discord.RoleID) *discord.Role {
			r, _ := s.Cabinet.Role(guildID, id)
			return r
		})
		if ok {
			color = c.String()
		}
	}

	chip := author.NewChip(ctx, imgutil.HTTPProvider)
	chip.SetName(name)
	chip.SetColor(color)
	chip.SetAvatar(gtkcord.InjectAvatarSize(user.AvatarURL()))

	return chip
}
