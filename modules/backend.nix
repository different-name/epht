{
  lib,
  config,
  pkgs,
  utils,
  ...
}:
let
  inherit (import ./lib.nix { inherit lib config; }) entries unsetVolumes;

  users = config.users.users;

  inHome = home: path: path == home || lib.hasPrefix "${home}/" path;

  # owner of a path created on the way to an entry
  ownerOf =
    path:
    let
      owner = lib.findFirst (u: inHome u.home path) null (lib.attrValues users);
    in
    if owner == null then
      {
        user = "root";
        group = "root";
        mode = "0755";
      }
    else
      {
        user = owner.name;
        inherit (owner) group;
        mode = if path == owner.home then "0700" else "0755";
      };

  withDefaults =
    e:
    let
      defaults =
        if e.owner == null then
          {
            user = "root";
            group = "root";
          }
        else
          {
            user = e.owner;
            inherit (users.${e.owner}) group;
          };
    in
    e
    // {
      user = if e.user != null then e.user else defaults.user;
      group = if e.group != null then e.group else defaults.group;
      mode = if e.mode != null then e.mode else "0755";
    };

  resolved = map withDefaults entries;
  dirs = lib.filter (e: e.kind == "dir") resolved;
  files = lib.filter (e: e.kind == "file") resolved;
  storeRoots = lib.unique (map (e: e.store) resolved);

  mkMount = prefix: e: {
    what = "${prefix}${e.storePath}";
    where = "${prefix}${e.live}";
    type = "none";
    options = lib.concatStringsSep "," ([ "bind" ] ++ lib.optional e.hideMounts "x-gvfs-hide");
    unitConfig.DefaultDependencies = false;
  };

  initrdDirs = lib.filter (e: lib.elem e.live utils.pathsNeededForBoot) dirs;

  parentsOf =
    path:
    let
      parts = lib.filter (p: p != "") (lib.splitString "/" path);
    in
    lib.genList (n: "/" + lib.concatStringsSep "/" (lib.take (n + 1) parts)) (lib.length parts - 1);

  # the dir entry whose subtree holds a path, excluding one entry
  coveringDir =
    path: self:
    lib.foldl' (
      best: d:
      if
        d != self
        && (d.live == path || lib.hasPrefix "${d.live}/" path)
        && (best == null || lib.stringLength d.live > lib.stringLength best.live)
      then
        d
      else
        best
    ) null dirs;

  dirSpecs =
    let
      parent = path: { inherit path; } // ownerOf path;
      storeParent = e: path: { path = "${e.store}${path}"; } // ownerOf path;
      leaf = e: {
        inherit (e) user group mode;
        path = e.live;
      };
      storeLeaf = e: {
        inherit (e) user group mode;
        path = e.storePath;
      };
      # a path nested inside another volume's entry also needs its mount point in that store copy
      outerCopies =
        e:
        lib.concatMap (
          path:
          let
            outer = coveringDir path e;
          in
          lib.optional (outer != null && outer.store != e.store) (
            { path = "${outer.store}${path}"; } // ownerOf path
          )
        ) (parentsOf e.live ++ [ e.live ]);
      roots = lib.concatMap (root: map parent (parentsOf root ++ [ root ])) storeRoots;
      perEntry =
        e:
        map parent (parentsOf e.live)
        ++ map (storeParent e) (parentsOf e.live)
        ++ outerCopies e
        ++ lib.optionals (e.kind == "dir") [
          (storeLeaf e)
          (leaf e)
        ];
      all = roots ++ lib.concatMap perEntry resolved;
      byPath = lib.foldl' (acc: d: if acc ? ${d.path} then acc else acc // { ${d.path} = d; }) { } all;
      depth = d: lib.length (lib.splitString "/" d.path);
    in
    lib.sort (a: b: depth a < depth b) (lib.attrValues byPath);

  mkdirScript = pkgs.writeShellScript "keep-dirs" ''
    status=0
    keep_dir() {
      if [[ -d $1 ]]; then
        return
      fi
      if [[ -e $1 || -L $1 ]]; then
        echo "keep: $1 exists and is not a directory" >&2
        status=1
        return
      fi
      mkdir -m "$4" "$1" && chown "$2:$3" "$1" || status=1
    }
    ${lib.concatMapStrings (
      d:
      "keep_dir ${
        lib.escapeShellArgs [
          d.path
          d.user
          d.group
          d.mode
        ]
      }\n"
    ) dirSpecs}
    exit $status
  '';

  # bind the store copy when it exists, else link so the first write lands in the store
  fileScript = pkgs.writeShellScript "keep-file" ''
    store=$1
    live=$2
    if [[ -L $live && $(readlink -f "$live") == "$store" ]]; then
      exit 0
    fi
    if ${pkgs.util-linux}/bin/findmnt "$live" > /dev/null; then
      exit 0
    fi
    if [[ -s $live ]]; then
      echo "keep: $live exists and is not empty" >&2
      exit 1
    fi
    if [[ -e $store ]]; then
      touch "$live"
      ${pkgs.util-linux}/bin/mount -o bind "$store" "$live"
    else
      # an empty file at the live path is a stale mount point
      rm -f "$live"
      ln -s "$store" "$live"
    fi
  '';

  unfileScript = pkgs.writeShellScript "keep-unfile" ''
    live=$1
    if [[ -L $live ]]; then
      rm "$live"
    else
      ${pkgs.util-linux}/bin/umount "$live"
      rm "$live"
    fi
  '';

  filesScript = pkgs.writeShellScript "keep-files" ''
    status=0
    ${lib.concatMapStrings (
      f:
      "${fileScript} ${
        lib.escapeShellArgs [
          f.storePath
          f.live
        ]
      } || status=1\n"
    ) files}
    exit $status
  '';

  fileServices = lib.listToAttrs (
    map (
      f:
      lib.nameValuePair "keep-${utils.escapeSystemdPath f.live}" {
        description = "Persist ${f.live}";
        wantedBy = [ "local-fs.target" ];
        before = [ "local-fs.target" ];
        unitConfig.DefaultDependencies = false;
        serviceConfig = {
          Type = "oneshot";
          RemainAfterExit = true;
          ExecStart = "${fileScript} ${
            lib.escapeShellArgs [
              f.storePath
              f.live
            ]
          }";
          ExecStop = "${unfileScript} ${lib.escapeShellArg f.live}";
        };
      }
    ) files
  );

  fileSystemOf =
    path:
    lib.foldl' (
      best: fs:
      if
        (fs.mountPoint == "/" || inHome fs.mountPoint path)
        && (best == null || lib.stringLength fs.mountPoint > lib.stringLength best.mountPoint)
      then
        fs
      else
        best
    ) null (lib.attrValues config.fileSystems);

  neededForBoot =
    path:
    let
      fs = fileSystemOf path;
    in
    fs == null || utils.fsNeededForBoot fs;

  duplicates =
    let
      lives = map (e: e.live) resolved;
    in
    lib.unique (lib.filter (p: lib.count (q: q == p) lives > 1) lives);

  # the unit name is derived from the path by nixpkgs, which escapes % itself, so the body cannot match it
  percent = lib.filter (p: lib.hasInfix "%" p) (map (e: e.storePath) resolved);

  unpinned =
    lib.attrNames (lib.filterAttrs (_: u: u.uid == null) users)
    ++ lib.attrNames (lib.filterAttrs (_: g: g.gid == null) config.users.groups);
in
{
  config = lib.mkIf (resolved != [ ] || unsetVolumes != [ ]) {
    assertions = [
      {
        assertion = unsetVolumes == [ ];
        message = "keep: these volumes have entries but no path: ${lib.concatStringsSep ", " unsetVolumes}";
      }
      {
        assertion = lib.all neededForBoot storeRoots;
        message = "keep: every filesystem holding a volume must be neededForBoot: ${
          lib.concatStringsSep ", " (lib.filter (r: !neededForBoot r) storeRoots)
        }";
      }
      {
        assertion = duplicates == [ ];
        message = "keep: paths declared in more than one volume: ${lib.concatStringsSep ", " duplicates}";
      }
      {
        assertion = percent == [ ];
        message = "keep: systemd units cannot carry a path with %: ${lib.concatStringsSep ", " percent}";
      }
    ];

    warnings =
      lib.optional (!(lib.any (e: e.live == "/var/lib/nixos") dirs) && unpinned != [ ])
        "keep: /var/lib/nixos is not persisted and these users and groups have no fixed id, so ownership of persisted files will change between boots: ${lib.concatStringsSep ", " unpinned}";

    systemd.mounts = map (
      e:
      mkMount "" e
      // {
        wantedBy = [ "local-fs.target" ];
        before = [ "local-fs.target" ];
      }
    ) dirs;

    boot.initrd.systemd.mounts = map (
      e:
      mkMount "/sysroot" e
      // {
        wantedBy = [ "initrd.target" ];
        before = [ "initrd-nixos-activation.service" ];
      }
    ) initrdDirs;

    systemd.services = fileServices;

    system.activationScripts = {
      keep-dirs = {
        deps = [
          "users"
          "groups"
        ];
        text = "${mkdirScript}";
      };
      keep-files = {
        deps = [ "keep-dirs" ];
        text = "${filesScript}";
      };
    };
  };
}
