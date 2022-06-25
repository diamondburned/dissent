package message

import (
	"context"
	"html"
	"log"
	"strconv"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

type contentReactions struct {
	*gtk.FlowBox
	ctx       context.Context
	reactions map[string]*contentReaction
	parent    *Content
}

var reactionsCSS = cssutil.Applier("message-reactions", `
	.message-reactions {
		padding: 0;
		margin-top: 2px;
	}
`)

func newContentReactions(ctx context.Context, parent *Content) *contentReactions {
	rs := contentReactions{
		ctx:       ctx,
		parent:    parent,
		reactions: make(map[string]*contentReaction),
	}

	rs.FlowBox = gtk.NewFlowBox()
	rs.FlowBox.SetOrientation(gtk.OrientationHorizontal)
	rs.FlowBox.SetHomogeneous(true)
	rs.FlowBox.SetMaxChildrenPerLine(100)
	rs.FlowBox.SetSelectionMode(gtk.SelectionNone)
	reactionsCSS(rs)

	rs.FlowBox.SetSortFunc(func(child1, child2 *gtk.FlowBoxChild) int {
		name1 := child1.Name()
		name2 := child2.Name()

		react1 := rs.reactions[name1]
		react2 := rs.reactions[name2]

		if react1.count != react2.count {
			if react1.count > react2.count {
				return 1
			}
			return -1
		}

		return strings.Compare(name1, name2)
	})

	return &rs
}

func (rs *contentReactions) Clear() {
	for k, child := range rs.reactions {
		rs.Remove(child)
		delete(rs.reactions, k)
	}
}

func (rs *contentReactions) AddReactions(reactions []discord.Reaction) {
	for _, react := range reactions {
		rs.addReaction(react)
	}
	for _, reaction := range rs.reactions {
		reaction.Invalidate()
	}
}

func (rs *contentReactions) addReaction(reaction discord.Reaction) {
	name := reaction.Emoji.String()

	r, ok := rs.reactions[name]
	if ok {
		r.count++
		r.me = reaction.Me
	} else {
		r = newContentReaction(rs, reaction)
		rs.reactions[name] = r
		rs.Insert(r, -1)
	}
}

type contentReaction struct {
	*gtk.FlowBoxChild
	reactions  *contentReactions
	countLabel *gtk.Label

	tooltip    string
	hasTooltip bool

	emoji discord.Emoji
	count int
	me    bool
}

var reactionCSS = cssutil.Applier("message-reaction", `
	.message-reaction {
		border: 1px solid @borders;
		padding: 2px 4px;
	}
	.message-reaction-me {
		background-color: alpha(@theme_selected_bg_color, 0.25);
	}
	.message-reaction-count {
		margin-left: 4px;
	}
	.message-reaction-emoji-unicode {
		font-size: 18px;
	}
	.message-reaction-emoji-custom {
		min-width:  22px;
		min-height: 22px;
	}
`)

func newContentReaction(rs *contentReactions, reaction discord.Reaction) *contentReaction {
	r := contentReaction{
		reactions: rs,
		emoji:     reaction.Emoji,
		count:     reaction.Count,
		me:        reaction.Me,
	}

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)

	r.FlowBoxChild = gtk.NewFlowBoxChild()
	r.FlowBoxChild.SetName(reaction.Emoji.String())
	r.FlowBoxChild.SetChild(box)
	reactionCSS(r)

	var loadedUser bool
	r.FlowBoxChild.SetHasTooltip(true)
	r.FlowBoxChild.ConnectQueryTooltip(func(_, _ int, _ bool, tooltip *gtk.Tooltip) bool {
		if !loadedUser {
			loadedUser = true
			r.InvalidateUsers()
		}

		if !r.hasTooltip {
			tooltip.SetText("Loading...")
			return true
		}

		tooltip.SetMarkup(r.tooltip)
		return r.tooltip != ""
	})

	r.countLabel = gtk.NewLabel("")
	r.countLabel.AddCSSClass("message-reaction-count")
	r.countLabel.SetHExpand(true)
	r.countLabel.SetXAlign(1)

	if reaction.Emoji.IsCustom() {
		emoji := onlineimage.NewPicture(rs.ctx, imgutil.HTTPProvider)
		emoji.AddCSSClass("message-reaction-emoji")
		emoji.AddCSSClass("message-reaction-emoji-custom")
		emoji.SetSizeRequest(gtkcord.InlineEmojiSize, gtkcord.InlineEmojiSize)
		emoji.SetKeepAspectRatio(true)
		emoji.SetURL(reaction.Emoji.EmojiURL())

		anim := emoji.EnableAnimation()
		anim.ConnectMotion(r)

		box.Append(emoji)
	} else {
		label := gtk.NewLabel(reaction.Emoji.Name)
		label.AddCSSClass("message-reaction-emoji")
		label.AddCSSClass("message-reaction-emoji-unicode")

		box.Append(label)
	}

	box.Append(r.countLabel)

	return &r
}

// Invalidate invalidates the widget state.
func (r *contentReaction) Invalidate() {
	r.FlowBoxChild.Changed()
	r.countLabel.SetLabel(strconv.Itoa(r.count))

	if r.me {
		if !r.HasCSSClass("message-reaction-me") {
			r.AddCSSClass("message-reaction-me")
		}
	} else {
		r.RemoveCSSClass("message-reaction-me")
	}
}

func (r *contentReaction) InvalidateUsers() {
	if r.emoji.IsCustom() {
		r.tooltip = ":" + html.EscapeString(r.emoji.Name) + ":\n"
		r.hasTooltip = true
	}

	tooltip := r.tooltip

	gtkutil.Async(r.reactions.ctx, func() func() {
		client := gtkcord.FromContext(r.reactions.ctx)

		u, err := client.Reactions(
			r.reactions.parent.view.ChannelID(),
			r.reactions.parent.MessageID(),
			discord.NewCustomEmoji(r.emoji.ID, r.emoji.Name), 11,
		)
		if err != nil {
			log.Print("cannot fetch reactions for message ", r.reactions.parent.MessageID(), ": ", err)
			return nil
		}

		var hasMore bool
		if len(u) > 10 {
			hasMore = true
			u = u[:10]
		}

		for _, user := range u {
			tooltip += client.MemberMarkup(
				r.reactions.parent.view.GuildID(),
				&discord.GuildUser{User: user},
			)
			tooltip += "\n"
		}

		if hasMore {
			tooltip += "..."
		}

		tooltip = strings.TrimSuffix(tooltip, "\n")

		return func() {
			r.tooltip = tooltip
			r.hasTooltip = true
		}
	})
}
