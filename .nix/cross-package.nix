{ GOOS, GOARCH, crossSystem, system }:

let goPkgs = import ./pkgs.nix {};

	pkgsSrc = (import ./src.nix).nixpkgs;
	pkgsWith = attrs: import pkgsSrc attrs;

	pkgsCross = pkgsWith {
		overlays = [ (import ./overlay.nix) ];
		crossSystem.config = "aarch64-unknown-linux-gnu";
	};

	pkgsTarget = pkgsWith {
		system = "aarch64-linux";
	};

	base = import ./package-base.nix;
	
	buildGoModule = goPkgs.callPackage "${pkgsSrc}/pkgs/development/go-modules/generic" {
		go = goPkgs.go // { inherit GOOS GOARCH; };
		stdenv = pkgsCross.stdenv;
	};
	
in buildGoModule {
	src = ./..;

	inherit (base) version vendorSha256;

	CGO_ENABLED = "1";

	pname = base.pname + "-${GOOS}-${GOARCH}";

	buildInputs = base.buildInputs pkgsTarget;
	nativeBuildInputs = base.nativeBuildInputs goPkgs;

	subPackages = [ "." ];
}
