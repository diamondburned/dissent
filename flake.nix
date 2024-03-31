{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
		flake-utils.url = "github:numtide/flake-utils";

		gomod2nix = {
			url = "github:nix-community/gomod2nix";
			inputs.nixpkgs.follows = "nixpkgs";
			inputs.flake-utils.follows = "flake-utils";
		};

		gotk4-nix = {
			url = "git+file:///home/diamond/Scripts/gotk4/gotk4-nix";
			inputs.nixpkgs.follows = "nixpkgs";
			inputs.gomod2nix.follows = "gomod2nix";
			inputs.flake-utils.follows = "flake-utils";
		};
  };

	outputs = { self, ... }@inputs:
		let
			base = import ./nix/base.nix // { src = self; };
		in
		(inputs.flake-utils.lib.eachDefaultSystem (system:
			let
				pkgs = inputs.nixpkgs.legacyPackages.${system}.appendOverlays [
					inputs.gomod2nix.overlays.default
					inputs.gotk4-nix.overlays.patchelf
				];
			in
			{
				devShells.default = inputs.gotk4-nix.lib.mkShell {
					inherit base pkgs;
					buildInputs = with pkgs; [
						jq
						niv
						libxml2 # for xmllint
						python3
						gomod2nix
						imagemagick
						(callPackage ./.github/tools {})
					];
				};

				packages.default = inputs.gotk4-nix.lib.mkPackage {
					inherit base pkgs;
					buildPkgs = pkgs;
					version = self.rev or "unknown";
				};
			}
		)) // {
			lib.mkPackageCross =
				{
					system ? builtins.currentSystem,
					version ? self.rev or "unknown",
					...
				}@args:
				inputs.gotk4-nix.lib.mkPackageCross (args // {
					inherit base;
					pkgs = inputs.nixpkgs.legacyPackages.${system};
				});
		};
}
