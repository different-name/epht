{
  description = "Tools for impermanence ephemeral roots";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-25.11";

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

    systems.url = "github:nix-systems/default";
  };

  outputs =
    inputs:
    let
      inherit (inputs) self;
      mkEphtModule = import ./modules self;
    in
    inputs.flake-parts.lib.mkFlake { inherit inputs; } {
      systems = import inputs.systems;

      perSystem =
        { pkgs, ... }:
        {
          packages.default = pkgs.callPackage ./pkgs/epht/package.nix { };

          devShells.default = pkgs.mkShell {
            packages = with pkgs; [
              go
              gopls
              gotools
              nixfmt
            ];
          };

          formatter = pkgs.nixfmt;
        };

      flake = {
        nixosModules = {
          default = self.nixosModules.epht;
          epht = mkEphtModule "nixos";
        };

        homeModules = {
          default = self.homeModules.epht;
          epht = mkEphtModule "home-manager";
        };
      };
    };
}
