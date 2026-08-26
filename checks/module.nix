# enable stays false, so only the manifest attrs are forced, nothing is built
{
  pkgs,
  lib,
  mkModule,
}:
let
  manifestOf =
    format: mock: fixture:
    (lib.evalModules {
      modules = [
        (mkModule format)
        mock
        fixture
        { _module.args.pkgs = pkgs; }
      ];
    }).config.programs.epht.manifest;

  # sort lists so the assertion does not depend on traversal order
  sortStrs = lib.sort (a: b: a < b);
  norm = m: {
    inherit (m) version exclude_globs;
    stores = sortStrs m.stores;
    exclude = sortStrs m.exclude;
    persisted = lib.sort (a: b: a.live < b.live) m.persisted;
  };

  expect =
    name: got: exp:
    if norm got == norm exp then
      true
    else
      throw "epht ${name} manifest mismatch:\n  got:      ${builtins.toJSON (norm got)}\n  expected: ${builtins.toJSON (norm exp)}";

  floor = [
    "/proc"
    "/sys"
    "/dev"
    "/run"
    "/tmp"
    "/var/tmp"
    "/nix"
    "/boot"
  ];

  nixosMock =
    { lib, ... }:
    {
      options.environment.persistence = lib.mkOption {
        type = lib.types.attrs;
        default = { };
      };
      options.users.users = lib.mkOption {
        type = lib.types.attrs;
        default = { };
      };
      options.home-manager = lib.mkOption {
        type = lib.types.attrs;
        default = { };
      };
      # target of the disabled config branch, must exist to receive it
      options.environment.systemPackages = lib.mkOption {
        type = lib.types.listOf lib.types.package;
        default = [ ];
      };
    };

  nixosFixture = {
    programs.epht.extraExcludes = [ "/.swapvol" ]; # system-absolute
    environment.persistence.default = {
      persistentStoragePath = "/persist/system";
      directories = [
        "/var/log"
        { directory = "/etc/ssh"; }
      ];
      files = [ "/var/lib/x" ];
      users.alice = {
        directories = [ ".ssh" ];
        files = [ ];
      };
    };
    users.users.alice.home = "/home/alice";
    home-manager.users.bob = {
      home.homeDirectory = "/home/bob";
      home.persistence."/persist" = {
        persistentStoragePath = "/persist";
        directories = [ ".config/app" ];
        files = [ ];
      };
      # folded home-level exclude, home-relative
      programs.epht.extraExcludes = [ ".cache/junk" ];
    };
  };

  nixosExpected = {
    version = 1;
    exclude_globs = [ ];
    stores = [
      "/persist/system"
      "/persist"
    ];
    exclude = floor ++ [
      "/persist/system"
      "/persist"
      "/.swapvol"
      "/home/bob/.cache/junk"
    ];
    persisted = [
      {
        kind = "dir";
        live = "/var/log";
        store = "/persist/system";
      }
      {
        kind = "dir";
        live = "/etc/ssh";
        store = "/persist/system";
      }
      {
        kind = "file";
        live = "/var/lib/x";
        store = "/persist/system";
      }
      {
        kind = "dir";
        live = "/home/alice/.ssh";
        store = "/persist/system";
      }
      {
        kind = "dir";
        live = "/home/bob/.config/app";
        store = "/persist";
      }
    ];
  };

  hmMock =
    { lib, ... }:
    {
      options.home = lib.mkOption {
        type = lib.types.attrs;
        default = { };
      };
    };

  hmFixture = {
    home.homeDirectory = "/home/diffy";
    home.persistence."/persist" = {
      persistentStoragePath = "/persist";
      directories = [
        ".ssh"
        { directory = ".config/foo"; }
        "/abs/dir"
      ];
      files = [ ".bashrc" ];
    };
    programs.epht.extraExcludes = [ ".cache/thumbs" ]; # home-relative
  };

  hmExpected = {
    version = 1;
    exclude_globs = [ ];
    stores = [ "/persist" ];
    exclude = [
      "/persist"
      "/home/diffy/.cache/thumbs"
    ];
    persisted = [
      {
        kind = "dir";
        live = "/home/diffy/.ssh";
        store = "/persist";
      }
      {
        kind = "dir";
        live = "/home/diffy/.config/foo";
        store = "/persist";
      }
      {
        kind = "dir";
        live = "/abs/dir";
        store = "/persist";
      }
      {
        kind = "file";
        live = "/home/diffy/.bashrc";
        store = "/persist";
      }
    ];
  };
in
assert expect "nixos" (manifestOf "nixos" nixosMock nixosFixture) nixosExpected;
assert expect "home-manager" (manifestOf "home-manager" hmMock hmFixture) hmExpected;
pkgs.runCommand "epht-module-manifest-test" { } "touch $out"
