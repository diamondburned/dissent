let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "4f498cd56a726dc2ecb19af471cb43bb759708bb";
		hash  = "sha256:0009jbdj2y2vqi522a3r64xf4drp44ghbidf32j6bslswqf3wy4m";
	};
}
