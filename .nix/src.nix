let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "17e14620e28322567f50664f1fa8ae7fee79d6fc";
		hash  = "sha256:1mznaxchd3g9nq7scncfxa7cln44bkwr8sbkqbhb9lq06mf9cyjw";
	};
}
