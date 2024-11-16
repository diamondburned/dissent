{ pkgs, src ? ../. }:

{
	inherit src;

	pname = "dissent";
	# 0000000000000000000000000000000000000000000000000000000000000000
	# vendorSha256 = "10ijsv73bfgrsmvzirwv0nanyicxy6a6nayimif9dfvi9m5a7521";
	modules = ./gomod2nix.toml;

	buildInputs = pkgs: with pkgs; [
		# Required

		gst_all_1.gstreamer
		gst_all_1.gst-plugins-base
		libadwaita
		libspelling
		gtksourceview5

		# Optional

		gst_all_1.gst-plugins-good
		gst_all_1.gst-plugins-bad
		gst_all_1.gst-plugins-ugly
	];

	files = {
		desktop = {
			name = "so.libdb.dissent.desktop";
			path = ./so.libdb.dissent.desktop;
		};
		service = {
			name = "so.libdb.dissent.service";
			path = ./so.libdb.dissent.service;
		};
		icons = {
			path = pkgs.linkFarm "dissent-icons" (map
				(path: {
					name = baseNameOf path;
					inherit path;
				})
				[
					../internal/icons/hicolor
					../internal/icons/scalable
					../internal/icons/symbolic
				]
			);
		};
	};
}
