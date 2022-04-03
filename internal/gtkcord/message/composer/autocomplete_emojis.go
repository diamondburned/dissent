package composer

import (
	"context"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/autocomplete"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/sahilm/fuzzy"

	unicodeemoji "github.com/enescakir/emoji"
)

const (
	maxAutocompletion = 15
	emojiCacheExpiry  = time.Minute
)

type emojis []EmojiData

func (e emojis) String(i int) string { return e[i].Name }
func (e emojis) Len() int            { return len(e) }

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
	now := time.Now()

	if c.emojis != nil && c.updated.Add(emojiCacheExpiry).After(now) {
		return c.search(str)
	}

	c.updated = now

	if c.emojis != nil {
		c.emojis = c.emojis[:0]
	} else {
		c.emojis = make(emojis, 0, len(unicodeEmojis)+64)
	}

	for name, unicode := range unicodeEmojis {
		c.emojis = append(c.emojis, EmojiData{
			Name:    name,
			Content: unicode,
		})
	}

	state := gtkcord.FromContext(ctx)
	emojis, _ := state.EmojiState.Get(c.guildID)

	for _, guild := range emojis {
		for _, emoji := range guild.Emojis {
			c.emojis = append(c.emojis, EmojiData{
				ID:      emoji.ID,
				GuildID: guild.ID,
				Name:    emoji.Name,
				Content: emoji.String(),
			})
		}
	}

	return c.search(str)
}

func (c *emojiCompleter) search(str string) []autocomplete.Data {
	res := fuzzy.FindFrom(str, c.emojis)
	if len(res) > maxAutocompletion {
		res = res[:maxAutocompletion]
	}

	data := c.matched[:0]
	for _, r := range res {
		data = append(data, c.emojis[r.Index])
	}

	return data
}

// EmojiData is the Data structure for each emoji.
type EmojiData struct {
	ID      discord.EmojiID
	GuildID discord.GuildID
	Name    string
	Content string
}

const emojiSize = 32 // px

var _ = cssutil.WriteCSS(`
	.autocompleter-unicode {
		font-size: 26px;
	}
`)

// Row satisfies autocomplete.Data.
func (d EmojiData) Row(ctx context.Context) *gtk.ListBoxRow {
	b := gtk.NewBox(gtk.OrientationHorizontal, 4)

	if !d.ID.IsValid() {
		l := gtk.NewLabel(d.Content)
		l.AddCSSClass("autocompleter-unicode")

		b.Append(l)
	} else {
		// Use a background context so we don't constantly thrash the server
		// with cancelled requests every time we time.
		i := onlineimage.NewImage(context.Background(), imgutil.HTTPProvider)
		i.AddCSSClass("autocompleter-customemoji")
		i.SetSizeRequest(emojiSize, emojiSize)
		i.SetFromURL(gtkcord.EmojiURL(d.ID.String(), false))

		b.Append(i)
	}

	l := gtk.NewLabel(d.Name)
	l.SetMaxWidthChars(35)
	l.SetEllipsize(pango.EllipsizeEnd)
	b.Append(l)

	r := gtk.NewListBoxRow()
	r.AddCSSClass("autocomplete-emoji")
	r.SetChild(b)

	return r
}
