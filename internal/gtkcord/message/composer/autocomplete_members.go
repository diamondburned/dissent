package composer

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/autocomplete"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/sahilm/fuzzy"
)

const memberCacheExpiry = 15 * time.Second

type members []discord.Member

func (m members) String(i int) string { return m[i].Nick + m[i].User.Tag() }
func (m members) Len() int            { return len(m) }

type memberCompleter struct {
	members members
	matched []autocomplete.Data
	updated time.Time
	guildID discord.GuildID
}

// NewMemberCompleter creates a new autocomplete searcher that searches for
// members.
func NewMemberCompleter(gID discord.GuildID) autocomplete.Searcher {
	return &memberCompleter{
		guildID: gID,
		matched: make([]autocomplete.Data, 0, maxAutocompletion),
	}
}

func (c *memberCompleter) Rune() rune { return '@' }

func (c *memberCompleter) Search(ctx context.Context, str string) []autocomplete.Data {
	now := time.Now()

	if c.members != nil && c.updated.Add(memberCacheExpiry).After(now) {
		return c.search(str)
	}

	c.updated = now

	state := gtkcord.FromContext(ctx)

	mems, _ := state.Cabinet.Members(c.guildID)
	c.members = members(mems)

	if data := c.search(str); len(data) > 0 {
		return data
	}

	state.MemberState.SearchMember(c.guildID, str)
	return nil
}

func (c *memberCompleter) search(str string) []autocomplete.Data {
	res := fuzzy.FindFrom(str, c.members)
	if len(res) > maxAutocompletion {
		res = res[:maxAutocompletion]
	}

	data := c.matched[:0]
	for _, r := range res {
		data = append(data, MemberData(c.members[r.Index]))
	}

	return data
}

// MemberData is the Data structure for each member.
type MemberData discord.Member

func (d MemberData) Row(ctx context.Context) *gtk.ListBoxRow {
	i := onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, emojiSize)
	i.AddCSSClass("autocompleter-customemoji")
	i.SetFromURL(gtkcord.InjectAvatarSize(d.User.AvatarURLWithType(discord.PNGImage)))

	l := gtk.NewLabel("")
	l.SetMaxWidthChars(45)
	l.SetWrap(false)
	l.SetEllipsize(pango.EllipsizeEnd)
	l.SetXAlign(0)
	l.SetJustify(gtk.JustifyLeft)

	if d.Nick != "" {
		l.SetLines(2)
		l.SetMarkup(fmt.Sprintf(
			`%s`+"\n"+`<span size="smaller" fgalpha="75%%" rise="-1200">%s</span>`,
			d.Nick, d.User.Tag(),
		))
	} else {
		l.SetLines(1)
		l.SetText(d.User.Tag())
	}

	b := gtk.NewBox(gtk.OrientationHorizontal, 4)
	b.Append(i)
	b.Append(l)

	r := gtk.NewListBoxRow()
	r.AddCSSClass("autocomplete-member")
	r.SetChild(b)

	return r
}
