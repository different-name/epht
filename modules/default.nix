self: {
  nixos = rec {
    keep = import ./keep.nix "nixos";
    backend = ./backend.nix;
    cli = import ./cli.nix self "nixos";
    default.imports = [
      keep
      backend
      cli
    ];
  };

  home = rec {
    keep = import ./keep.nix "home-manager";
    cli = import ./cli.nix self "home-manager";
    default.imports = [
      keep
      cli
    ];
  };
}
