#!/usr/bin/env bash
set -e

declare -A colors
colors=(
	[dark]="#DEDEDE"
	[light]="#0D0D0D"
)

for svg in svg/*; {
	name="$(basename "${svg%%.*}")"
	for color in "${!colors[@]}"; {
		convert \
			-background "#00000000"   \
			-fill "${colors[$color]}" \
			-colorize 255,255,255     \
			-size "256x256"           \
			"$svg" "png/$name-$color.png"
	}
}
