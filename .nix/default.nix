{ action }:

let src = import ./src.nix;
	base' = import ./base.nix;
	systemPkgs = import <nixpkgs> {};

	extraPkgs = {
		shell = pkgs: with pkgs; [
			imagemagick
		];
	};

	base = base' // {
		buildInputs = pkgs:
			(base'.buildInputs pkgs) ++
			((pkgs.lib.attrByPath [action] (pkgs: []) (extraPkgs)) pkgs);
	};

in import "${src.gotk4-nix}/${action}.nix" {
	base = base;
	pkgs = import "${src.gotk4-nix}/pkgs.nix" {
		sourceNixpkgs = systemPkgs.fetchFromGitHub {
			owner = "NixOS";
			repo  = "nixpkgs";
			rev   = "614a842";
			hash  = "sha256:0gkpnjdcrh5s4jx0i8dc6679qfkffmz4m719aarzki4jss4l5n5p";
		};
		useFetched = true;
		overlays = [
			(import ./overlay.nix)
			(import "${src.gotk4-nix}/overlay.nix")
		];
	};
}
