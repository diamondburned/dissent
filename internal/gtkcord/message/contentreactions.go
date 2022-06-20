package message

import (
	"context"
	"strconv"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

type contentReactions struct {
	*gtk.FlowBox
	ctx       context.Context
	reactions map[string]*contentReaction
}

var reactionsCSS = cssutil.Applier("message-reactions", `
	.message-reactions {
		padding: 0;
	}
`)

func newContentReactions(ctx context.Context) *contentReactions {
	rs := contentReactions{
		ctx:       ctx,
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
}

func (rs *contentReactions) addReaction(reaction discord.Reaction) {
	name := reaction.Emoji.String()

	r, ok := rs.reactions[name]
	if ok {
		r.count++
		r.me = reaction.Me
		r.Invalidate()
		rs.InvalidateSort()
	} else {
		r = newContentReaction(rs.ctx, reaction)
		rs.reactions[name] = r
		rs.Insert(r, -1)
	}
}

type contentReaction struct {
	*gtk.FlowBoxChild
	countLabel *gtk.Label

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

func newContentReaction(ctx context.Context, reaction discord.Reaction) *contentReaction {
	r := contentReaction{
		count: reaction.Count,
		me:    reaction.Me,
	}

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)

	r.FlowBoxChild = gtk.NewFlowBoxChild()
	r.FlowBoxChild.SetName(reaction.Emoji.String())
	r.FlowBoxChild.SetChild(box)
	reactionCSS(r)

	r.countLabel = gtk.NewLabel("")
	r.countLabel.AddCSSClass("message-reaction-count")
	r.countLabel.SetHExpand(true)
	r.countLabel.SetXAlign(1)

	if reaction.Emoji.IsCustom() {
		emoji := onlineimage.NewPicture(ctx, imgutil.HTTPProvider)
		emoji.AddCSSClass("message-reaction-emoji")
		emoji.AddCSSClass("message-reaction-emoji-custom")
		emoji.SetSizeRequest(gtkcord.InlineEmojiSize, gtkcord.InlineEmojiSize)
		emoji.SetKeepAspectRatio(true)
		emoji.SetTooltipText(":" + reaction.Emoji.Name + ":")
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

	r.Invalidate()
	return &r
}

// Invalidate invalidates the widget state.
func (r *contentReaction) Invalidate() {
	r.countLabel.SetLabel(strconv.Itoa(r.count))

	if r.me {
		if !r.HasCSSClass("message-reaction-me") {
			r.AddCSSClass("message-reaction-me")
		}
	} else {
		r.RemoveCSSClass("message-reaction-me")
	}
}
