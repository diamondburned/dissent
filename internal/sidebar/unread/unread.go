// Package unread contains state structures to synchronize the guild and channel
// views.
package unread

import "github.com/diamondburned/arikawa/v3/discord"

// GuildState is the unread state for a guild.
type GuildState struct {
	chs map[discord.ChannelID]struct{}
}
