format:
{
  lib,
  config,
  osConfig ? null,
  ...
}:
let
  inherit (lib) types;
  inherit (import ./options.nix { inherit lib format; }) entryOptions xdgHomes;

  isHome = format == "home-manager";
  cfg = config.keep;

  storeRoot = path: if isHome then path else "${path}/system";

  volume =
    { name, config, ... }:
    {
      options = entryOptions // {
        path =
          lib.mkOption {
            type = types.nullOr types.str;
            default = if isHome && osConfig != null then osConfig.keep.${name}.path or null else null;
            description = "Root of the volume. System paths persist under its system directory, home paths under the user's home directory inside it";
            example = "/persist";
          }
          // lib.optionalAttrs isHome {
            defaultText = lib.literalExpression "osConfig.keep.<name>.path";
          };

        hideMounts = lib.mkOption {
          type = types.bool;
          default = true;
          description = "Whether to hide the volume's mounts from file managers";
        };

        store = lib.mkOption {
          type = types.nullOr types.str;
          internal = true;
          readOnly = true;
          description = "Root the volume's entries are stored under, null while the path is unset";
        };

        entries = lib.mkOption {
          type = types.listOf types.attrs;
          internal = true;
          readOnly = true;
          description = "Entries of the volume with xdg keys resolved, each with its kind, path and store root";
        };
      };

      config = {
        store = if config.path == null then null else storeRoot config.path;

        entries =
          lib.mapAttrsToList (live: d: {
            kind = "dir";
            inherit live;
            inherit (config) store;
            inherit (d) user group mode;
          }) (resolvedDirs config)
          ++ map (live: {
            kind = "file";
            inherit live;
            inherit (config) store;
            user = null;
            group = null;
            mode = null;
          }) (lib.attrNames (resolvedFiles config));
      };
    };

  homeDir = config.home.homeDirectory;
  xdgHome = home: config.xdg."${home}Home";
  xdgRelative = home: lib.removePrefix "${homeDir}/" (xdgHome home);

  resolveXdg =
    kind: v:
    lib.optionalAttrs isHome (
      lib.mergeAttrsList (
        map (
          home: lib.mapAttrs' (key: x: lib.nameValuePair "${xdgRelative home}/${key}" x) v."${home}${kind}"
        ) xdgHomes
      )
    );

  resolvedDirs = v: v.dirs // resolveXdg "Dirs" v;
  resolvedFiles = v: v.files // resolveXdg "Files" v;

  allKeys = lib.concatMap (
    v: lib.concatMap (name: lib.attrNames v.${name}) (lib.attrNames entryOptions)
  ) (lib.attrValues cfg);
in
{
  options.keep = lib.mkOption {
    type = types.attrsOf (types.submodule volume);
    default = { };
    description = "Volumes of persisted state, each a directory tree that survives the ephemeral root";
  };

  config.assertions =
    lib.mapAttrsToList (name: v: {
      assertion = v.path == null || lib.hasPrefix "/" v.path;
      message = "keep.${name}.path must be absolute, got ${toString v.path}";
    }) cfg
    ++ map (key: {
      assertion = !(lib.elem ".." (lib.splitString "/" key));
      message = "keep: path ${key} must not contain ..";
    }) allKeys
    ++ map (key: {
      assertion = lib.hasPrefix "/" key == !isHome;
      message = "keep: path ${key} must be ${
        if isHome then "relative to the home directory" else "absolute"
      }";
    }) allKeys
    ++ lib.optionals isHome (
      map (home: {
        assertion = lib.hasPrefix "${homeDir}/" (xdgHome home);
        message = "keep: xdg ${home} home ${xdgHome home} is outside ${homeDir}";
      }) xdgHomes
    );
}
