{ lib, config }:
let
  hmUsers = if config ? home-manager then config.home-manager.users or { } else { };

  withPath = lib.filterAttrs (_: v: v.store != null);

  systemEntries = lib.concatLists (
    lib.mapAttrsToList (
      name: v:
      map (
        e:
        e
        // {
          volume = name;
          owner = null;
          inherit (v) hideMounts;
        }
      ) v.entries
    ) (withPath config.keep)
  );

  homeEntries = lib.concatLists (
    lib.mapAttrsToList (
      user: ucfg:
      lib.concatLists (
        lib.mapAttrsToList (
          name: v:
          map (
            e:
            e
            // {
              volume = name;
              owner = user;
              live = "${ucfg.home.homeDirectory}/${e.live}";
              inherit (v) hideMounts;
            }
          ) v.entries
        ) (withPath (ucfg.keep or { }))
      )
    ) hmUsers
  );

  unset = lib.filterAttrs (_: v: v.store == null && v.entries != [ ]);
in
{
  entries = map (e: e // { storePath = "${e.store}${e.live}"; }) (systemEntries ++ homeEntries);

  volumes = lib.mapAttrs (_: v: { inherit (v) path; }) (
    lib.foldl' (acc: keep: withPath keep // acc) (withPath config.keep) (
      lib.mapAttrsToList (_: ucfg: ucfg.keep or { }) hmUsers
    )
  );

  unsetVolumes =
    lib.attrNames (unset config.keep)
    ++ lib.concatLists (
      lib.mapAttrsToList (
        user: ucfg: map (name: "${name} for ${user}") (lib.attrNames (unset (ucfg.keep or { })))
      ) hmUsers
    );
}
