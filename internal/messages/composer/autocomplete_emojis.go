package composer

import (
	"context"
	"fmt"
	"html"
	"sort"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/autocomplete"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/ningen/v3/states/emoji"
	"github.com/sahilm/fuzzy"
	"libdb.so/dissent/internal/gtkcord"

	unicodeemoji "github.com/enescakir/emoji"
)

const (
	maxAutocompletion = 15
	emojiCacheExpiry  = time.Minute
)

type emojis []EmojiData

func (e emojis) Len() int            { return len(e) }
func (e emojis) String(i int) string { return e[i].EmojiName + " " + e[i].GuildName }

type emojiCompleter struct {
	emojis  emojis
	matched []autocomplete.Data
	updated time.Time
	guildID discord.GuildID
}

var unicodeEmojis = unicodeemoji.Map()

// NewEmojiCompleter creaets a new autocomplete searcher that searches for
// emojis.
func NewEmojiCompleter(gID discord.GuildID) autocomplete.Searcher {
	return &emojiCompleter{
		guildID: gID,
		matched: make([]autocomplete.Data, 0, maxAutocompletion),
	}
}

func (c *emojiCompleter) Rune() rune { return ':' }

func (c *emojiCompleter) Search(ctx context.Context, str string) []autocomplete.Data {
	if len(str) < 2 {
		return nil
	}

	now := time.Now()

	state := gtkcord.FromContext(ctx)
	hasNitro := state.EmojiState.HasNitro()

	if c.emojis != nil && c.updated.Add(emojiCacheExpiry).After(now) {
		return c.search(str, hasNitro)
	}

	c.updated = now

	if c.emojis != nil {
		c.emojis = c.emojis[:0]
	} else {
		c.emojis = make(emojis, 0, len(unicodeEmojis)+64)
	}

	for name, unicode := range unicodeEmojis {
		c.emojis = append(c.emojis, EmojiData{
			Emoji:     &discord.Emoji{Name: unicode},
			EmojiName: name,
		})
	}

	var emojis []emoji.Guild
	if showAllEmojis.Value() {
		emojis, _ = state.EmojiState.AllEmojis()
	} else {
		emojis, _ = state.EmojiState.ForGuild(c.guildID)
	}

	for i, guild := range emojis {
		for j, emoji := range guild.Emojis {
			c.emojis = append(c.emojis, EmojiData{
				GuildID:   guild.ID,
				GuildName: guild.Name,
				Emoji:     &emojis[i].Emojis[j],
				EmojiName: emoji.Name,
			})
		}
	}

	return c.search(str, hasNitro)
}

func (c *emojiCompleter) search(str string, hasNitro bool) []autocomplete.Data {
	res := fuzzy.FindFrom(str, c.emojis)
	if len(res) > maxAutocompletion {
		res = res[:maxAutocompletion]
	}

	data := c.matched[:0]
	for _, r := range res {
		data = append(data, c.emojis[r.Index])
	}

	// Put the guild emojis first if we don't have Nitro.
	if !hasNitro {
		sort.SliceStable(data, func(i, j int) bool {
			a := data[i].(EmojiData)
			b := data[j].(EmojiData)
			correctA := a.GuildID == c.guildID
			correctB := b.GuildID == c.guildID
			return correctA && !correctB
		})
	}

	return data
}

// EmojiData is the Data structure for each emoji.
type EmojiData struct {
	GuildID   discord.GuildID
	GuildName string
	Emoji     *discord.Emoji
	EmojiName string
}

const emojiSize = 32 // px

// Row satisfies autocomplete.Data.
func (d EmojiData) Row(ctx context.Context) *gtk.ListBoxRow {
	b := gtk.NewBox(gtk.OrientationHorizontal, 4)
	markup := html.EscapeString(d.EmojiName)

	if !d.Emoji.ID.IsValid() {
		l := gtk.NewLabel(d.Emoji.Name)
		l.AddCSSClass("autocompleter-unicode")

		b.Append(l)
	} else {
		i := onlineimage.NewImage(ctx, imgutil.HTTPProvider)
		i.AddCSSClass("autocompleter-customemoji")
		i.SetSizeRequest(emojiSize, emojiSize)
		i.SetFromURL(gtkcord.EmojiURL(d.Emoji.ID.String(), false))

		b.Append(i)

		markup += "\n" + fmt.Sprintf(
			`<span size="smaller" fgalpha="75%%" rise="-1200">%s</span>`,
			html.EscapeString(d.GuildName),
		)
	}

	l := gtk.NewLabel("")
	l.SetMaxWidthChars(35)
	l.SetEllipsize(pango.EllipsizeEnd)
	l.SetMarkup(markup)
	b.Append(l)

	r := gtk.NewListBoxRow()
	r.AddCSSClass("autocomplete-emoji")
	r.SetChild(b)

	return r
}
