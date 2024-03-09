package messages

import (
	"context"
	"slices"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/chatkit/components/author"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"libdb.so/dissent/internal/gtkcord"
)

const typerTimeout = 10 * time.Second

// TypingIndicator is a struct that represents a typing indicator box.
type TypingIndicator struct {
	*gtk.Revealer
	child struct {
		*gtk.Box
		Dots  gtk.Widgetter
		Label *gtk.Label
	}

	typers  []typingTyper
	state   *gtkcord.State
	chID    discord.ChannelID
	guildID discord.GuildID
}

type typingTyper struct {
	UserMarkup string
	UserID     discord.UserID
	When       discord.UnixTimestamp
}

var typingIndicatorCSS = cssutil.Applier("messages-typing-indicator", `
	.messages-typing-box {
		padding: 1px 15px;
		font-size: 0.85em;
	}
	.messages-typing-box .messages-breathing-dots {
		margin-right: 11px;
	}
`)

// NewTypingIndicator creates a new TypingIndicator.
func NewTypingIndicator(ctx context.Context, chID discord.ChannelID) *TypingIndicator {
	state := gtkcord.FromContext(ctx)

	t := &TypingIndicator{
		Revealer: gtk.NewRevealer(),
		typers:   make([]typingTyper, 0, 3),
		state:    state,
		chID:     chID,
	}

	ch, _ := state.Cabinet.Channel(chID)
	if ch != nil {
		t.guildID = ch.GuildID
	}

	t.child.Dots = newBreathingDots()

	t.child.Label = gtk.NewLabel("")
	t.child.Label.AddCSSClass("messages-typing-label")
	t.child.Label.SetHExpand(true)
	t.child.Label.SetXAlign(0)
	t.child.Label.SetWrap(false)
	t.child.Label.SetEllipsize(pango.EllipsizeEnd)
	t.child.Label.SetSingleLineMode(true)

	t.child.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	t.child.Box.AddCSSClass("messages-typing-box")
	t.child.Box.Append(t.child.Dots)
	t.child.Box.Append(t.child.Label)

	t.SetTransitionType(gtk.RevealerTransitionTypeSlideUp)
	t.SetOverflow(gtk.OverflowHidden)
	t.SetChild(t.child.Box)
	typingIndicatorCSS(t)

	state.AddHandlerForWidget(t,
		func(ev *gateway.TypingStartEvent) {
			if ev.ChannelID != chID {
				return
			}
			t.AddTyperMember(ev.UserID, ev.Timestamp, ev.Member)
		},
		func(ev *gateway.MessageCreateEvent) {
			if ev.ChannelID != chID {
				return
			}
			t.RemoveTyper(ev.Author.ID)
		},
	)
	t.updateAndScheduleNext()

	return t
}

// AddTyper adds a typer to the typing indicator.
func (t *TypingIndicator) AddTyper(userID discord.UserID, when discord.UnixTimestamp) {
	t.AddTyperMember(userID, when, nil)
}

// AddTyperMember adds a typer to the typing indicator with a member object.
func (t *TypingIndicator) AddTyperMember(userID discord.UserID, when discord.UnixTimestamp, member *discord.Member) {
	defer t.updateAndScheduleNext()

	ix := slices.IndexFunc(t.typers, func(t typingTyper) bool { return t.UserID == userID })
	if ix != -1 {
		t.typers[ix].When = when
		return
	}

	mods := []author.MarkupMod{author.WithMinimal()}

	var markup string
	if member != nil {
		markup = t.state.MemberMarkup(t.guildID, &discord.GuildUser{
			User:   member.User,
			Member: member,
		}, mods...)
	} else {
		markup = t.state.UserIDMarkup(t.chID, userID, mods...)
	}

	t.typers = append(t.typers, typingTyper{
		UserMarkup: markup,
		UserID:     userID,
		When:       when,
	})
}

// RemoveTyper removes a typer from the typing indicator.
func (t *TypingIndicator) RemoveTyper(userID discord.UserID) {
	t.typers = slices.DeleteFunc(t.typers, func(t typingTyper) bool { return t.UserID == userID })
	t.updateAndScheduleNext()
}

// updateAndScheduleNext updates the typing indicator and schedules the next
// cleanup using TimeoutAdd.
func (t *TypingIndicator) updateAndScheduleNext() {
	now := time.Now()
	earliest := discord.UnixTimestamp(now.Add(-typerTimeout).Unix())

	nowUnix := discord.UnixTimestamp(now.Unix())
	next := nowUnix

	typers := t.typers[:0]
	for _, typer := range t.typers {
		if typer.When > earliest {
			typers = append(typers, typer)
			next = min(next, typer.When)
		}
	}
	for i := len(typers); i < len(t.typers); i++ {
		// Prevent memory leaks.
		t.typers[i] = typingTyper{}
	}
	t.typers = typers

	if len(t.typers) == 0 {
		t.SetRevealChild(false)
		return
	}

	slices.SortFunc(t.typers, func(a, b typingTyper) int {
		return int(a.When - b.When)
	})

	t.SetRevealChild(true)
	t.child.Label.SetMarkup(renderTypingMarkup(t.typers))

	// Schedule the next cleanup.
	// Prevent rounding errors by adding a small buffer.
	cleanUpInSeconds := uint(next-nowUnix) + 1
	glib.TimeoutSecondsAdd(cleanUpInSeconds, func() {
		t.updateAndScheduleNext()
	})
}

func renderTypingMarkup(typers []typingTyper) string {
	switch len(typers) {
	case 0:
		return ""
	case 1:
		return locale.Sprintf(
			"%s is typing...",
			typers[0].UserMarkup,
		)
	case 2:
		return locale.Sprintf(
			"%s and %s are typing...",
			typers[0].UserMarkup, typers[1].UserMarkup,
		)
	case 3:
		return locale.Sprintf(
			"%s, %s and %s are typing...",
			typers[0].UserMarkup, typers[1].UserMarkup, typers[2].UserMarkup,
		)
	default:
		return locale.Get(
			"Several people are typing...",
		)
	}
}

var breathingDotsCSS = cssutil.Applier("messages-breathing-dots", `
	@keyframes messages-breathing {
		0% {   opacity: 0.66; }
		100% { opacity: 0.12; }
	}
	.messages-breathing-dots label {
		animation: messages-breathing 800ms infinite alternate;
	}
	.messages-breathing-dots label:nth-child(1) { animation-delay: 000ms; }
	.messages-breathing-dots label:nth-child(2) { animation-delay: 150ms; }
	.messages-breathing-dots label:nth-child(3) { animation-delay: 300ms; }
`)

func newBreathingDots() gtk.Widgetter {
	const ch = "●"

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(gtk.NewLabel(ch))
	box.Append(gtk.NewLabel(ch))
	box.Append(gtk.NewLabel(ch))
	breathingDotsCSS(box)

	return box
}
