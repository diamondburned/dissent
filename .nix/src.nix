let systemPkgs = import <nixpkgs> {};

in {
	gotk4-nix = ../../gotk4-nix;
	# gotk4-nix = systemPkgs.fetchFromGitHub {
	# 	owner = "diamondburned";
	# 	repo  = "gotk4-nix";
	# 	rev   = "4f407c20f8b07f4a87f0152fbefdc9a380042b83";
	# 	hash  = "sha256:0zij5vbyjfbb2vda05vpvq268i7vx9bhzlbzzsa4zfzzr9427w66";
	# };
}
