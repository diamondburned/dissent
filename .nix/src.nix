let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "c78a1f0b188eee16ddd31f5b174d2ca0ffa282f0";
		hash  = "sha256:1viisf81sspy1na5d26ybpppdpcvcnfvh2l0p2pdpcjx4mq9j52s";
	};
}
