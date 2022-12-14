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
		overlays = [
			(import ./overlay.nix)
			(import "${src.gotk4-nix}/overlay.nix")
		];
	};
}
