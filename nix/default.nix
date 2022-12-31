{ action, ... }@args':

let src = import ./sources.nix;
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

	# src-gotk4-nix = /home/diamond/Scripts/gotk4/gotk4-nix;
	src-gotk4-nix = src.gotk4-nix;

	# Delete the action argument.
	args = builtins.removeAttrs args' [ "action" ];

in import "${src-gotk4-nix}/${action}.nix" (args // {
	base = base;
	pkgs = import "${src-gotk4-nix}/pkgs.nix" {
		sourceNixpkgs = src.nixpkgs;
		useFetched = true;
		overlays = [
			(import ./overlay.nix)
			(import "${src-gotk4-nix}/overlay.nix")
  			(import "${src.gomod2nix}/overlay.nix")
		];
	};
})
