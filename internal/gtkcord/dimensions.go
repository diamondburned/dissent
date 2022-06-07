package gtkcord

import (
	"fmt"

	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

var _ = cssutil.WriteCSS(`
	.titlebar {
		min-height: {$header_height};
	}
`)

// Constants for dimensions.
const (
	HeaderHeight      = 42
	HeaderPadding     = 16
	GuildIconSize     = 48
	ChannelIconSize   = 32
	MessageAvatarSize = 42
	EmbedMaxWidth     = 250
	EmbedImgHeight    = 300
	InlineEmojiSize   = 22
	LargeEmojiSize    = 48
	StickerSize       = 92
	UserBarAvatarSize = 32
)

func init() {
	cssutil.AddCSSVariables(map[string]string{
		"header_height":       px(HeaderHeight),
		"header_padding":      px(HeaderPadding),
		"guild_icon_size":     px(GuildIconSize),
		"channel_icon_size":   px(ChannelIconSize),
		"message_avatar_size": px(MessageAvatarSize),
		"embed_max_width":     px(EmbedMaxWidth),
		"embed_img_height":    px(EmbedImgHeight),
		"inline_emoji_size":   px(InlineEmojiSize),
		"large_emoji_size":    px(LargeEmojiSize),
	})
}

func px(num int) string {
	return fmt.Sprintf("%dpx", num)
}
