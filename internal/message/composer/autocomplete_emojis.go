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
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3/states/emoji"
	"github.com/sahilm/fuzzy"

	unicodeemoji "github.com/enescakir/emoji"
)

const (
	maxAutocompletion = 15
	emojiCacheExpiry  = time.Minute
)

type emojis []EmojiData

func (e emojis) Len() int { return len(e) }

func (e emojis) String(i int) string {
	name := e[i].Name
	if e[i].Guild != nil {
		// Allow suffixing the guild name to search.
		name += " " + e[i].Guild.Name
	}
	return name
}

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

	var emojis []emoji.Guild
	if showAllEmojis.Value() {
		emojis, _ = state.EmojiState.AllEmojis()
	} else {
		emojis, _ = state.EmojiState.ForGuild(c.guildID)
	}

	hasNitro := state.EmojiState.HasNitro()

	for i, guild := range emojis {
		for _, emoji := range guild.Emojis {
			var content string
			// Check if the user can use the emoji if they have Nitro or if the
			// emoji is not animated and comes from the same guild thay they're
			// sending it to.
			if hasNitro || (guild.ID == c.guildID && !emoji.Animated) {
				// Use the default emoji format. This string is subject to
				// server-side validation.
				content = emoji.String()
			} else {
				// Use the emoji URL instead of the emoji code to allow
				// non-Nitro users to send emojis by sending the image URL.
				content = gtkcord.InjectSizeUnscaled(emoji.EmojiURL(), gtkcord.LargeEmojiSize)
				// Hint the user the emoji name.
				content += "#" + emoji.Name
			}

			c.emojis = append(c.emojis, EmojiData{
				Guild:   &emojis[i],
				ID:      emoji.ID,
				Name:    emoji.Name,
				Content: content,
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
			correctA := a.Guild != nil && a.Guild.ID == c.guildID
			correctB := b.Guild != nil && b.Guild.ID == c.guildID
			return correctA && !correctB
		})
	}

	return data
}

// EmojiData is the Data structure for each emoji.
type EmojiData struct {
	Guild   *emoji.Guild
	ID      discord.EmojiID
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
	markup := html.EscapeString(d.Name)

	if !d.ID.IsValid() {
		l := gtk.NewLabel(d.Content)
		l.AddCSSClass("autocompleter-unicode")

		b.Append(l)
	} else {
		i := onlineimage.NewImage(ctx, imgutil.HTTPProvider)
		i.AddCSSClass("autocompleter-customemoji")
		i.SetSizeRequest(emojiSize, emojiSize)
		i.SetFromURL(gtkcord.EmojiURL(d.ID.String(), false))

		b.Append(i)

		markup += "\n" + fmt.Sprintf(
			`<span size="smaller" fgalpha="75%%" rise="-1200">%s</span>`,
			html.EscapeString(d.Guild.Name),
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
