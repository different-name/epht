package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/different-name/keep-nix/internal/manifest"
)

func buildTree(t *testing.T) (string, *manifest.Matcher) {
	t.Helper()
	root := t.TempDir()
	write(t, root,
		"persisted_dir/a.txt",
		"machine-id",
		"excluded/junk",
		"persist/var/log/old.log",
		"fullynew/m/n.txt",
		"partial/keep/x",
		"partial/loose.txt",
		"partial/newdir/deep/y",
		"var/log/sys.log",
		"var/logging/app.log",
	)
	if err := os.Symlink("/nowhere", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	abs := func(p string) string { return filepath.Join(root, p) }
	m := &manifest.Manifest{
		Version: manifest.Version,
		Volumes: map[string]manifest.Volume{"persist": {Path: abs("persist")}},
		Entries: []manifest.Entry{
			{Volume: "persist", Kind: manifest.KindDir, Live: abs("persisted_dir"), Store: abs("persist")},
			{Volume: "persist", Kind: manifest.KindFile, Live: abs("machine-id"), Store: abs("persist")},
			{Volume: "persist", Kind: manifest.KindDir, Live: abs("partial/keep"), Store: abs("persist")},
			{Volume: "persist", Kind: manifest.KindDir, Live: abs("var/log"), Store: abs("persist")},
		},
		Exclude: []string{abs("excluded")},
	}
	return root, manifest.NewMatcher(m)
}

func TestUnkeptFiles(t *testing.T) {
	root, mt := buildTree(t)
	a := func(p string) string { return filepath.Join(root, p) }

	res, err := Unkept([]string{root}, mt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("fullynew/m/n.txt"),
		a("partial/loose.txt"),
		a("partial/newdir/deep/y"),
		a("var/logging/app.log"),
	})
}

func TestUnkeptCollapse(t *testing.T) {
	root, mt := buildTree(t)
	a := func(p string) string { return filepath.Join(root, p) }

	res, err := Unkept([]string{root}, mt, Options{Collapse: true})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("fullynew"),
		a("partial/loose.txt"),
		a("partial/newdir"),
		a("var/logging"),
	})
}

func TestUnkeptSubtree(t *testing.T) {
	root, mt := buildTree(t)
	a := func(p string) string { return filepath.Join(root, p) }

	res, err := Unkept([]string{a("partial")}, mt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("partial/loose.txt"),
		a("partial/newdir/deep/y"),
	})
}
