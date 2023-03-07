{}:

import ./nix {
	action = "shell";
	buildInputs = pkgs: with pkgs; [
		jq
		niv
		gomod2nix
		(callPackage ./.github/tools {})
		(writeShellScriptBin "staticcheck" "") # too slow
	];
}
