package channels

import (
	"sort"

	"github.com/diamondburned/arikawa/v3/discord"
)

// drainer provides a drain method that filters and removes channels that the
// callback returns true on.
type drainer []discord.Channel

func (d *drainer) sort() {
	s := *d
	sort.Slice(s, func(i, j int) bool {
		return s[i].Position < s[j].Position
	})
}

func (d *drainer) drain(f func(ch discord.Channel) bool) {
	old := *d
	*d = (*d)[:0]

	for _, ch := range old {
		if !f(ch) {
			*d = append(*d, ch)
		}
	}
}
