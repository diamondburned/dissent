package messages

import (
	"context"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/md"
	"github.com/diamondburned/chatkit/md/block"
	"github.com/diamondburned/chatkit/md/mdrender"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/ningen/v3/discordmd"
	"github.com/yuin/goldmark/ast"
	"libdb.so/ctxt"
	"libdb.so/dissent/internal/gtkcord"
)

type markdownState struct {
	bindedSpoilerBlocks map[*block.TextBlock]struct{}
}

func newMarkdownState() *markdownState {
	return &markdownState{
		bindedSpoilerBlocks: make(map[*block.TextBlock]struct{}),
	}
}

func mustMarkdownState(ctx context.Context) *markdownState {
	state, ok := ctxt.From[*markdownState](ctx)
	if !ok {
		panic("invalid markdown state")
	}
	return state
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

func renderEmoji(ctx context.Context, r *mdrender.Renderer, n ast.Node) ast.WalkStatus {
	emoji := n.(*discordmd.Emoji)
	text := r.State(ctx).TextBlock()

	picture := onlineimage.NewPicture(ctx, imgutil.HTTPProvider)
	picture.EnableAnimation().OnHover()
	picture.SetContentFit(gtk.ContentFitContain)
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

func getSpoilerColor(state *block.ContainerState, alpha float32) string {
	l := gtk.NewLabel("")
	l.AddCSSClass("md-spoiler")

	c := state.Viewer.StyleContext().Color()
	if !textutil.ColorIsDark(c.Red(), c.Green(), c.Blue()) {
		l.AddCSSClass("dark")
	}

	c = l.StyleContext().Color()
	c.SetAlpha(alpha)
	return c.String()
}

func renderInline(ctx context.Context, r *mdrender.Renderer, n ast.Node) ast.WalkStatus {
	inline := n.(*discordmd.Inline)

	stateInternal := mustMarkdownState(ctx)
	state := r.State(ctx)
	text := state.TextBlock()

	// Only bind our cursor position listener if we were not in a text block,
	// since this text block is a new one.
	if _, binded := stateInternal.bindedSpoilerBlocks[text]; !binded {
		stateInternal.bindedSpoilerBlocks[text] = struct{}{}
		slog.Debug(
			"binding cursor-position handler for text block",
			"native", text.TextView.Object.Native())

		// Handle the spoiler being revealed.
		text.Buffer.NotifyProperty("cursor-position", func() {
			insert := text.Buffer.GetInsert()
			insertIter := text.Buffer.IterAtMark(insert)

			spoilerTag := state.Viewer.TagTable().Lookup("spoiler")
			if spoilerTag != nil && insertIter.HasTag(spoilerTag) {
				spoilerStart := insertIter.Copy()
				spoilerStart.BackwardToTagToggle(spoilerTag)

				spoilerEnd := insertIter.Copy()
				spoilerEnd.ForwardToTagToggle(spoilerTag)

				slog.Debug(
					"clicked on spoiler tag",
					"start", spoilerStart.Offset(),
					"end", spoilerEnd.Offset(),
					"native", text.TextView.Object.Native())

				text.Buffer.RemoveTag(spoilerTag, spoilerStart, spoilerEnd)

				revealedTagAttrs := textutil.TextTag{
					"background": getSpoilerColor(state, 0.75),
				}
				revealedTag := revealedTagAttrs.FromTable(state.Viewer.TagTable(), "spoiler-revealed")
				text.Buffer.ApplyTag(revealedTag, spoilerStart, spoilerEnd)
			}
		})
	}

	startIx := text.Iter.Offset()

	if inline.Attr.Has(discordmd.AttrSpoiler) {
		// Pad with a space.
		text.Insert(" ")
	}

	// Render everything inside. We'll wrap the whole region with tags.
	r.RenderChildren(ctx, n)

	if inline.Attr.Has(discordmd.AttrSpoiler) {
		text.Insert(" ")
	}

	start := text.Buffer.IterAtOffset(startIx)
	end := text.Iter

	for tag, htmltag := range htmlTagMap {
		if inline.Attr.Has(tag) {
			tag := text.Tag(htmltag)
			text.Buffer.ApplyTag(tag, start, end)
		}
	}

	if inline.Attr.Has(discordmd.AttrSpoiler) {
		spoilerColor := getSpoilerColor(state, 1.0)
		tagAttrs := textutil.TextTag{
			"background": spoilerColor,
			"foreground": spoilerColor,
		}

		tag := tagAttrs.FromTable(state.Viewer.TagTable(), "spoiler")
		text.Buffer.ApplyTag(tag, start, end)
	}

	return ast.WalkSkipChildren
}

// rgba(111, 120, 219, 0.3)
const defaultMentionColor = "#6F78DB"

func mentionTag(ctx context.Context, r *mdrender.Renderer, color string) *gtk.TextTag {
	tag := textutil.TextTag{"background": color + "76"}
	return tag.FromTable(r.State(ctx).TagTable(), tag.Hash())
}

func renderMention(ctx context.Context, r *mdrender.Renderer, n ast.Node) ast.WalkStatus {
	mention := n.(*discordmd.Mention)

	text := r.State(ctx).TextBlock()

	switch {
	case mention.Channel != nil:
		text.TagBounded(mentionTag(ctx, r, defaultMentionColor), func() {
			text.Insert(" #" + mention.Channel.Name + " ")
		})

	case mention.GuildRole != nil:
		roleColor := defaultMentionColor
		if mention.GuildRole.Color != discord.NullColor {
			roleColor = mention.GuildRole.Color.String()
		}

		text.TagBounded(mentionTag(ctx, r, roleColor), func() {
			text.Insert(" @" + mention.GuildRole.Name + " ")
		})

	case mention.GuildUser != nil:
		chip := newAuthorChip(ctx, mention.Message.GuildID, mention.GuildUser)
		chip.InsertText(text.TextView, text.Iter)
	}

	return ast.WalkContinue
}
