{
  description = "Persisted state on an ephemeral root";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs:
    let
      inherit (inputs) self;
      modules = import ./modules self;
    in
    inputs.flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      imports = [ ./checks ];

      perSystem =
        { pkgs, ... }:
        {
          packages.default = pkgs.callPackage ./pkgs/keep/package.nix { };

          devShells.default = pkgs.mkShell {
            packages = with pkgs; [
              go
              gopls
              gotools
              nixfmt
            ];
          };

          formatter = pkgs.nixfmt-tree.override {
            runtimeInputs = [ pkgs.go ];
            settings.formatter.gofmt = {
              command = "gofmt";
              options = [ "-w" ];
              includes = [ "*.go" ];
            };
          };
        };

      flake = {
        nixosModules = modules.nixos;
        homeModules = modules.home;

        # the entries option set, for modules that extend a volume
        lib.entryOptions =
          { lib, format }: (import ./modules/options.nix { inherit lib format; }).entryOptions;
      };
    };
}
