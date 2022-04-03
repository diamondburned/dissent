{}:

let src = import ./src.nix;

	goPkgs = import ./pkgs.nix { useFetched = true; };
	pkgs   = import ./pkgs.nix {};
	lib    = pkgs.lib;

	shell = import "${src.gotk4}/.nix/shell.nix" {
		inherit pkgs;
	};

in shell.overrideAttrs (old: {
	buildInputs = with pkgs; [
		gtk4
		gtk4.debug
		glib
		glib.debug
		graphene
		gdk-pixbuf
		gobjectIntrospection
		libcanberra-gtk3

		pkgconfig

		# Always use patched Go, since it's much faster.
		goPkgs.go
		goPkgs.gopls
		goPkgs.gotools
		goPkgs.dominikh.gotools

		imagemagick
		cambalache

		patchelf-x86_64
		patchelf-aarch64
	];

	# Workaround for the lack of wrapGAppsHook:
	# https://nixos.wiki/wiki/Development_environment_with_nix-shell
	shellHook = with pkgs; with pkgs.gnome; ''
		XDG_DATA_DIRS=$XDG_DATA_DIRS:${hicolor-icon-theme}/share:${adwaita-icon-theme}/share
		XDG_DATA_DIRS=$XDG_DATA_DIRS:$GSETTINGS_SCHEMAS_PATH
	'';

	NIX_DEBUG_INFO_DIRS = ''${pkgs.gtk4.debug}/lib/debug:${pkgs.glib.debug}/lib/debug'';

	CGO_ENABLED  = "1";
	CGO_CFLAGS   = "-g2 -O2";
	CGO_CXXFLAGS = "-g2 -O2";
	CGO_FFLAGS   = "-g2 -O2";
	CGO_LDFLAGS  = "-g2 -O2";
})
