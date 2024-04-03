#!/usr/bin/env nix-shell
#! nix-shell -i bash -p inkscape imagemagick

tmp=$(mktemp -d)

inkscape \
	-w 256 -h 256 \
	-o $tmp/logo.png \
	./hicolor/scalable/apps/so.libdb.dissent.svg

convert \
	-define icon:auto-resize=256,128,96,64,48,32,16 $tmp/logo.png \
	-colors 256 $tmp/logo.ico

mv $tmp/logo.ico ./windows/dissent.ico

rm -r $tmp
