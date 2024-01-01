package messages

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

type contentReactions struct {
	*gtk.FlowBox
	ctx       context.Context
	reactions map[discord.APIEmoji]*contentReaction
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
		reactions: make(map[discord.APIEmoji]*contentReaction),
	}

	rs.FlowBox = gtk.NewFlowBox()
	rs.FlowBox.SetOrientation(gtk.OrientationHorizontal)
	rs.FlowBox.SetHomogeneous(true)
	rs.FlowBox.SetMaxChildrenPerLine(100)
	rs.FlowBox.SetSelectionMode(gtk.SelectionBrowse)
	rs.FlowBox.SetRowSpacing(2)
	rs.FlowBox.SetColumnSpacing(2)
	rs.FlowBox.SetActivateOnSingleClick(true)
	reactionsCSS(rs)

	rs.FlowBox.ConnectChildActivated(func(child *gtk.FlowBoxChild) {
		client := gtkcord.FromContext(rs.ctx)
		chID := rs.parent.ChannelID()
		msgID := rs.parent.MessageID()

		emoji := discord.APIEmoji(child.Name())
		selected := rs.reactions[emoji].me

		child.SetSensitive(false)
		go func() {
			var err error
			if selected {
				err = client.Unreact(chID, msgID, emoji)
			} else {
				err = client.React(chID, msgID, emoji)
			}

			if err != nil {
				if selected {
					err = fmt.Errorf("failed to react: %w", err)
				} else {
					err = fmt.Errorf("failed to unreact: %w", err)
				}
			}

			glib.IdleAdd(func() {
				child.SetSensitive(true)
				if err != nil {
					app.Error(rs.ctx, err)
				}
			})
		}()
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
	name := reaction.Emoji.APIString()

	r, ok := rs.reactions[name]
	if ok {
		r.count++
		r.me = reaction.Me
	} else {
		r = newContentReaction(rs, reaction)
		rs.reactions[name] = r

		// Manually search where this reaction should be inserted.
		pos := -1
		for _, curr := range rs.reactions {
			if curr.count > r.count {
				pos = curr.Index()
			}
		}
		rs.Insert(r, pos)
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
		/* background-color: alpha(@theme_selected_bg_color, 0.25); */
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
	r.FlowBoxChild.SetName(string(reaction.Emoji.APIString()))
	r.FlowBoxChild.SetChild(box)
	reactionCSS(r)

	var loadedUser bool
	r.FlowBoxChild.SetHasTooltip(true)
	r.FlowBoxChild.ConnectQueryTooltip(func(_, _ int, _ bool, tooltip *gtk.Tooltip) bool {
		if !loadedUser {
			loadedUser = true
			r.invalidateUsers(func() {
				tooltip.SetMarkup(r.tooltip)
			})
		}

		if !r.hasTooltip {
			tooltip.SetText(locale.Get("Loading..."))
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
		r.AddCSSClass("message-reaction-me")
		r.reactions.SelectChild(r.FlowBoxChild)
	} else {
		r.RemoveCSSClass("message-reaction-me")
		r.reactions.UnselectChild(r.FlowBoxChild)
	}
}

func (r *contentReaction) InvalidateUsers() {
	r.invalidateUsers(func() {})
}

func (r *contentReaction) invalidateUsers(done func()) {
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
			discord.NewAPIEmoji(r.emoji.ID, r.emoji.Name), 11,
		)
		if err != nil {
			log.Print("cannot fetch reactions for message ", r.reactions.parent.MessageID(), ": ", err)
			return done
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
			done()
		}
	})
}
