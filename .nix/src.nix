let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "d2bd6577f1867cb740b281baa48a895aed494967";
		hash  = "sha256:02b2h6a6dip2lsw07jm6ch3775gcms6h7hjfll448f7d99ln1b7m";
	};
}
