package messages

import (
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

var summaryCSS = cssutil.Applier("message-summary-row", `
	.message-summary-row {
		margin: 0.25em 0;
	}
	.message-summary-symbol {
		font-size: 2em;
		min-width: calc((8px * 2) + {$message_avatar_size});
		min-height: calc(1em + 0.7rem);
	}
	.message-summary-title {
		margin: 0.1em 0;
	}
`)

type messageSummaryWidget struct {
	key    messageKey
	header *gtk.Label
	title  *gtk.Label
	body   *gtk.Label
}

func (v *View) updateSummaries(summaries []gateway.ConversationSummary) {
	if !showSummaries.Value() {
		return
	}

	for _, summary := range summaries {
		v.appendSummary(summary)
	}

	for id, sw := range v.summaries {
		if _, ok := v.msgs[sw.key]; !ok {
			delete(v.summaries, id)
		}
	}
}

func (v *View) appendSummary(summary gateway.ConversationSummary) messageKey {
	if v.summaries == nil {
		v.summaries = make(map[discord.Snowflake]messageSummaryWidget, 2)
	}

	sw, ok := v.summaries[summary.ID]
	if !ok {
		header := gtk.NewLabel("")
		header.AddCSSClass("message-summary-header")
		header.SetEllipsize(pango.EllipsizeEnd)
		header.SetXAlign(0)
		header.SetHExpand(true)

		title := gtk.NewLabel("")
		title.AddCSSClass("message-summary-title")
		title.SetWrap(true)
		title.SetXAlign(0)
		title.SetHExpand(true)

		body := gtk.NewLabel("")
		body.AddCSSClass("message-summary-body")
		body.SetWrap(true)
		body.SetXAlign(0)
		body.SetHExpand(true)

		sw = messageSummaryWidget{
			key:    messageKeyLocal(),
			header: header,
			title:  title,
			body:   body,
		}

		right := gtk.NewBox(gtk.OrientationVertical, 0)
		right.AddCSSClass("message-summary-right")
		right.SetHExpand(true)
		right.Append(header)
		right.Append(title)
		right.Append(body)

		symbol := gtk.NewLabel("∗")
		symbol.SetXAlign(0.5)
		symbol.SetYAlign(0.5)
		symbol.AddCSSClass("message-summary-symbol")
		symbol.SetTooltipText(locale.Get("Summary of this conversation"))

		box := gtk.NewBox(gtk.OrientationHorizontal, 0)
		box.Append(symbol)
		box.Append(right)

		row := gtk.NewListBoxRow()
		row.SetName(string(sw.key))
		row.SetChild(box)
		summaryCSS(row)

		// TODO: highlight relevant messages when hovered.
		// This will be a lot easier than just inserting this at the right
		// position, probably.

		v.summaries[summary.ID] = sw
		v.msgs[sw.key] = messageRow{ListBoxRow: row}

		refMsg, ok := v.msgs[messageKeyID(summary.EndID)]
		if ok {
			ix := refMsg.Index()
			v.List.Insert(row, ix)

			reset := v.surroundingMessagesResetter(sw.key)
			reset()
		} else {
			v.List.Append(row)
		}
	}

	state := gtkcord.FromContext(v.ctx).Offline()
	markups := formatSummary(state, v.guildID, summary)

	// Make everything small.
	markups.header = "<small>" + markups.header + "</small>"
	markups.title = "<small>" + markups.title + "</small>"
	markups.body = "<small>" + markups.body + "</small>"

	if markups.header != "" {
		sw.header.SetMarkup(markups.header)
		sw.header.Show()
	} else {
		sw.header.Hide()
	}

	sw.title.SetMarkup(markups.title)
	sw.body.SetMarkup(markups.body)

	return sw.key
}

type summaryMarkups struct {
	header string
	title  string
	body   string
}

func formatSummary(state *gtkcord.State, guildID discord.GuildID, summary gateway.ConversationSummary) summaryMarkups {
	var markups summaryMarkups

	if len(summary.People) > 0 {
		var header strings.Builder
		header.WriteString("<small>")

		names := make([]string, 0, min(3, len(summary.People)))
		for _, uID := range summary.People {
			name, _ := state.MemberDisplayName(guildID, uID)
			if name != "" {
				name = "<b>" + html.EscapeString(name) + "</b>"
			} else {
				name = locale.Get("?")
			}
			names = append(names, name)
			if len(names) == 3 {
				break
			}
		}

		switch len(summary.People) {
		case 1:
			header.WriteString(names[0])
		case 2:
			header.WriteString(names[0])
			header.WriteString(locale.Get(" and "))
			header.WriteString(names[1])
		case 3:
			header.WriteString(names[0])
			header.WriteString(locale.Get(", "))
			header.WriteString(names[1])
			header.WriteString(locale.Get(" and "))
			header.WriteString(names[2])
		default:
			header.WriteString(names[0])
			header.WriteString(locale.Get(", "))
			header.WriteString(names[1])
			header.WriteString(locale.Get(" and "))
			header.WriteString(locale.Sprintf("%d others", len(summary.People)-2))
		}

		header.WriteString(":</small>")
		markups.header = header.String()
	}

	markups.title = "<b>" + html.EscapeString(summary.Topic) + "</b>"
	markups.body = html.EscapeString(summary.ShortSummary)

	return markups
}

var _ = cssutil.WriteCSS(`
	.message-summaries-popover list {
		background-color: transparent;
	}
	.message-summary-item {
		margin: 0.25em 0;
	}
	.message-summary-item:not(:first-child):not(:last-child) {
		margin: 0.5em 0;
	}
	.message-summary-item label:nth-child(2) {
		margin-top: 0.1em;
	}
`)

func (v *View) initSummariesPopover(popover *gtk.Popover) {
	popover.AddCSSClass("message-summaries-popover")
	state := gtkcord.FromContext(v.ctx).Offline()

	summaries := state.SummaryState.Summaries(v.chID)
	if len(summaries) == 0 {
		placeholder := gtk.NewLabel(locale.Get("No message summaries available."))
		placeholder.AddCSSClass("message-summaries-placeholder")

		popover.SetChild(placeholder)
		return
	}

	list := gtk.NewBox(gtk.OrientationVertical, 0)
	list.AddCSSClass("message-summaries-list")
	list.SetHExpand(true)

	for _, summary := range summaries {
		markups := formatSummary(state, v.guildID, summary)

		header := gtk.NewLabel(fmt.Sprintf(
			`<span size="x-small">%s</span>`+"\n%s",
			locale.TimeAgo(summary.EndID.Time()),
			markups.header,
		))

		bottom := gtk.NewLabel(fmt.Sprintf(
			"%s\n%s",
			markups.title,
			markups.body,
		))

		for _, label := range []*gtk.Label{header, bottom} {
			label.AddCSSClass("popover-label")
			label.SetXAlign(0)
			label.SetHExpand(true)
			label.SetWrap(true)
			label.SetWrapMode(pango.WrapWordChar)
			label.SetUseMarkup(true)
		}

		box := gtk.NewBox(gtk.OrientationVertical, 0)
		box.AddCSSClass("message-summary-item")
		box.Append(header)
		box.Append(bottom)

		// TODO: add little user icons for participants.
		// we should probably use a grid for that.

		// TODO: scroll to message on click.
		list.Append(box)
	}

	scroll := gtk.NewScrolledWindow()
	scroll.SetPropagateNaturalWidth(true)
	scroll.SetPropagateNaturalHeight(true)
	scroll.SetMaxContentHeight(500)
	scroll.SetMaxContentWidth(300)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(list)

	popover.SetChild(scroll)
}
