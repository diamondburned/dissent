{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
		flake-utils.url = "github:numtide/flake-utils";
		flake-compat.url = "https://flakehub.com/f/edolstra/flake-compat/1.tar.gz";

		gomod2nix = {
			url = "github:nix-community/gomod2nix";
			inputs.nixpkgs.follows = "nixpkgs";
			inputs.flake-utils.follows = "flake-utils";
		};

		gotk4-nix = {
			url = "github:diamondburned/gotk4-nix?ref=flake";
			inputs.nixpkgs.follows = "nixpkgs";
			inputs.gomod2nix.follows = "gomod2nix";
			inputs.flake-utils.follows = "flake-utils";
		};
  };

	outputs =
		{ self, ... }@inputs:

		with builtins;
		with inputs.nixpkgs.lib;

		let
			base = import ./nix/base.nix // {
				src = self;
			};
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

				packages = rec {
					default = dissent;
					dissent = inputs.gotk4-nix.lib.mkPackage {
						inherit base pkgs;
						version = self.rev or "unknown";
						overridePackageAttrs = old: {
							meta = {
								description = "Tiny Discord app";
								homepage = "https://libdb.so/dissent";
								platforms = with platforms; linux ++ darwin ++ windows;
								mainProgram = "dissent";
							};
						};
					};
				};

				apps = rec {
					default = dissent;
					dissent = {
						type = "app";
						program = "${self.packages.${system}.dissent}/bin/dissent";
					};
					staticcheck = {
						type = "app";
						program = "${pkgs.go-tools}/bin/staticcheck";
					};
				};

				checks = import ./nix/checks.nix {
					inherit self pkgs;
				};
			}
		)) //
		{
			lib = inputs.gotk4-nix.lib.mkLib {
				inherit base;
				pkgs = inputs.nixpkgs.legacyPackages.${builtins.currentSystem};
			};
		};
}
