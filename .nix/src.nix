let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "f930bfbcc537b27722fe7419b9750c83d917d41a";
		hash  = "sha256:0rmgdgp87y4xi92wx67m6pyb8wh9f8bg6a4xm9x4zmasfrdidg4x";
	};
}
