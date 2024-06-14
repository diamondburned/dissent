package quickswitcher

import (
	"context"
	"log"
	"slices"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotkit/app"
	"github.com/sahilm/fuzzy"
	"libdb.so/dissent/internal/gtkcord"
)

type index struct {
	items  indexItems
	buffer indexItems
}

const searchLimit = 25

var excludedChannelTypes = []discord.ChannelType{
	discord.GuildCategory,
	discord.GuildForum,
}

var allowedChannelTypes = slices.DeleteFunc(
	slices.Clone(gtkcord.AllowedChannelTypes),
	func(t discord.ChannelType) bool {
		return slices.Contains(excludedChannelTypes, t)
	},
)

func (idx *index) update(ctx context.Context) {
	state := gtkcord.FromContext(ctx).Offline()
	items := make([]indexItem, 0, 250)

	dms, err := state.PrivateChannels()
	if err != nil {
		app.Error(ctx, err)
		return
	}

	for i := range dms {
		items = append(items, newChannelItem(state, nil, &dms[i]))
	}

	guilds, err := state.Guilds()
	if err != nil {
		app.Error(ctx, err)
		return
	}

	for i, guild := range guilds {
		chs, err := state.Channels(guild.ID, allowedChannelTypes)
		if err != nil {
			log.Print("quickswitcher: cannot populate channels for guild ", guild.Name, ": ", err)
			continue
		}
		items = append(items, newGuildItem(&guilds[i]))
		for j := range chs {
			items = append(items, newChannelItem(state, &guilds[i], &chs[j]))
		}
	}

	idx.items = items
}

func (idx *index) search(str string) []indexItem {
	if idx.items == nil {
		return nil
	}

	idx.buffer = idx.buffer[:0]
	if idx.buffer == nil {
		idx.buffer = make([]indexItem, 0, searchLimit)
	}

	matches := fuzzy.FindFrom(str, idx.items)
	for i := 0; i < len(matches) && i < searchLimit; i++ {
		idx.buffer = append(idx.buffer, idx.items[matches[i].Index])
	}

	return idx.buffer
}
