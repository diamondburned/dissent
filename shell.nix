{}:

import ./nix {
	action = "shell";
	buildInputs = pkgs: with pkgs; [
		jq
		niv
		gomod2nix
		imagemagick
		(callPackage ./.github/tools {})
		(writeShellScriptBin "staticcheck" "") # too slow
	];
	usePatchedGo = false;
}
