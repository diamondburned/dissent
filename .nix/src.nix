let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "22608efd555010180920c5b3bc3c5f7fdfe5ef86";
		hash  = "sha256:1bav8f33n0qc288k0j4f83g210fv64vrdk1zlpavw4fqb7d8j4d3";
	};
}
