self: format:
{
  lib,
  config,
  pkgs,
  ...
}:
let
  inherit (lib) types;

  cfg = config.programs.keep;
  isHome = format == "home-manager";

  # pseudo and volatile filesystems, never reported
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

  joinLive = base: p: if lib.hasPrefix "/" p then p else "${base}/${p}";

  homeDir = if isHome then config.home.homeDirectory else "/";

  system = import ./lib.nix { inherit lib config; };

  withPath = lib.filterAttrs (_: v: v.store != null);

  home = {
    entries = lib.concatLists (
      lib.mapAttrsToList (
        name: v:
        map (
          e:
          e
          // {
            volume = name;
            live = "${homeDir}/${e.live}";
          }
        ) v.entries
      ) (withPath config.keep)
    );
    volumes = lib.mapAttrs (_: v: { inherit (v) path; }) (withPath config.keep);
  };

  source = if isHome then home else system;

  hmUsers = if !isHome && config ? home-manager then config.home-manager.users or { } else { };

  exclude = lib.unique (
    lib.optionals (!isHome) floor
    ++ lib.mapAttrsToList (_: v: v.path) source.volumes
    ++ map (joinLive homeDir) cfg.exclude
    ++ lib.concatLists (
      lib.mapAttrsToList (
        _: ucfg: map (joinLive ucfg.home.homeDirectory) (ucfg.programs.keep.exclude or [ ])
      ) hmUsers
    )
  );

  manifest = {
    version = 2;
    inherit (source) volumes;
    entries = map (
      e:
      {
        inherit (e)
          volume
          kind
          live
          store
          ;
      }
      // lib.filterAttrs (_: x: x != null) { inherit (e) user group mode; }
    ) source.entries;
    inherit exclude;
  };

  manifestFile = pkgs.writeText "keep-manifest.json" (builtins.toJSON manifest);

  wrapped = pkgs.symlinkJoin {
    name = "keep-wrapped";
    paths = [ cfg.package ];
    nativeBuildInputs = [ pkgs.makeWrapper ];
    postBuild = ''
      wrapProgram $out/bin/keep --set KEEP_MANIFEST ${cfg.manifestPath}
    '';
  };
in
{
  options.programs.keep = {
    enable = lib.mkEnableOption "the keep command, which reports on persisted state";

    package = lib.mkOption {
      type = types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = lib.literalExpression "keep-nix.packages.\${system}.default";
      description = "The keep package to use";
    };

    exclude = lib.mkOption {
      type = types.listOf types.str;
      default = [ ];
      example = if isHome then [ ".cache/thumbnails" ] else [ "/swap" ];
      description =
        if isHome then
          "Paths left out of the unkept report, relative to the home directory. Under NixOS they are folded into the system-wide report"
        else
          "Absolute paths left out of the unkept report";
    };

    manifestPath = lib.mkOption {
      type = types.path;
      default = manifestFile;
      defaultText = lib.literalMD "a manifest generated from the keep volumes";
      description = "JSON manifest the keep command reads, listing the volumes, their entries and the excluded paths";
    };

    manifest = lib.mkOption {
      type = types.attrs;
      readOnly = true;
      internal = true;
      default = manifest;
      description = "The generated manifest, for inspection and tests";
    };
  };

  config = lib.mkIf cfg.enable (
    if isHome then { home.packages = [ wrapped ]; } else { environment.systemPackages = [ wrapped ]; }
  );
}
