{ action }:

let src = import ./src.nix;

in import "${src.gotk4-nix}/${action}.nix" {
	base = import ./base.nix;
}
