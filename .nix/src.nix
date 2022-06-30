let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "ebd3c3e0f5e52ed077c7a56179c4cb569743f9b1";
		hash  = "sha256:1qd8yprfl4nyky4c20h5sg9ndvgmsmcflbnsppikwdikpdxib4vs";
	};
}
