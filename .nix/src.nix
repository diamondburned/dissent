let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "38d4836aaadf1d897fbde17114ae712e9c2e0c59";
		hash  = "sha256:06vfb09zjvlik4c4kc85q8fpz7sysc1gwkpdkskkxskmf8s8ij7n";
	};
}
