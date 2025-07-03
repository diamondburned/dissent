package gtkcord

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
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
	EmbedMaxWidth     = 350
	EmbedImgHeight    = 400
	InlineEmojiSize   = 22
	LargeEmojiSize    = 48
	StickerSize       = 92
	UserBarAvatarSize = 32
)

var EmojiAttrs = textutil.Attrs(
	pango.NewAttrSize(32 * pango.SCALE),
)

func init() {
	cssutil.AddCSSVariables(map[string]string{
		"header_height":        px(HeaderHeight),
		"header_padding":       px(HeaderPadding),
		"guild_icon_size":      px(GuildIconSize),
		"channel_icon_size":    px(ChannelIconSize),
		"message_avatar_size":  px(MessageAvatarSize),
		"inline_emoji_size":    px(InlineEmojiSize),
		"large_emoji_size":     px(LargeEmojiSize),
		"sticker_size":         px(StickerSize),
		"user_bar_avatar_size": px(UserBarAvatarSize),
	})
}

func px(num int) string {
	return fmt.Sprintf("%dpx", num)
}
