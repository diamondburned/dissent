#!/usr/bin/env bash
set -e

# Files to upload.
files=(
	artifacts/gotktrix-nixos-x86_64
	artifacts/gotktrix-linux-x86_64
	artifacts/gotktrix-nixos-aarch64
	artifacts/gotktrix-linux-aarch64
)

main() {
	[[ $GITHUB_REF =~ refs/tags/(.*) ]] \
		&& tag=${BASH_REMATCH[1]} \
		|| exit
	
	release=$(ghcurl https://api.github.com/repos/diamondburned/gotktrix/releases/tags/"$tag")
	releaseID=$(jq '.id' <<< "$release")
	
	for file in "${files[@]}"; {
		name=$(basename "$file")
		ghcurl \
			-X POST \
			-H "Content-Type: application/x-executable" \
			--data-binary "@$file" \
			"https://uploads.github.com/repos/diamondburned/gotktrix/releases/$releaseID/assets?name=$name" > /dev/null
	}
}

ghcurl() {
	[[ ! -f ~/.github-token ]] && {
		echo "Missing token file." >&2
		return 1
	}

	curl \
		-H "Authorization: token $(< ~/.github-token)" \
		-H "Accept: application/vnd.github.v3+json" \
		-s \
		"$@"
}

main "$@"
