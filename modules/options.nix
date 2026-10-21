{ lib, format }:
let
  inherit (lib) types;

  isHome = format == "home-manager";
  relativeTo = if isHome then "the home directory" else "the root";

  permission =
    description:
    lib.mkOption {
      type = types.nullOr types.str;
      default = null;
      inherit description;
    };

  dir = types.submodule {
    options = {
      user = permission "Owner of the directory when it is created, the user for home entries and root for system entries";
      group = permission "Group of the directory when it is created";
      mode = permission "Mode of the directory when it is created, 0755 unless set";
    };
  };

  file = types.submodule { };

  entries = relative: {
    dirs = lib.mkOption {
      type = types.attrsOf dir;
      default = { };
      description = "Directories to persist, keyed by path relative to ${relative}";
    };

    files = lib.mkOption {
      type = types.attrsOf file;
      default = { };
      description = "Files to persist, keyed by path relative to ${relative}";
    };
  };

  xdgHomes = [
    # keep-sorted start
    "cache"
    "config"
    "data"
    "state"
    # keep-sorted end
  ];
in
{
  inherit xdgHomes;

  entryOptions =
    entries relativeTo
    // lib.optionalAttrs isHome {
      xdg = lib.mkOption {
        type = types.submodule {
          options = lib.genAttrs xdgHomes (
            home:
            lib.mkOption {
              type = types.submodule { options = entries "the xdg ${home} home"; };
              default = { };
              description = "Entries under the xdg ${home} home";
            }
          );
        };
        default = { };
        description = "Entries under the xdg base directories, resolved from the xdg options";
      };
    };
}
