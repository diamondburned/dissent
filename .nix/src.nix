let systemPkgs = import <nixpkgs> {};

in {
	gotk4 = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4";
		rev   = "4f507c20f8b07f4a87f0152fbefdc9a380042b83";
		hash  = "sha256:0zijivbyjfbb2vda05vpvq268i7vx9bhzlbzzsa4zfzzr9427w66";
	};
	nixpkgs = systemPkgs.fetchFromGitHub {
		owner = "NixOS";
		repo  = "nixpkgs";
		rev   = "0f316e4d72da";
		hash  = "sha256:0vh0fk5is5s9l0lxpi16aabv2kk1fwklr7szy731kfcz9gdrr65l";
	};
}
