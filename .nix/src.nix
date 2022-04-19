let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "fd5b734f0914b560c1bfaa0b6bf24458648ef6b3";
		hash  = "sha256:16axmj5wks0dxdvm9xwij1sm5s0y6c4pi329n4zndbw4l24p841h";
	};
}
