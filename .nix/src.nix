let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "b5bb50b31ffd7202decfdb8e84a35cbe88d42c64";
		hash  = "sha256:18wxf24shsra5y5hdbxqcwaf3abhvx1xla8k0vnnkrwz0r9n4iqq";
	};
}
