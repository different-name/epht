# enable stays false, so only the manifest attrs are forced, nothing is built
{
  pkgs,
  lib,
  self,
}:
let
  modules = import ../modules self;

  manifestOf =
    format: mock: fixture:
    (lib.evalModules {
      modules = [
        modules.${format}.keep
        modules.${format}.cli
        mock
        fixture
        { _module.args.pkgs = pkgs; }
      ];
    }).config.programs.keep.manifest;

  sortStrs = lib.sort (a: b: a < b);
  norm = m: {
    inherit (m) version volumes;
    exclude = sortStrs m.exclude;
    entries = lib.sort (a: b: a.live < b.live) m.entries;
  };

  expect =
    name: got: exp:
    if norm got == norm exp then
      true
    else
      throw "keep ${name} manifest mismatch:\n  got:      ${builtins.toJSON (norm got)}\n  expected: ${builtins.toJSON (norm exp)}";

  floor = [
    "/boot"
    "/dev"
    "/nix"
    "/proc"
    "/run"
    "/sys"
    "/tmp"
    "/var/tmp"
  ];

  attrs = lib.mkOption {
    type = lib.types.attrs;
    default = { };
  };

  nixosMock = {
    options = {
      assertions = lib.mkOption { type = lib.types.listOf lib.types.attrs; };
      users.users = attrs;
      home-manager = attrs;
      environment.systemPackages = lib.mkOption {
        type = lib.types.listOf lib.types.package;
        default = [ ];
      };
    };
  };

  nixosFixture = {
    programs.keep.extraExcludes = [ "/swap" ];
    keep.persist = {
      path = "/persist";
      dirs."/var/log" = { };
      dirs."/var/lib/thing".user = "alice";
      files."/var/lib/seed" = { };
    };
    keep.scratch = {
      path = "/scratch";
      dirs."/var/cache" = { };
    };
    users.users.bob.home = "/home/bob";
    home-manager.users.bob = {
      home.homeDirectory = "/home/bob";
      keep.persist = {
        store = "/persist";
        entries = [
          {
            kind = "dir";
            live = ".config/app";
            store = "/persist";
            user = null;
            group = null;
            mode = null;
          }
        ];
        hideMounts = true;
      };
      keep.scratch = {
        store = "/scratch";
        entries = [
          {
            kind = "dir";
            live = ".config/app/cache";
            store = "/scratch";
            user = null;
            group = null;
            mode = "0700";
          }
        ];
        hideMounts = true;
      };
      programs.keep.extraExcludes = [ ".cache/junk" ];
    };
  };

  nixosExpected = {
    version = 2;
    volumes = {
      persist.path = "/persist";
      scratch.path = "/scratch";
    };
    exclude = floor ++ [
      "/persist"
      "/scratch"
      "/swap"
      "/home/bob/.cache/junk"
    ];
    entries = [
      {
        volume = "persist";
        kind = "dir";
        live = "/var/log";
        store = "/persist/system";
      }
      {
        volume = "persist";
        kind = "dir";
        live = "/var/lib/thing";
        store = "/persist/system";
        user = "alice";
      }
      {
        volume = "persist";
        kind = "file";
        live = "/var/lib/seed";
        store = "/persist/system";
      }
      {
        volume = "scratch";
        kind = "dir";
        live = "/var/cache";
        store = "/scratch/system";
      }
      {
        volume = "persist";
        kind = "dir";
        live = "/home/bob/.config/app";
        store = "/persist";
      }
      {
        volume = "scratch";
        kind = "dir";
        live = "/home/bob/.config/app/cache";
        store = "/scratch";
        mode = "0700";
      }
    ];
  };

  hmMock = {
    options = {
      assertions = lib.mkOption { type = lib.types.listOf lib.types.attrs; };
      home = attrs;
      xdg = attrs;
    };
  };

  hmFixture = {
    home.homeDirectory = "/home/diffy";
    home.packages = [ ];
    xdg = {
      cacheHome = "/home/diffy/.cache";
      configHome = "/home/diffy/.config";
      dataHome = "/home/diffy/.local/share";
      stateHome = "/home/diffy/.local/state";
    };
    keep.persist = {
      path = "/persist";
      dirs.".ssh".mode = "0700";
      files.".bashrc" = { };
      xdg.data.dirs."Steam" = { };
      xdg.config.files."app/token" = { };
    };
    keep.scratch = {
      path = "/scratch";
      xdg.data.dirs."Steam/steamapps/common" = { };
    };
    programs.keep.extraExcludes = [ ".cache/thumbs" ];
  };

  hmExpected = {
    version = 2;
    volumes = {
      persist.path = "/persist";
      scratch.path = "/scratch";
    };
    exclude = [
      "/persist"
      "/scratch"
      "/home/diffy/.cache/thumbs"
    ];
    entries = [
      {
        volume = "persist";
        kind = "dir";
        live = "/home/diffy/.ssh";
        store = "/persist";
        mode = "0700";
      }
      {
        volume = "persist";
        kind = "file";
        live = "/home/diffy/.bashrc";
        store = "/persist";
      }
      {
        volume = "persist";
        kind = "dir";
        live = "/home/diffy/.local/share/Steam";
        store = "/persist";
      }
      {
        volume = "persist";
        kind = "file";
        live = "/home/diffy/.config/app/token";
        store = "/persist";
      }
      {
        volume = "scratch";
        kind = "dir";
        live = "/home/diffy/.local/share/Steam/steamapps/common";
        store = "/scratch";
      }
    ];
  };
in
assert expect "nixos" (manifestOf "nixos" nixosMock nixosFixture) nixosExpected;
assert expect "home-manager" (manifestOf "home" hmMock hmFixture) hmExpected;
pkgs.runCommand "keep-manifest-test" { } "touch $out"
