{
  pkgs,
  inputs,
  self,
}:
pkgs.testers.runNixOSTest {
  name = "keep";

  nodes.machine = {
    imports = [
      self.nixosModules.default
      inputs.home-manager.nixosModules.default
    ];

    virtualisation = {
      # tmpfs root, so only the two volume disks survive the restart
      diskImage = null;
      emptyDiskImages = [
        256
        256
      ];
      # with no root disk image the empty disks come first
      fileSystems."/persist" = {
        device = "/dev/vda";
        fsType = "ext4";
        autoFormat = true;
        neededForBoot = true;
      };
      fileSystems."/scratch" = {
        device = "/dev/vdb";
        fsType = "ext4";
        autoFormat = true;
        neededForBoot = true;
      };
    };

    boot.initrd.systemd.enable = true;

    users.users.alice.isNormalUser = true;

    programs.keep.enable = true;

    keep.persist = {
      path = "/persist";
      dirs."/var/lib/nixos" = { };
      dirs."/var/lib/thing" = {
        user = "alice";
        mode = "0700";
      };
      files."/var/lib/seed" = { };
      files."/var/lib/empty" = { };
    };

    keep.scratch.path = "/scratch";

    home-manager.users.alice = {
      imports = [ self.homeModules.keep ];
      home.stateVersion = "26.05";
      keep.persist = {
        dirs."Documents" = { };
        files.".token" = { };
        xdg.config.dirs."app" = { };
        xdg.data.dirs."Steam" = { };
      };
      keep.scratch.xdg.data.dirs."Steam/steamapps/common" = { };
    };

    system.stateVersion = "26.05";
  };

  testScript = ''
    def check_mounted(path, store):
        machine.succeed(f"findmnt -no SOURCE {path} | grep -qF '[{store}]'")

    machine.wait_for_unit("multi-user.target")

    with subtest("boot critical paths mount in the initrd"):
        machine.succeed("journalctl -b | grep -q 'Mounted /sysroot/var/lib/nixos'")

    check_mounted("/var/lib/nixos", "/system/var/lib/nixos")
    check_mounted("/var/lib/thing", "/system/var/lib/thing")
    check_mounted("/home/alice/Documents", "/home/alice/Documents")
    check_mounted("/home/alice/.config/app", "/home/alice/.config/app")
    check_mounted("/home/alice/.local/share/Steam", "/home/alice/.local/share/Steam")
    check_mounted("/home/alice/.local/share/Steam/steamapps/common", "/home/alice/.local/share/Steam/steamapps/common")

    machine.succeed("stat -c %U:%a /var/lib/thing | grep -qx alice:700")
    machine.succeed("stat -c %U:%a /persist/system/var/lib/thing | grep -qx alice:700")
    machine.succeed("stat -c %U:%a /home/alice/.config | grep -qx alice:755")
    machine.succeed("stat -c %U:%a /persist/home/alice | grep -qx alice:700")
    machine.succeed("stat -c %U:%a /persist/home/alice/.config/app | grep -qx alice:755")

    with subtest("a nested entry's mount point in the outer store copy belongs to the user"):
        machine.succeed("stat -c %U:%a /persist/home/alice/.local/share/Steam/steamapps | grep -qx alice:755")
        machine.succeed("stat -c %U:%a /persist/home/alice/.local/share/Steam/steamapps/common | grep -qx alice:755")
        machine.succeed("su alice -c 'touch ~/.local/share/Steam/steamapps/libraryfolders.vdf'")
        machine.succeed("test -f /persist/home/alice/.local/share/Steam/steamapps/libraryfolders.vdf")

    # a file with no store copy is linked so the first write lands in the store
    machine.succeed("test -L /var/lib/seed && test -L /home/alice/.token")

    with subtest("an empty file left at the live path is replaced by the link"):
        machine.succeed("rm /var/lib/empty && touch /var/lib/empty")
        machine.succeed("systemctl restart keep-var-lib-empty.service")
        machine.succeed("test -L /var/lib/empty")

    with subtest("check passes on a fresh boot"):
        machine.succeed("keep check | grep -qx '9 entries ok'")

    with subtest("list declared groups by volume"):
        out = machine.succeed("keep list declared")
        assert "persist (/persist)" in out and "scratch (/scratch)" in out, out
        assert "/home/alice/.config/app" in out, out

    machine.succeed("echo seed > /var/lib/seed && test -f /persist/system/var/lib/seed")
    machine.succeed("su alice -c 'echo token > ~/.token && echo state > ~/.config/app/state'")
    machine.succeed("stat -c %U /persist/home/alice/.token | grep -qx alice")

    with subtest("check accepts a file written through its link"):
        machine.succeed("keep check | grep -qx '9 entries ok'")

    with subtest("unkept sees live data outside the entries"):
        machine.succeed("mkdir -p /var/lib/loose && echo x > /var/lib/loose/data")
        out = machine.succeed("keep list unkept /var/lib || true")
        assert "/var/lib/loose\n" in out, out
        out = machine.succeed("keep list -f unkept /var/lib || true")
        assert "/var/lib/loose/data\n" in out, out
        machine.fail("keep list unkept /var/lib | grep -q /var/lib/thing")

    with subtest("orphaned sees store data no entry claims, in either volume"):
        machine.succeed("mkdir -p /persist/system/var/lib/old && echo x > /persist/system/var/lib/old/data")
        machine.succeed("echo x > /scratch/home/alice/.local/share/Steam/steamapps/stray")
        out = machine.succeed("keep list orphaned --json || true")
        assert '"path": "/persist/system/var/lib/old"' in out and '"kind": "dir"' in out, out
        assert '"path": "/scratch/home/alice/.local/share/Steam/steamapps/stray"' in out and '"volume": "scratch"' in out, out
        out = machine.succeed("keep list -f orphaned || true")
        assert "/persist/system/var/lib/old/data\n" in out, out
        machine.succeed("rm -r /persist/system/var/lib/old /scratch/home/alice/.local/share/Steam/steamapps/stray")

    uid = machine.succeed("id -u alice").strip()

    machine.shutdown()
    machine.start()
    machine.wait_for_unit("multi-user.target")

    assert machine.succeed("id -u alice").strip() == uid

    check_mounted("/var/lib/seed", "/system/var/lib/seed")
    check_mounted("/home/alice/.token", "/home/alice/.token")
    machine.succeed("test ! -L /var/lib/seed && grep -qx seed /var/lib/seed")
    machine.succeed("grep -qx token /home/alice/.token")
    machine.succeed("grep -qx state /home/alice/.config/app/state")
    machine.succeed("stat -c %U /home/alice/.config/app/state | grep -qx alice")
    machine.succeed("test -f /home/alice/.local/share/Steam/steamapps/libraryfolders.vdf")

    with subtest("check passes after the files became mounts"):
        machine.succeed("keep check | grep -qx '9 entries ok'")

    with subtest("check reports a missing mount"):
        machine.succeed("umount /home/alice/Documents")
        out = machine.fail("keep check")
        assert "/home/alice/Documents: not mounted" in out, out
  '';
}
