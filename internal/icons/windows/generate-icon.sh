#!/usr/bin/env nix-shell
#! nix-shell -i bash -p inkscape imagemagick

svgInput="./hicolor/scalable/apps/so.libdb.dissent.svg"
icoOutput="./windows/dissent.ico"

svgHash=$(sha256sum "$svgInput" | cut -d' ' -f1)
svgHashFile="./windows/so.libdb.dissent.svg.sha256"

if [[ ! -f $svgHashFile || $(< $svgHashFile) != $svgHash ]]; then
	tmp=$(mktemp -d)

	inkscape \
		-w 256 -h 256 \
		-o $tmp/logo.png \
		"$svgInput"
	
	convert \
		-define icon:auto-resize=256,128,96,64,48,32,16 $tmp/logo.png \
		-colors 256 $tmp/logo.ico
	
	mv $tmp/logo.ico "$icoOutput"
	rm -r $tmp

	echo $svgHash > $svgHashFile
fi
