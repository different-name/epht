self: format:
{
  lib,
  config,
  pkgs,
  ...
}:
let
  inherit (lib) types;

  cfg = config.programs.epht;

  isNixos =
    if format == "nixos" then
      true
    else if format == "home-manager" then
      false
    else
      throw "unexpected `format`, must be one of: nixos, home-manager";

  # os pseudo and volatile filesystems, never reported live
  floorExcludes = [
    "/proc"
    "/sys"
    "/dev"
    "/run"
    "/tmp"
    "/var/tmp"
    "/nix"
    "/boot"
  ];

  # an entry is a bare string or an impermanence submodule
  entryPath =
    key: e:
    if builtins.isString e then e else (e.${key} or (throw "epht: persistence entry missing `${key}`"));

  joinLive = base: p: if lib.hasPrefix "/" p then p else "${base}/${p}";

  entriesFor =
    base: storePath: persistence:
    (map (e: {
      kind = "dir";
      live = joinLive base (entryPath "directory" e);
      store = storePath;
    }) persistence.directories)
    ++ (map (e: {
      kind = "file";
      live = joinLive base (entryPath "file" e);
      store = storePath;
    }) persistence.files);

  enabledStores = lib.filterAttrs (_: s: s.enable or true);

  nixosStores = enabledStores (config.environment.persistence or { });

  nixosEntries = lib.concatLists (
    lib.mapAttrsToList (
      _: store:
      let
        sp = store.persistentStoragePath;
      in
      entriesFor "/" sp store
      ++ lib.concatLists (
        lib.mapAttrsToList (user: ucfg: entriesFor (ucfg.home or config.users.users.${user}.home) sp ucfg) (
          store.users or { }
        )
      )
    ) nixosStores
  );

  hmUsers =
    if (isNixos && cfg.includeHomeManagerUsers or false && config ? home-manager) then
      config.home-manager.users
    else
      { };

  hmFoldEntries = lib.concatLists (
    lib.mapAttrsToList (
      _: ucfg:
      lib.concatLists (
        lib.mapAttrsToList (
          _: store: entriesFor ucfg.home.homeDirectory store.persistentStoragePath store
        ) (enabledStores (ucfg.home.persistence or { }))
      )
    ) hmUsers
  );

  hmFoldStores = lib.concatLists (
    lib.mapAttrsToList (
      _: ucfg:
      lib.mapAttrsToList (_: s: s.persistentStoragePath) (enabledStores (ucfg.home.persistence or { }))
    ) hmUsers
  );

  homeStores = enabledStores (config.home.persistence or { });

  homeEntries = lib.concatLists (
    lib.mapAttrsToList (
      _: store: entriesFor config.home.homeDirectory store.persistentStoragePath store
    ) homeStores
  );

  entries = if isNixos then nixosEntries ++ hmFoldEntries else homeEntries;

  storeRoots = lib.unique (
    if isNixos then
      (lib.mapAttrsToList (_: s: s.persistentStoragePath) nixosStores) ++ hmFoldStores
    else
      (lib.mapAttrsToList (_: s: s.persistentStoragePath) homeStores)
  );

  # `or [ ]` so home excludes work without the home-manager module enabled
  hmFoldExcludes = lib.concatLists (
    lib.mapAttrsToList (
      _: ucfg: map (joinLive ucfg.home.homeDirectory) (ucfg.programs.epht.extraExcludes or [ ])
    ) hmUsers
  );

  userExcludes =
    if isNixos then
      cfg.extraExcludes ++ hmFoldExcludes
    else
      map (joinLive config.home.homeDirectory) cfg.extraExcludes;

  # nixos owns the system floor, a per-user home manifest only knows its stores
  baseExcludes = (lib.optionals isNixos floorExcludes) ++ storeRoots ++ userExcludes;

  manifest = {
    version = 1;
    stores = storeRoots;
    persisted = entries;
    exclude = lib.unique baseExcludes;
    exclude_globs = [ ];
  };

  manifestFile = pkgs.writeText "epht-manifest.json" (builtins.toJSON manifest);

  wrapped = pkgs.symlinkJoin {
    name = "epht-wrapped";
    paths = [ cfg.package ];
    nativeBuildInputs = [ pkgs.makeWrapper ];
    postBuild = ''
      wrapProgram $out/bin/epht --set EPHT_MANIFEST ${cfg.manifestPath}
    '';
  };
in
{
  options.programs.epht = {
    enable = lib.mkEnableOption "epht";

    package = lib.mkOption {
      type = types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = lib.literalExpression "epht.packages.\${system}.default";
      description = "The epht package to use.";
    };

    extraExcludes = lib.mkOption {
      type = types.listOf types.str;
      default = [ ];
      example = if isNixos then [ "/.swapvol" ] else [ ".cache/thumbnails" ];
      description =
        if isNixos then
          "Extra absolute paths to ignore when reporting unpersisted data."
        else
          ''
            Extra paths to ignore when reporting unpersisted data. Relative entries
            resolve against your home directory. When Home Manager runs as a NixOS
            module they apply to the system-wide epht too.
          '';
    };

    manifestPath = lib.mkOption {
      type = types.path;
      default = manifestFile;
      defaultText = lib.literalMD "a manifest generated from your persistence config";
      description = ''
        Path to the JSON manifest epht reads, listing your persisted paths and
        storage roots. Override to supply your own.
      '';
    };

    manifest = lib.mkOption {
      type = types.attrs;
      readOnly = true;
      internal = true;
      default = manifest;
      description = "The generated manifest attrs, exposed for inspection and tests.";
    };
  }
  // lib.optionalAttrs isNixos {
    includeHomeManagerUsers = lib.mkOption {
      type = types.bool;
      default = true;
      description = ''
        Include each Home Manager user's `home.persistence`, so one system-wide
        epht run covers every user's storage as well as the system's.
      '';
    };
  };

  config = lib.mkIf cfg.enable (
    if isNixos then { environment.systemPackages = [ wrapped ]; } else { home.packages = [ wrapped ]; }
  );
}
