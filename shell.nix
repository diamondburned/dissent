{}:

import ./nix {
	action = "shell";
	buildInputs = pkgs: with pkgs; [
		niv
		gomod2nix
		(callPackage ./.github/tools {})
	];
}
