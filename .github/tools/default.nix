{ pkgs ? import <nixpkgs> {} }:

with pkgs;
with lib;
with builtins;

let scripts = filterSource
	(path: type: !hasSuffix (baseNameOf path) ".nix")
	./.;

in runCommand "github-tools" {
	src = scripts;
} ''
	mkdir -p $out/bin
	cp -r $src/* $out/bin
''
