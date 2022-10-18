let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "2c031f93638f8c97a298807df80424f68ffaac76";
		hash  = "sha256:0lpbnbzl1sc684ypf6ba5f8jnj6sd8z8ajs0pa2sqi8j9w0c87b0";
	};
}
