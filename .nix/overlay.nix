self: super:

let patchelfer = arch: interpreter: super.writeShellScriptBin
		"patchelf-${arch}"
		"${super.patchelf}/bin/patchelf --set-interpreter ${interpreter} \"$@\"";

	cambalache-pkgs = import (super.fetchFromGitHub {
		owner  = "NixOS";
		repo   = "nixpkgs";
		rev    = "4e42a187c30ed30bacaf46c32300583208fc41d7";
		sha256 = "0vhl10w5bs9lz9wj8gilj8my547lgz76bp3nb4x90ic7lbgjvq1r";
	}) {};
	
in {
	go = super.go.overrideAttrs (old: {
		version = "1.17.6";
		src = builtins.fetchurl {
			url    = "https://go.dev/dl/go1.17.6.src.tar.gz";
			sha256 = "sha256:1j288zwnws3p2iv7r938c89706hmi1nmwd8r5gzw3w31zzrvphad";
		};
		doCheck = false;
		patches = [
			# cmd/go/internal/work: concurrent ccompile routines
			(builtins.fetchurl "https://github.com/diamondburned/go/commit/4e07fa9fe4e905d89c725baed404ae43e03eb08e.patch")
			# cmd/cgo: concurrent file generation
			(builtins.fetchurl "https://github.com/diamondburned/go/commit/432db23601eeb941cf2ae3a539a62e6f7c11ed06.patch")
		];
	});
	buildGoModule = super.buildGoModule.override {
		inherit (self) go;
	};
	gotools = super.gotools; # TODO

	cambalache = cambalache-pkgs.cambalache;

	dominikh = {
		gotools = self.buildGoModule {
			name = "dominikh-go-tools";

			src = super.fetchFromGitHub {
				owner  = "dominikh";
				repo   = "go-tools";
				rev    = "c8caa92bad8c27ae734c6725b8a04932d54a147b";
				sha256 = "1yhbz2sf332b6i00slsj4cn8r66x27kddw5vcjygkkiyny1a99qb";
			};

			vendorSha256 = "09jbarlbq47pcxy5zkja8gqvnqjp2mpbxnciv9lhilw9swqqwc0j";

			doCheck = false;
			subPackages = [ "cmd/staticcheck" ];
		};
	};

	# See https://sourceware.org/glibc/wiki/ABIList.
	patchelf-x86_64  = patchelfer "x86_64"  "/lib64/ld-linux-x86-64.so.2";
	patchelf-aarch64 = patchelfer "aarch64" "/lib/ld-linux-aarch64.so.1";
}
