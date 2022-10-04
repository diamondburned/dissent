#!/usr/bin/env bash
set -e

SVGToPNG() {
	convert -background "#00000000" -size "256x256" "${@:3}" "$1" "$2"
}

for svg in svg/*; {
	name="$(basename "${svg%%.*}")"

	# Ensure that the SVG has a custom fill color. Otherwise, just render it
	# without filling.
	if grep 'fill="\(currentColor\|none\)"' "$svg" &> /dev/null; then
		SVGToPNG "$svg" "png/$name-dark.png"  -fill "#DEDEDE" -colorize 255,255,255
		SVGToPNG "$svg" "png/$name-light.png" -fill "#0D0D0D" -colorize 255,255,255
	else
		SVGToPNG "$svg" "png/$name.png"
	fi
}
