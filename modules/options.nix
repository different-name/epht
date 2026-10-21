{ lib, format }:
let
  inherit (lib) types;

  isHome = format == "home-manager";

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

  keyed =
    entry: types.coercedTo (types.listOf types.str) (l: lib.genAttrs l (_: { })) (types.attrsOf entry);

  entries =
    {
      dirs,
      files,
      relative,
      example ? null,
    }:
    {
      ${dirs} = lib.mkOption (
        {
          type = keyed dir;
          default = { };
          description = "Directories to persist, relative to ${relative}. Either a list of paths, or an attribute set keyed by path to set ownership and mode";
        }
        // lib.optionalAttrs (example != null) { example = example.dirs; }
      );

      ${files} = lib.mkOption (
        {
          type = keyed file;
          default = { };
          description = "Files to persist, relative to ${relative}. Either a list of paths, or an attribute set keyed by path";
        }
        // lib.optionalAttrs (example != null) { example = example.files; }
      );
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
    if isHome then
      entries {
        dirs = "dirs";
        files = "files";
        relative = "the home directory";
        example = {
          dirs = [ "Documents" ];
          files = [ ".bash_history" ];
        };
      }
      // lib.foldl' (
        acc: home:
        acc
        // entries {
          dirs = "${home}Dirs";
          files = "${home}Files";
          relative = "the xdg ${home} home";
        }
      ) { } xdgHomes
    else
      entries {
        dirs = "dirs";
        files = "files";
        relative = "the root";
        example = {
          dirs = [ "/var/lib/nixos" ];
          files = [ "/etc/machine-id" ];
        };
      };
}
