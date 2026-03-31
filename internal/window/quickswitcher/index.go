package quickswitcher

import (
	"context"
	"log/slog"
	"slices"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotkit/app"
	"github.com/sahilm/fuzzy"
	"libdb.so/dissent/internal/gtkcord"
)

type index struct {
	channelItems  channelIndexItems
	guildItems    guildIndexItems
	channelBuffer channelIndexItems
	guildBuffer   guildIndexItems
}

const channelsSearchLimit = 15
const guildSearchLimit = 10

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
	guildItems := make([]guildIndexItem, 0, 250)
	channelItems := make([]channelIndexItem, 0, 250)

	dms, err := state.PrivateChannels()
	if err != nil {
		app.Error(ctx, err)
		return
	}

	for i := range dms {
		channelItems = append(channelItems, newChannelItem(state, nil, &dms[i]))
	}

	guilds, err := state.Guilds()
	if err != nil {
		app.Error(ctx, err)
		return
	}

	for i, guild := range guilds {
		chs, err := state.Channels(guild.ID, allowedChannelTypes)
		if err != nil {
			slog.Error(
				"cannot populate channels for guild in quick switcher",
				"guild", guild.Name,
				"guild_id", guild.ID,
				"err", err)
			continue
		}

		guildItems = append(guildItems, newGuildItem(&guilds[i]))
		for j := range chs {
			channelItems = append(channelItems, newChannelItem(state, &guilds[i], &chs[j]))
		}
	}

	idx.guildItems = guildItems
	idx.channelItems = channelItems
}

func (idx *index) search(str string) ([]channelIndexItem, []guildIndexItem) {

	if idx.channelItems != nil {
		idx.channelBuffer = idx.channelBuffer[:0]
		if idx.channelBuffer == nil {
			idx.channelBuffer = make([]channelIndexItem, 0, channelsSearchLimit)
		}
		channelMatches := fuzzy.FindFrom(str, idx.channelItems)
		for i := 0; i < len(channelMatches) && i < channelsSearchLimit; i++ {
			idx.channelBuffer = append(idx.channelBuffer, idx.channelItems[channelMatches[i].Index])
		}
	} else {
		idx.channelBuffer = nil
	}

	if idx.guildItems != nil {
		idx.guildBuffer = idx.guildBuffer[:0]
		if idx.guildBuffer == nil {
			idx.guildBuffer = make([]guildIndexItem, 0, channelsSearchLimit)
		}
		guildMatches := fuzzy.FindFrom(str, idx.guildItems)
		for i := 0; i < len(guildMatches) && i < guildSearchLimit; i++ {
			idx.guildBuffer = append(idx.guildBuffer, idx.guildItems[guildMatches[i].Index])
		}
	} else {
		idx.guildBuffer = nil
	}

	return idx.channelBuffer, idx.guildBuffer
}
