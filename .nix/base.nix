{
	pname = "gtkcord4";
	version = "0.0.1-tip";
	# 0000000000000000000000000000000000000000000000000000000000000000
	vendorSha256 = "1mn7vwmrw5azd6rvg1cgnmar49h8zwk3nwchna733bf4w7jaswly";

	src = ../.;

	buildInputs = buildPkgs: with buildPkgs; [
		# Optional
		sound-theme-freedesktop
		libcanberra-gtk3
		gst_all_1.gstreamer
		gst_all_1.gst-plugins-base
		gst_all_1.gst-plugins-good
		gst_all_1.gst-plugins-bad
		gst_all_1.gst-plugins-ugly
		libadwaita
	];

	files = {
		desktop = {
			name = "com.github.diamondburned.gtkcord4.desktop";
			path = ./com.github.diamondburned.gtkcord4.desktop;
		};
		logo = {
			name = "gtkcord4.png";
			path = ../internal/icons/png/logo.png;
		};
	};
}
