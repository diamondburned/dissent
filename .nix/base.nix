{
	pname = "gtkcord4";
	version = "0.0.1-tip";
	# 0000000000000000000000000000000000000000000000000000000000000000
	vendorSha256 = "JZpf33zrv3pVVdhz0enEadFKPCrawz5vzDkBbr/5jek=";

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
