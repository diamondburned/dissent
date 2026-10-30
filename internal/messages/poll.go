package messages

import (
	"context"
	_ "embed"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/dustin/go-humanize"
	"libdb.so/dissent/internal/gtkcord"
)

var (
	//go:embed poll.ui
	pollUI string
	//go:embed poll_answer.ui
	pollAnswerUI string
	//go:embed poll.css
	pollCSS string
)

var _ = cssutil.WriteCSS(pollCSS)

// pollWidth is the natural width of a poll.
const pollWidth = 360

// pollAnswerBarCSSf draws an answer's share of the votes as its background.
const pollAnswerBarCSSf = `
	.message-poll-answer {
		background-image: linear-gradient(to right, %s %.2f%%, transparent %.2f%%);
	}
`

// poll is the widget that shows a message's poll. It keeps the user's pending
// selection across updates.
type poll struct {
	*gtk.Box
	ctx   context.Context
	view  *View
	chID  discord.ChannelID
	msgID discord.MessageID

	question *gtk.Label
	hint     *gtk.Label
	answers  *gtk.Box
	status   *gtk.Label
	results  *gtk.Button
	action   *gtk.Button

	poll        *discord.Poll
	selected    map[int]bool
	showResults bool
	voting      bool
}

func newPoll(ctx context.Context, v *View, m *discord.Message) *poll {
	ui := gtk.NewBuilderFromString(pollUI)
	p := &poll{
		Box:      ui.GetObject("rootContent").Cast().(*gtk.Box),
		ctx:      ctx,
		view:     v,
		chID:     m.ChannelID,
		msgID:    m.ID,
		question: ui.GetObject("question").Cast().(*gtk.Label),
		hint:     ui.GetObject("hint").Cast().(*gtk.Label),
		answers:  ui.GetObject("answers").Cast().(*gtk.Box),
		status:   ui.GetObject("status").Cast().(*gtk.Label),
		results:  ui.GetObject("results").Cast().(*gtk.Button),
		action:   ui.GetObject("action").Cast().(*gtk.Button),
		selected: make(map[int]bool),
	}

	// Prefer being wide without stopping the window from getting narrower.
	header := ui.GetObject("header").Cast().(*gtk.Box)
	header.Prepend(newSizer(pollWidth, 0))

	p.results.ConnectClicked(func() {
		p.showResults = !p.showResults
		p.render()
	})
	p.action.ConnectClicked(p.onAction)

	p.Update(m.Poll)
	return p
}

// Update updates the poll with the given data.
func (p *poll) Update(data *discord.Poll) {
	p.poll = data
	if p.hasVoted() {
		clear(p.selected)
	}
	p.render()
}

func (p *poll) ended() bool {
	if p.poll.Results != nil && p.poll.Results.IsFinalized {
		return true
	}
	return p.poll.Expiry.IsValid() && time.Now().After(p.poll.Expiry.Time())
}

func (p *poll) hasVoted() bool {
	if p.poll.Results == nil {
		return false
	}
	for _, count := range p.poll.Results.AnswerCounts {
		if count.MeVoted {
			return true
		}
	}
	return false
}

func (p *poll) count(answerID int) discord.PollAnswerCount {
	if p.poll.Results != nil {
		for _, count := range p.poll.Results.AnswerCounts {
			if count.ID == answerID {
				return count
			}
		}
	}
	return discord.PollAnswerCount{ID: answerID}
}

func (p *poll) render() {
	ended := p.ended()
	voted := p.hasVoted()

	p.question.SetText(p.poll.Question.Text)

	p.hint.SetVisible(!ended && !voted)
	if p.poll.AllowMultiselect {
		p.hint.SetText(locale.Get("Select one or more answers"))
	} else {
		p.hint.SetText(locale.Get("Select one answer"))
	}

	var total, highest int
	for _, answer := range p.poll.Answers {
		count := p.count(answer.AnswerID).Count
		total += count
		highest = max(highest, count)
	}

	for child := p.answers.FirstChild(); child != nil; child = p.answers.FirstChild() {
		p.answers.Remove(child)
	}
	for _, answer := range p.poll.Answers {
		p.answers.Append(p.newAnswer(answer, total, highest))
	}

	status := locale.Sprintf("%d votes", total)
	switch {
	case ended:
		status += " · " + locale.Get("Poll closed")
	case p.poll.Expiry.IsValid():
		status += " · " + locale.Sprintf("%s left", strings.TrimSpace(humanize.RelTime(time.Now(), p.poll.Expiry.Time(), "", "")))
	}
	p.status.SetText(status)

	p.results.SetVisible(!ended && !voted)
	if p.showResults {
		p.results.SetLabel(locale.Get("Hide results"))
	} else {
		p.results.SetLabel(locale.Get("Show results"))
	}

	p.action.SetVisible(!ended)
	if voted {
		p.action.SetLabel(locale.Get("Remove vote"))
		p.action.RemoveCSSClass("suggested-action")
		p.action.SetSensitive(!p.voting)
	} else {
		p.action.SetLabel(locale.Get("Vote"))
		p.action.AddCSSClass("suggested-action")
		p.action.SetSensitive(!p.voting && len(p.selected) > 0)
	}
}

func (p *poll) newAnswer(answer discord.PollAnswer, total, highest int) gtk.Widgetter {
	ui := gtk.NewBuilderFromString(pollAnswerUI)
	button := ui.GetObject("rootContent").Cast().(*gtk.Button)
	check := ui.GetObject("check").Cast().(*gtk.Image)
	emoji := ui.GetObject("emoji").Cast().(*gtk.Box)
	text := ui.GetObject("text").Cast().(*gtk.Label)
	voted := ui.GetObject("voted").Cast().(*gtk.Image)
	percent := ui.GetObject("percent").Cast().(*gtk.Label)

	ended := p.ended()
	count := p.count(answer.AnswerID)
	selectable := !ended && !p.hasVoted()
	revealed := !selectable || p.showResults

	text.SetText(answer.PollMedia.Text)

	if e := answer.PollMedia.Emoji; e != nil {
		if e.IsCustom() {
			image := onlineimage.NewPicture(p.ctx, imgutil.HTTPProvider)
			image.SetSizeRequest(gtkcord.InlineEmojiSize, gtkcord.InlineEmojiSize)
			image.SetURL(e.EmojiURL())
			emoji.Append(image)
		} else {
			emoji.Append(gtk.NewLabel(e.Name))
		}
	} else {
		emoji.SetVisible(false)
	}

	check.SetVisible(selectable)
	if p.selected[answer.AnswerID] {
		check.SetFromIconName("checkbox-checked-symbolic")
		button.AddCSSClass("selected")
	}

	voted.SetVisible(count.MeVoted)
	if count.MeVoted {
		button.AddCSSClass("voted")
	}
	if ended && highest > 0 && count.Count == highest {
		button.AddCSSClass("winner")
	}

	var fraction float64
	if total > 0 {
		fraction = float64(count.Count) / float64(total)
	}

	if revealed {
		barColor := "alpha(@theme_fg_color, 0.1)"
		if count.MeVoted {
			barColor = "alpha(@accent_bg_color, 0.35)"
		}
		cssutil.Applyf(button, pollAnswerBarCSSf, barColor, fraction*100, fraction*100)
	}
	percent.SetVisible(revealed)
	percent.SetText(fmt.Sprintf("%.0f%%", fraction*100))
	percent.SetTooltipText(locale.Sprintf("%d votes", count.Count))

	if !selectable {
		button.SetCanTarget(false)
		button.SetFocusable(false)
		return button
	}

	button.ConnectClicked(func() {
		id := answer.AnswerID
		if p.selected[id] {
			delete(p.selected, id)
		} else {
			if !p.poll.AllowMultiselect {
				clear(p.selected)
			}
			p.selected[id] = true
		}
		p.render()
	})
	return button
}

func (p *poll) onAction() {
	var answerIDs []int
	if !p.hasVoted() {
		for _, answer := range p.poll.Answers {
			if p.selected[answer.AnswerID] {
				answerIDs = append(answerIDs, answer.AnswerID)
			}
		}
	}

	p.voting = true
	p.render()

	state := gtkcord.FromContext(p.ctx)
	gtkutil.Async(p.ctx, func() func() {
		err := state.VotePoll(p.chID, p.msgID, answerIDs...)
		return func() {
			p.voting = false
			p.render()

			if err != nil {
				toast := adw.NewToast(locale.Get("Cannot vote in poll"))
				toast.SetTimeout(5)
				p.view.AddToast(toast)
			}
		}
	})
}

// pollResultMarkup returns the system message markup for a PollResultMessage.
func pollResultMarkup(m *discord.Message) string {
	fields := map[string]string{}
	for _, embed := range m.Embeds {
		if embed.Type == discord.PollResultEmbed {
			for _, field := range embed.Fields {
				fields[field.Name] = field.Value
			}
		}
	}

	question := html.EscapeString(fields["poll_question_text"])
	if m.Reference != nil {
		question = fmt.Sprintf(`<a href="#message/%d">%s</a>`, m.Reference.MessageID, question)
	}

	total, _ := strconv.Atoi(fields["total_votes"])
	votes, _ := strconv.Atoi(fields["victor_answer_votes"])

	switch {
	case fields["victor_answer_id"] != "":
		victor := html.EscapeString(fields["victor_answer_text"])
		if emoji := fields["victor_answer_emoji_name"]; emoji != "" && fields["victor_answer_emoji_id"] == "" {
			victor = html.EscapeString(emoji) + " " + victor
		}
		return locale.Sprintf(
			"The poll %s has ended. <b>%s</b> won with %d of %d votes (%d%%).",
			question, victor, votes, total, votes*100/max(total, 1))
	case total == 0:
		return locale.Sprintf("The poll %s has ended with no votes.", question)
	default:
		return locale.Sprintf("The poll %s has ended in a tie.", question)
	}
}
