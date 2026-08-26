package scan

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/different-name/epht/internal/manifest"
)

func buildTree(t *testing.T) (string, *manifest.Matcher) {
	t.Helper()
	root := t.TempDir()

	files := []string{
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
	}
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	abs := func(p string) string { return filepath.Join(root, p) }
	m := &manifest.Manifest{
		Version: manifest.Version,
		Stores:  []string{abs("persist")},
		Persisted: []manifest.Entry{
			{Kind: manifest.KindDir, Live: abs("persisted_dir"), Store: abs("persist")},
			{Kind: manifest.KindDir, Live: abs("partial/keep"), Store: abs("persist")},
			{Kind: manifest.KindDir, Live: abs("var/log"), Store: abs("persist")},
			{Kind: manifest.KindFile, Live: abs("machine-id"), Store: abs("persist")},
		},
		Exclude: []string{abs("excluded")},
	}
	return root, manifest.NewMatcher(m)
}

func paths(rs []Result) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Path
	}
	sort.Strings(out)
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("got %d results %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("result %d: got %q, want %q\nfull got %v\nwant %v", i, got[i], want[i], got, want)
		}
	}
}

func TestVolatileCollapse(t *testing.T) {
	root, mt := buildTree(t)
	a := func(p string) string { return filepath.Join(root, p) }

	res, err := Volatile([]string{root}, mt, Options{Collapse: true})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("fullynew"),          // whole subtree volatile -> rolled up
		a("partial/loose.txt"), // loose file beside a persisted sibling
		a("partial/newdir"),    // whole sub-subtree volatile -> rolled up
		a("var/logging"),       // component-aware: not covered by var/log
	})

	for _, r := range res {
		wantDir := r.Path != a("partial/loose.txt")
		if (r.Kind == manifest.KindDir) != wantDir {
			t.Errorf("%q kind = %v", r.Path, r.Kind)
		}
	}
}

func TestVolatileDefaultFiles(t *testing.T) {
	root, mt := buildTree(t)
	a := func(p string) string { return filepath.Join(root, p) }

	res, err := Volatile([]string{root}, mt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("fullynew/m/n.txt"),
		a("partial/loose.txt"),
		a("partial/newdir/deep/y"),
		a("var/logging/app.log"),
	})
	for _, r := range res {
		if r.Kind != manifest.KindFile {
			t.Errorf("default view should emit files, got dir %q", r.Path)
		}
	}
}

func TestVolatileIgnoresSymlinks(t *testing.T) {
	root := t.TempDir()
	a := func(p string) string { return filepath.Join(root, p) }

	mkfile := func(p string) {
		full := a(p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mklink := func(p string) {
		full := a(p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("/nix/store/whatever", full); err != nil {
			t.Fatal(err)
		}
	}

	mkfile("keep/x")        // persisted dir, makes root partial
	mkfile("data/real.txt") // boundary with real data -> reported dir
	mkfile("loose.txt")     // loose regular file -> reported
	mklink("loose-link")    // loose symlink -> skipped
	mklink("onlylinks/l1")  // boundary of only symlinks -> not reported
	mklink("onlylinks/sub/l2")

	m := &manifest.Manifest{
		Version:   manifest.Version,
		Persisted: []manifest.Entry{{Kind: manifest.KindDir, Live: a("keep"), Store: "/persist"}},
	}
	mt := manifest.NewMatcher(m)

	res, err := Volatile([]string{root}, mt, Options{Collapse: true})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("data"),      // holds a regular file
		a("loose.txt"), // regular file
	})

	full, err := Volatile([]string{root}, mt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(full), []string{
		a("data/real.txt"),
		a("loose.txt"),
	})
}

func TestVolatileSubRoot(t *testing.T) {
	root, mt := buildTree(t)
	a := func(p string) string { return filepath.Join(root, p) }

	res, err := Volatile([]string{a("partial")}, mt, Options{Collapse: true})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("partial/loose.txt"),
		a("partial/newdir"),
	})
}

func TestVolatileMissingRoot(t *testing.T) {
	root, mt := buildTree(t)
	if _, err := Volatile([]string{filepath.Join(root, "nope")}, mt, Options{}); err == nil {
		t.Fatal("missing root should error")
	}
}

func TestResolveRoots(t *testing.T) {
	r, err := ResolveRoots(nil)
	if err != nil || len(r) != 1 || r[0] != "/" {
		t.Fatalf("default roots = %v, %v", r, err)
	}
	r, err = ResolveRoots([]string{"/a", "b/../c"})
	if err != nil {
		t.Fatal(err)
	}
	if r[0] != "/a" {
		t.Errorf("abs arg = %q", r[0])
	}
	if !filepath.IsAbs(r[1]) || filepath.Base(r[1]) != "c" {
		t.Errorf("relative arg not resolved: %q", r[1])
	}
}
