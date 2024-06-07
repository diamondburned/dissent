package messages

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"libdb.so/dissent/internal/gtkcord"
)

type messageReaction struct {
	discord.Reaction
	GuildID   discord.GuildID
	ChannelID discord.ChannelID
	MessageID discord.MessageID
}

func (r messageReaction) Equal(other messageReaction) bool {
	return true &&
		r.MessageID == other.MessageID &&
		r.ChannelID == other.ChannelID &&
		r.Me == other.Me &&
		r.Count == other.Count &&
		r.Emoji.APIString() == other.Emoji.APIString()
}

type contentReactions struct {
	*gtk.FlowBox

	// *gtk.ScrolledWindow
	// grid *gtk.GridView

	ctx       context.Context
	parent    *Content
	reactions *gioutil.ListModel[messageReaction]
}

var reactionsCSS = cssutil.Applier("message-reactions", `
	.message-reactions {
		padding: 0;
		margin-top: 4px;
		background: none;
	}
	.message-reactions > flowboxchild {
		margin: 4px 0;
		margin-right: 6px;
		padding: 0;
	}
`)

func newContentReactions(ctx context.Context, parent *Content) *contentReactions {
	rs := contentReactions{
		ctx:       ctx,
		parent:    parent,
		reactions: gioutil.NewListModel[messageReaction](),
	}

	// TODO: complain to the GTK devs about how broken GridView is.
	// Why is it not reflowing widgets? and other mysteries to solve in the GTK
	// framework.

	// rs.grid = gtk.NewGridView(
	// 	gtk.NewNoSelection(rs.reactions.ListModel),
	// 	newContentReactionsFactory(ctx))
	// rs.grid.SetOrientation(gtk.OrientationHorizontal)
	// reactionsCSS(rs.grid)
	//
	// rs.ScrolledWindow = gtk.NewScrolledWindow()
	// rs.ScrolledWindow.SetPolicy(gtk.PolicyNever, gtk.PolicyNever)
	// rs.ScrolledWindow.SetPropagateNaturalWidth(true)
	// rs.ScrolledWindow.SetPropagateNaturalHeight(false)
	// rs.ScrolledWindow.SetChild(rs.grid)

	rs.FlowBox = gtk.NewFlowBox()
	rs.FlowBox.SetOrientation(gtk.OrientationHorizontal)
	rs.FlowBox.SetHomogeneous(true)
	rs.FlowBox.SetMaxChildrenPerLine(30)
	rs.FlowBox.SetSelectionMode(gtk.SelectionNone)
	reactionsCSS(rs)

	rs.FlowBox.BindModel(rs.reactions.ListModel, func(o *glib.Object) gtk.Widgetter {
		reaction := gioutil.ObjectValue[messageReaction](o)
		w := newContentReaction()
		w.SetReaction(ctx, rs.FlowBox, reaction)
		return w
	})

	gtkutil.BindActionCallbackMap(rs, map[string]gtkutil.ActionCallback{
		"reactions.toggle": {
			ArgType: glib.NewVariantType("s"),
			Func: func(args *glib.Variant) {
				emoji := discord.APIEmoji(args.String())
				selected := rs.isReacted(emoji)

				client := gtkcord.FromContext(rs.ctx).Online()
				gtkutil.Async(rs.ctx, func() func() {
					var err error
					if selected {
						err = client.Unreact(rs.parent.ChannelID(), rs.parent.MessageID(), emoji)
					} else {
						err = client.React(rs.parent.ChannelID(), rs.parent.MessageID(), emoji)
					}

					if err != nil {
						if selected {
							err = fmt.Errorf("failed to react: %w", err)
						} else {
							err = fmt.Errorf("failed to unreact: %w", err)
						}
						app.Error(rs.ctx, err)
					}

					return nil
				})
			},
		},
	})

	return &rs
}

func (rs *contentReactions) findReactionIx(emoji discord.APIEmoji) int {
	var i int
	foundIx := -1

	iter := rs.reactions.All()
	iter(func(reaction messageReaction) bool {
		if reaction.Emoji.APIString() == emoji {
			foundIx = i
			return false
		}
		i++
		return true
	})

	return foundIx
}

func (rs *contentReactions) isReacted(emoji discord.APIEmoji) bool {
	ix := rs.findReactionIx(emoji)
	if ix == -1 {
		return false
	}
	return rs.reactions.At(ix).Me
}

// SetReactions sets the reactions of the message.
//
// TODO: implement Add and Remove event handlers directly in this container to
// avoid having to clear the whole list.
func (rs *contentReactions) SetReactions(reactions []discord.Reaction) {
	messageReactions := make([]messageReaction, len(reactions))
	for i, r := range reactions {
		messageReactions[i] = messageReaction{
			Reaction:  r,
			GuildID:   rs.parent.view.GuildID(),
			ChannelID: rs.parent.view.ChannelID(),
			MessageID: rs.parent.MessageID(),
		}
	}
	rs.reactions.Splice(0, rs.reactions.Len(), messageReactions...)
}

/*
func newContentReactionsFactory(ctx context.Context) *gtk.ListItemFactory {
	reactionWidgets := make(map[uintptr]*contentReaction)

	factory := gtk.NewSignalListItemFactory()
	factory.ConnectSetup(func(item *gtk.ListItem) {
		w := newContentReaction()
		item.SetChild(w)
		reactionWidgets[item.Native()] = w
	})
	factory.ConnectTeardown(func(item *gtk.ListItem) {
		item.SetChild(nil)
		delete(reactionWidgets, item.Native())
	})

	factory.ConnectBind(func(item *gtk.ListItem) {
		reaction := gioutil.ObjectValue[messageReaction](item.Item())

		w := reactionWidgets[item.Native()]
		w.SetReaction(ctx, reaction)
	})
	factory.ConnectUnbind(func(item *gtk.ListItem) {
		w := reactionWidgets[item.Native()]
		w.Clear()
	})

	return &factory.ListItemFactory
}
*/

type reactionsLoadState uint8

const (
	reactionsNotLoaded reactionsLoadState = iota
	reactionsLoading
	reactionsLoaded
)

type contentReaction struct {
	*gtk.ToggleButton
	iconBin    *adw.Bin
	countLabel *gtk.Label

	reaction messageReaction
	client   *gtkcord.State

	tooltip      string
	tooltipState reactionsLoadState
}

var reactionCSS = cssutil.Applier("message-reaction", `
	.message-reaction {
		/* min-width: 4em; */
		min-width: 0;
		min-height: 0;
		padding: 0;
	}
	.message-reaction > box {
		margin: 6px;
	}
	.message-reaction-emoji-icon {
		min-width:  22px;
		min-height: 22px;
	}
	.message-reaction-emoji-unicode {
		font-size: 18px;
	}
`)

func newContentReaction() *contentReaction {
	r := contentReaction{}

	r.ToggleButton = gtk.NewToggleButton()
	r.ToggleButton.AddCSSClass("message-reaction")
	r.ToggleButton.ConnectClicked(func() {
		r.SetSensitive(false)

		ok := r.ActivateAction("reactions.toggle", glib.NewVariantString(string(r.reaction.Emoji.APIString())))
		if !ok {
			slog.Error(
				"failed to activate reactions.toggle",
				"emoji", r.reaction.Emoji.APIString())
		}
	})

	r.ToggleButton.SetHasTooltip(true)
	r.ToggleButton.ConnectQueryTooltip(func(_, _ int, _ bool, tooltip *gtk.Tooltip) bool {
		tooltip.SetText(locale.Get("Loading..."))
		r.invalidateUsers(tooltip.SetMarkup)
		return true
	})

	r.iconBin = adw.NewBin()
	r.iconBin.AddCSSClass("message-reaction-icon")

	r.countLabel = gtk.NewLabel("")
	r.countLabel.AddCSSClass("message-reaction-count")
	r.countLabel.SetHExpand(true)
	r.countLabel.SetXAlign(1)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(r.iconBin)
	box.Append(r.countLabel)

	r.ToggleButton.SetChild(box)
	reactionCSS(r)

	return &r
}

// SetReaction sets the reaction of the widget.
func (r *contentReaction) SetReaction(ctx context.Context, flowBox *gtk.FlowBox, reaction messageReaction) {
	r.reaction = reaction
	r.client = gtkcord.FromContext(ctx).Online()

	if reaction.Emoji.IsCustom() {
		emoji := onlineimage.NewPicture(ctx, imgutil.HTTPProvider)
		emoji.AddCSSClass("message-reaction-emoji")
		emoji.AddCSSClass("message-reaction-emoji-custom")
		emoji.SetSizeRequest(gtkcord.InlineEmojiSize, gtkcord.InlineEmojiSize)
		emoji.SetKeepAspectRatio(true)
		emoji.SetURL(reaction.Emoji.EmojiURL())

		// TODO: get this working:
		// Currently, it just jitters in size. The button itself can still be
		// sized small, FlowBox is just forcing it to be big. This does mean
		// that it's not the GIF that is causing this.

		// anim := emoji.EnableAnimation()
		// anim.ConnectMotion(r)

		r.iconBin.SetChild(emoji)
	} else {
		label := gtk.NewLabel(reaction.Emoji.Name)
		label.AddCSSClass("message-reaction-emoji")
		label.AddCSSClass("message-reaction-emoji-unicode")

		r.iconBin.SetChild(label)
	}

	r.countLabel.SetLabel(strconv.Itoa(reaction.Count))

	r.ToggleButton.SetActive(reaction.Me)
	if reaction.Me {
		r.AddCSSClass("message-reaction-me")
	} else {
		r.RemoveCSSClass("message-reaction-me")
	}
}

func (r *contentReaction) Clear() {
	r.reaction = messageReaction{}
	r.client = nil
	r.tooltipState = reactionsNotLoaded
	r.iconBin.SetChild(nil)
	r.ToggleButton.SetActive(false)
	r.ToggleButton.RemoveCSSClass("message-reaction-me")
}

func (r *contentReaction) invalidateUsers(callback func(string)) {
	if r.tooltipState != reactionsNotLoaded {
		callback(r.tooltip)
		return
	}

	r.tooltipState = reactionsLoading
	r.tooltip = ""

	reaction := r.reaction
	client := r.client

	var tooltip string
	if reaction.Emoji.IsCustom() {
		tooltip = ":" + html.EscapeString(reaction.Emoji.Name) + ":\n"
	}

	done := func(tooltip string, err error) {
		glib.IdleAdd(func() {
			if !r.reaction.Equal(reaction) {
				// The reaction has changed,
				// so we don't care about the result.
				return
			}

			if err != nil {
				r.tooltipState = reactionsNotLoaded
				r.tooltip = tooltip + "<b>" + locale.Get("Error: ") + "</b>" + err.Error()

				slog.Error(
					"cannot load reaction tooltip",
					"channel", reaction.ChannelID,
					"message", reaction.MessageID,
					"emoji", reaction.Emoji.APIString(),
					"err", err)
			} else {
				r.tooltipState = reactionsLoaded
				r.tooltip = tooltip
			}

			callback(r.tooltip)
		})
	}

	go func() {
		u, err := client.Reactions(
			reaction.ChannelID,
			reaction.MessageID,
			reaction.Emoji.APIString(), 11)
		if err != nil {
			done(tooltip, err)
			return
		}

		var hasMore bool
		if len(u) > 10 {
			hasMore = true
			u = u[:10]
		}

		for _, user := range u {
			tooltip += fmt.Sprintf(
				`<span size="small">%s</span>`+"\n",
				client.MemberMarkup(reaction.GuildID, &discord.GuildUser{User: user}),
			)
		}

		if hasMore {
			tooltip += "..."
		} else {
			tooltip = strings.TrimRight(tooltip, "\n")
		}

		done(tooltip, nil)
	}()
}
