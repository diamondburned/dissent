package messages

import (
	_ "embed"
	"html"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"libdb.so/dissent/internal/gtkcord"
)

var summaryCSS = cssutil.Applier("message-summary-row", `
	.message-summary-row {
		margin: 0.25em 0;
	}
	.message-summary-symbol {
		font-size: 2em;
		min-width: calc((9px * 2) + {$message_avatar_size});
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
		if _, ok := v.rows[sw.key]; !ok {
			delete(v.summaries, id)
		}
	}
}

func (v *View) appendSummary(summary gateway.ConversationSummary) (messageKey, bool) {
	// Skip this summary if the EndID isn't in the current channel.
	endMsg, ok := v.rows[messageKeyID(summary.EndID)]
	if !ok {
		return "", false
	}

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
		v.rows[sw.key] = messageRow{
			ListBoxRow: row,
			info: messageInfo{
				author:    messageAuthor{userID: discord.NullUserID},
				timestamp: discord.Timestamp(summary.EndID.Time()),
			},
		}
		v.List.Insert(row, endMsg.Index()+1)

		reset := v.surroundingMessagesResetter(sw.key)
		reset()
	}

	state := gtkcord.FromContext(v.ctx).Offline()
	markups := formatSummary(state, v.guildID, summary)

	// Make everything small.
	markups.header = "<small>" + markups.header + "</small>"
	markups.title = "<small>" + markups.title + "</small>"
	markups.body = "<small>" + markups.body + "</small>"

	if markups.header != "" {
		sw.header.SetMarkup(markups.header)
		sw.header.SetVisible(false)
	} else {
		sw.header.SetVisible(true)
	}

	sw.title.SetMarkup(markups.title)
	sw.body.SetMarkup(markups.body)

	return sw.key, true
}

type summaryMarkups struct {
	header string
	title  string
	body   string
}

func formatSummary(state *gtkcord.State, guildID discord.GuildID, summary gateway.ConversationSummary) summaryMarkups {
	var markups summaryMarkups

	if people := summaryPeople(state, guildID, summary); people != "" {
		markups.header = "<small>" + people + ":</small>"
	}

	markups.title = "<b>" + html.EscapeString(summary.Topic) + "</b>"
	markups.body = html.EscapeString(summary.ShortSummary)

	return markups
}

// summaryPeople returns the markup listing the people in the summary, or an
// empty string if there are none.
func summaryPeople(state *gtkcord.State, guildID discord.GuildID, summary gateway.ConversationSummary) string {
	if len(summary.People) == 0 {
		return ""
	}

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

	var people strings.Builder
	switch len(summary.People) {
	case 1:
		people.WriteString(names[0])
	case 2:
		people.WriteString(names[0])
		people.WriteString(locale.Get(" and "))
		people.WriteString(names[1])
	case 3:
		people.WriteString(names[0])
		people.WriteString(locale.Get(", "))
		people.WriteString(names[1])
		people.WriteString(locale.Get(" and "))
		people.WriteString(names[2])
	default:
		people.WriteString(names[0])
		people.WriteString(locale.Get(", "))
		people.WriteString(names[1])
		people.WriteString(locale.Get(" and "))
		people.WriteString(locale.Sprintf("%d others", len(summary.People)-2))
	}

	return people.String()
}

var (
	//go:embed summaries.ui
	summariesUI string
	//go:embed summary_row.ui
	summaryRowUI string
)

var _ = cssutil.WriteCSS(`
	.message-summaries-popover > contents {
		padding: 0;
	}
	.message-summaries-heading {
		margin: 12px 12px 0 12px;
	}
	.message-summaries-list {
		margin: 12px;
	}
	.message-summary-item {
		padding: 10px 12px;
	}
`)

func (v *View) initSummariesPopover(popover *gtk.Popover) bool {
	popover.AddCSSClass("message-summaries-popover")
	state := gtkcord.FromContext(v.ctx).Offline()

	ui := gtk.NewBuilderFromString(summariesUI)
	list := ui.GetObject("list").Cast().(*gtk.ListBox)

	for _, summary := range state.SummaryState.Summaries(v.chID) {
		row := gtk.NewBuilderFromString(summaryRowUI)
		get := func(name string) glib.Objector { return row.GetObject(name).Cast() }

		start := summary.StartID.Time()

		meta := get("meta").(*gtk.Label)
		meta.SetText(locale.TimeAgo(start) + " · " + locale.Sprintf("%d messages", summary.Count))
		meta.SetTooltipText(locale.Time(start, true) + " – " + locale.Time(summary.EndID.Time(), true))

		get("topic").(*gtk.Label).SetText(summary.Topic)
		get("summary").(*gtk.Label).SetText(summary.ShortSummary)

		if people := summaryPeople(state, v.guildID, summary); people != "" {
			label := get("people").(*gtk.Label)
			label.SetMarkup(people)
			label.SetVisible(true)
		}

		// TODO: scroll to message on click.
		list.Append(get("rootContent").(*gtk.ListBoxRow))
	}

	popover.SetChild(ui.GetObject("rootContent").Cast().(gtk.Widgetter))
	return true
}
