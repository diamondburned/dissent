{
	pname = "gtkcord4";
	version = "0.0.1-tip";
	# 0000000000000000000000000000000000000000000000000000000000000000
	vendorSha256 = "04lcvvxmw0nkc29dmacl4z7phdpvgf0pmyammix2pljxrpc3ji9a";

	src = ../.;

	buildInputs = buildPkgs: with buildPkgs; [
		# Optional
		sound-theme-freedesktop
		libcanberra-gtk3
	];

	files = {
		desktop = ./com.github.diamondburned.gtkcord4.desktop;
		logo = ../internal/icons/png/logo.png;
	};
}
