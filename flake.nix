{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    flake-compat.url = "https://flakehub.com/f/edolstra/flake-compat/1.tar.gz";

    gomod2nix = {
      url = "github:nix-community/gomod2nix/8f3534eb8f6c5c3fce799376dc3b91bae6b11884";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };

    gotk4-nix = {
      url = "github:diamondburned/gotk4-nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.gomod2nix.follows = "gomod2nix";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      gotk4-nix,
      gomod2nix,
      ...
    }:

    with builtins;
    with nixpkgs.lib;

    let
      baseFunc =
        pkgs:
        import ./nix/base.nix {
          inherit pkgs;
          src = self;
        };
    in

    (flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system}.appendOverlays [
          gomod2nix.overlays.default
          gotk4-nix.overlays.patchelf
        ];

        go = pkgs.go_1_24;
      in

      {
        devShells.default = gotk4-nix.lib.mkShell {
          pkgs = pkgs;
          base = baseFunc pkgs;
          buildInputs = with pkgs; [
            jq
            niv
            libxml2 # for xmllint
            python3
            imagemagick
            pkgs.gomod2nix
            (callPackage ./.github/tools { })
          ];
          inherit go;
        };

        packages.default = gotk4-nix.lib.mkPackage {
          pkgs = pkgs;
          base = baseFunc pkgs;
          version = self.rev or "unknown";
          inherit go;
        };

        apps = rec {
          default = dissent;
          dissent = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/dissent";
          };
          staticcheck = {
            type = "app";
            program = "${pkgs.go-tools}/bin/staticcheck";
          };
        };
      }
    ))
    // {
      lib = gotk4-nix.lib.mkLib rec {
        pkgs = nixpkgs.legacyPackages.${builtins.currentSystem};
        base = baseFunc pkgs;
        inherit go;
      };
    };
}
