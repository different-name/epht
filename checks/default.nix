{ self, ... }:
{
  perSystem =
    { self', pkgs, ... }:
    {
      checks = {
        epht = self'.packages.default;

        module-manifest = import ./module.nix {
          inherit pkgs;
          inherit (pkgs) lib;
          mkModule = import ../modules self;
        };

        go = pkgs.runCommand "go-check" { nativeBuildInputs = [ pkgs.go ]; } ''
          cp -r ${../src} src
          chmod -R u+w src
          cd src

          export HOME=$TMPDIR
          export GOCACHE=$TMPDIR/go-cache
          export GOFLAGS=-mod=mod
          export GOPROXY=off

          unformatted=$(gofmt -l .)
          if [ -n "$unformatted" ]; then
            echo "not gofmt clean:"
            echo "$unformatted"
            exit 1
          fi

          go vet ./...

          touch $out
        '';

        formatting = pkgs.runCommand "check-formatting" { nativeBuildInputs = [ pkgs.nixfmt ]; } ''
          nixfmt --check $(find ${self} -name '*.nix')
          touch $out
        '';
      };
    };
}
