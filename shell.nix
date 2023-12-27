{}:

import ./nix {
	action = "shell";
	buildInputs = pkgs: with pkgs; [
		jq
		niv
		libxml2 # for xmllint
		gomod2nix
		imagemagick
		(callPackage ./.github/tools {})
		(writeShellScriptBin "staticcheck" "") # too slow
	];
	usePatchedGo = false;
}
