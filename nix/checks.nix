{ self, pkgs }:

let
	inherit (self.packages.${pkgs.system})
		dissent;

	inherit (pkgs)
		lib;
in

{
	default = pkgs.nixosTest {
		name = "dissent-check-default";
		nodes.machine = { config, pkgs, lib, ... }: {
			system.stateVersion = "23.10";

			boot.loader.systemd-boot.enable = true;
			boot.loader.efi.canTouchEfiVariables = true;

			services.xserver.displayManager.autoLogin = {
				enable = true;
				user = "alice";
			};

			users.users.alice = {
				isNormalUser = true;
				extraGroups = [ "wheel" ];
			};

			services.cage = {
				enable = true;
				user = "alice";
				program = "${lib.getExe dissent}";
			};
		};
		testScript = { nodes, ... }: (builtins.replaceStrings ["\t"] ["  "] ''
			machine.wait_until_succeeds("ps aux | grep -v ps | grep dissent")
			machine.sleep(1)
			machine.screenshot("screen")
		'');
	};
}
