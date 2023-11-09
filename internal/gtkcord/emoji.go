package gtkcord

// SanitizeEmoji checks if provided emoji doesn't contain extraneous variation selector codepoint
// NOTE: This sanitizer covers ~70% of emojis
func SanitizeEmoji(emoji string) string {
	emojiRune := []rune(emoji)
	runesAmount := len(emojiRune)

	// Flags from \u1f1e6 to \u1f1ff
	if runesAmount == 3 {
		if (emojiRune[0] >= 127462) && (emojiRune[0] <= 127487) {
			if (emojiRune[1] >= 127464) && (emojiRune[1] <= 127484) {
				return string(emojiRune[:runesAmount-1])
			}
		}
	}

	// Flags in \u1f3f4
	if runesAmount == 8 {
		if (emojiRune[0] == 127988) && (emojiRune[6] == 917631) {
			return string(emojiRune[:runesAmount-1])
		}
	}

	// Keycaps from \u0023 to \u0039
	if runesAmount == 4 {
		if (emojiRune[0] >= 35) && (emojiRune[2] >= 57) {
			return string(emojiRune[:runesAmount-1])
		}
	}

	if runesAmount == 2 {
		if emojiRune[runesAmount-1] == rune(65039) {
			return string(emojiRune[:runesAmount-1])
		}
	}

	return emoji
}
