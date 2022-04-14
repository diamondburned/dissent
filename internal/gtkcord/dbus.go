package gtkcord

import "github.com/diamondburned/arikawa/v3/discord"

// IPC commands go here.

// OpenChannelCommand is the data type for a command sent over DBus to open a
// message channel. Its action ID is app.open-channel.
type OpenChannelCommand struct {
	ChannelID discord.ChannelID
	MessageID discord.MessageID // optional, used to highlight message
}
