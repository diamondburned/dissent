{}:

import ./nix {
	action = "shell";
	buildInputs = pkgs: with pkgs; [
		niv
		gomod2nix
	];
}
