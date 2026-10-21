{
  lib,
  buildGoModule,
}:
buildGoModule {
  pname = "keep";
  version = "0.1.0";

  src = builtins.path {
    path = ../../src;
    name = "keep-src";
  };

  subPackages = [ "cmd/keep" ];

  # stdlib only, no module dependencies
  vendorHash = null;

  # os/user resolves through nss with cgo, the pure resolver reads /etc directly
  env.CGO_ENABLED = 0;

  ldflags = [
    "-s"
    "-w"
  ];

  meta = {
    description = "Persisted state on an ephemeral root";
    homepage = "https://github.com/different-name/keep-nix";
    license = lib.licenses.gpl3Plus;
    mainProgram = "keep";
    platforms = lib.platforms.linux;
  };
}
