{ self, inputs, ... }:
{
  perSystem =
    {
      self',
      pkgs,
      system,
      ...
    }:
    {
      checks = {
        keep = self'.packages.default;

        manifest = import ./manifest.nix {
          inherit pkgs self;
          inherit (pkgs) lib;
        };

        go = pkgs.runCommand "go-check" { nativeBuildInputs = [ pkgs.go ]; } ''
          cp -r ${../src} src
          chmod -R u+w src
          cd src

          export HOME=$TMPDIR
          export GOCACHE=$TMPDIR/go-cache
          export GOFLAGS=-mod=mod
          export GOPROXY=off
          export CGO_ENABLED=0

          unformatted=$(gofmt -l .)
          if [ -n "$unformatted" ]; then
            echo "not gofmt clean:"
            echo "$unformatted"
            exit 1
          fi

          go vet ./...
          go test ./...

          touch $out
        '';

        formatting = pkgs.runCommand "check-formatting" { nativeBuildInputs = [ pkgs.nixfmt ]; } ''
          nixfmt --check $(find ${self} -name '*.nix')
          touch $out
        '';
      }
      // pkgs.lib.optionalAttrs (system == "x86_64-linux") {
        vm = import ./vm.nix { inherit pkgs inputs self; };
      };
    };
}
