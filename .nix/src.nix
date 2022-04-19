let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "c8059c72e890b5128cb54f34eb659325031c7ec4";
		hash  = "sha256:009n4vnjqcvfl147vqk3dgj28s3787v1b7l1p4hn3a1jyy70h9l2";
	};
}
