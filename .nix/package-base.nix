{
	pname = "gtkcord4";
	version = "0.0.1-tip";
	# 0000000000000000000000000000000000000000000000000000000000000000
	vendorSha256 = "04jsmqz3m9y5dpqb7xbbxxv66mp7nj5wvl5q1qcxhgi2zyk0hbpx";

	buildInputs = buildPkgs: with buildPkgs; [
		gtk4
		glib
		graphene
		gdk-pixbuf
		gobjectIntrospection
		hicolor-icon-theme

		# Optional
		sound-theme-freedesktop
		libcanberra-gtk3
	];

	nativeBuildInputs = pkgs: with pkgs; [
		pkgconfig
	];
}
