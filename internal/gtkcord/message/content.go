package message

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/chatkit/components/author"
	"github.com/diamondburned/chatkit/md"
	"github.com/diamondburned/chatkit/md/mdrender"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3/discordmd"
	"github.com/yuin/goldmark/ast"
)

// Content is the message content widget.
type Content struct {
	*gtk.Box
	ctx    context.Context
	parent *View
	menu   *gio.Menu
	view   *mdrender.MarkdownViewer
	react  *contentReactions
	child  []gtk.Widgetter
}

var contentCSS = cssutil.Applier("message-content-box", `
	.message-content-box {
		margin-right: 4px;
	}
	.message-reply-content,
	.message-reply-header {
		color: alpha(@theme_fg_color, 0.85);
	}
	.message-reply-header,
	.message-reply-box .mauthor-chip {
		font-size: 0.9em;
	}
	.message-reply-content {
		font-size: 0.95em;
	}
`)

// NewContent creates a new Content widget.
func NewContent(ctx context.Context, v *View) *Content {
	c := Content{
		ctx:    ctx,
		parent: v,
		child:  make([]gtk.Widgetter, 0, 2),
	}
	c.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	contentCSS(c.Box)

	return &c
}

// SetExtraMenu implements ExtraMenuSetter.
func (c *Content) SetExtraMenu(menu gio.MenuModeller) {
	c.menu = gio.NewMenu()
	c.menu.InsertSection(0, "Message", menu)

	if c.view != nil {
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

	gtkutil.WalkWidget(c.Box, func(w gtk.Widgetter) bool {
		s, ok := w.(extraMenuSetter)
		if ok {
			s.SetExtraMenu(menu)
		}
		return false
	})
}

var systemContentCSS = cssutil.Applier("message-system-content", `
	.message-system-content {
		font-style: italic;
		color: alpha(@theme_fg_color, 0.9);
	}
`)

// Update replaces Content with the message.
func (c *Content) Update(m *discord.Message, customs ...gtk.Widgetter) {
	c.clear()

	state := gtkcord.FromContext(c.ctx)

	if m.Reference != nil {
		header := gtk.NewLabel("<a href=\"#\">Replying to</a> ")
		header.AddCSSClass("message-reply-header")
		header.SetUseMarkup(true)
		header.ConnectActivateLink(func(string) bool {
			c.parent.ScrollToMessage(m.ID)
			return true
		})

		topBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
		topBox.SetHAlign(gtk.AlignStart)
		topBox.Append(header)

		replyBox := gtk.NewBox(gtk.OrientationVertical, 0)
		replyBox.AddCSSClass("md-blockquote")
		replyBox.AddCSSClass("message-reply-box")
		replyBox.Append(topBox)

		msg := m.ReferencedMessage
		if msg == nil {
			msg, _ = state.Cabinet.Message(m.Reference.ChannelID, m.Reference.MessageID)
		}
		if msg != nil {
			if state.UserIsBlocked(msg.Author.ID) {
				header.SetLabel(header.Label() + "blocked user.")
			} else {
				member, _ := state.Cabinet.Member(m.Reference.GuildID, msg.Author.ID)
				chip := newAuthorChip(c.ctx, m.GuildID, &discord.GuildUser{
					User:   msg.Author,
					Member: member,
				})
				chip.Unpad()
				topBox.Append(chip)

				if preview := state.MessagePreview(msg); preview != "" {
					// Force single line.
					reply := gtk.NewLabel(strings.ReplaceAll(preview, "\n", "  "))
					reply.AddCSSClass("message-reply-content")
					reply.SetTooltipText(preview)
					reply.SetEllipsize(pango.EllipsizeEnd)
					reply.SetLines(1)
					reply.SetXAlign(0)

					replyBox.Append(reply)
					c.append(replyBox)
				}
			}
		} else {
			header.SetLabel(header.Label() + " unknown message.")
		}
	}

	var messageMarkup string
	switch m.Type {
	case discord.GuildMemberJoinMessage:
		messageMarkup = "Joined the server."
	case discord.CallMessage:
		messageMarkup = "Calling you."
	case discord.ChannelIconChangeMessage:
		messageMarkup = "Changed the channel icon."
	case discord.ChannelNameChangeMessage:
		messageMarkup = "Changed the channel name to #" + html.EscapeString(m.Content) + "."
	case discord.ChannelPinnedMessage:
		messageMarkup = fmt.Sprintf(`Pinned <a href="#message/%d">a message.</a>`, m.ID)
	case discord.RecipientAddMessage, discord.RecipientRemoveMessage:
		mentioned := state.MemberMarkup(m.GuildID, &m.Mentions[0], author.WithMinimal())
		switch m.Type {
		case discord.RecipientAddMessage:
			messageMarkup = "Added " + mentioned + " to the group."
		case discord.RecipientRemoveMessage:
			messageMarkup = "Removed " + mentioned + " from the group."
		}
	case discord.NitroBoostMessage:
		messageMarkup = "Boosted the server!"
	case discord.NitroTier1Message:
		messageMarkup = "The server is now Nitro Boosted to Tier 1."
	case discord.NitroTier2Message:
		messageMarkup = "The server is now Nitro Boosted to Tier 2."
	case discord.NitroTier3Message:
		messageMarkup = "The server is now Nitro Boosted to Tier 3."
	}

	c.view = nil

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
					c.parent.ScrollToMessage(discord.MessageID(id))
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
		l.SetXAlign(0)
		l.SetSelectable(true)
		c.append(l)

	// We don't render the message content if all it is is the URL to the
	// embedded image, because that's what the official client does.
	case len(m.Embeds) != 1 ||
		m.Embeds[0].Type != discord.ImageEmbed ||
		m.Embeds[0].URL != m.Content:

		src := []byte(m.Content)
		node := discordmd.ParseWithMessage(src, *state.Cabinet, m, true)

		c.view = mdrender.NewMarkdownViewer(c.ctx, src, node, renderers...)
		c.append(c.view)
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

func (c *Content) append(w gtk.Widgetter) {
	c.Box.Append(w)
	c.child = append(c.child, w)
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

	red := gtk.NewLabel("Redacted.")
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
		c.react = newContentReactions(c.ctx)
		c.append(c.react)
	}

	c.react.Clear()
	c.react.AddReactions(reactions)
}

var renderers = []mdrender.OptionFunc{
	mdrender.WithRenderer(discordmd.KindEmoji, renderEmoji),
	mdrender.WithRenderer(discordmd.KindInline, renderInline),
	mdrender.WithRenderer(discordmd.KindMention, renderMention),
}

func renderEmoji(r *mdrender.Renderer, n ast.Node) ast.WalkStatus {
	emoji := n.(*discordmd.Emoji)
	text := r.State.TextBlock()

	picture := onlineimage.NewPicture(r.State.Context(), imgutil.HTTPProvider)
	picture.EnableAnimation().OnHover()
	picture.SetKeepAspectRatio(true)
	picture.SetTooltipText(emoji.Name)
	picture.SetURL(gtkcord.EmojiURL(emoji.ID, emoji.GIF))

	v := md.InsertCustomImageWidget(text.TextView, text.Buffer.CreateChildAnchor(text.Iter), picture)
	if emoji.Large {
		v.SetSizeRequest(gtkcord.LargeEmojiSize, gtkcord.LargeEmojiSize)
	} else {
		v.SetSizeRequest(gtkcord.InlineEmojiSize, gtkcord.InlineEmojiSize)
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
	name := user.Username
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
