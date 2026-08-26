{
  lib,
  buildGoModule,
}:
buildGoModule {
  pname = "epht";
  version = "0.1.0";

  src = builtins.path {
    path = ../../src;
    name = "epht-src";
  };

  # stdlib only, no module dependencies
  vendorHash = null;

  ldflags = [
    "-s"
    "-w"
  ];

  meta = {
    description = "Tools for impermanence ephemeral roots";
    homepage = "https://github.com/different-name/epht";
    license = lib.licenses.gpl3Plus;
    mainProgram = "epht";
    platforms = lib.platforms.linux;
  };
}
