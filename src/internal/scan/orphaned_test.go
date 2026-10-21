package scan

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/different-name/keep-nix/internal/manifest"
)

func write(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
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
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func buildStore(t *testing.T) (string, *manifest.Matcher) {
	t.Helper()
	store := t.TempDir()
	write(t, store,
		"var/log/old.log",
		"var/logging/app.log",
		"etc/machine-id",
		"etc/other.conf",
		"old/cache/x",
	)
	m := &manifest.Manifest{
		Version: manifest.Version,
		Volumes: map[string]manifest.Volume{"persist": {Path: store}},
		Entries: []manifest.Entry{
			{Volume: "persist", Kind: manifest.KindDir, Live: "/var/log", Store: store},
			{Volume: "persist", Kind: manifest.KindFile, Live: "/etc/machine-id", Store: store},
		},
	}
	return store, manifest.NewMatcher(m)
}

func TestOrphanedCollapse(t *testing.T) {
	store, mt := buildStore(t)
	a := func(p string) string { return filepath.Join(store, p) }

	res, err := orphaned(mt, Options{Collapse: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("etc/other.conf"),
		a("old"),
		a("var/logging"),
	})
	for _, r := range res {
		wantDir := r.Path != a("etc/other.conf")
		if (r.Kind == manifest.KindDir) != wantDir {
			t.Errorf("%q kind = %v", r.Path, r.Kind)
		}
		if r.Volume != "persist" {
			t.Errorf("%q volume = %q", r.Path, r.Volume)
		}
	}
}

func TestOrphanedFiles(t *testing.T) {
	store, mt := buildStore(t)
	a := func(p string) string { return filepath.Join(store, p) }

	res, err := orphaned(mt, Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("etc/other.conf"),
		a("old/cache/x"),
		a("var/logging/app.log"),
	})
}

// an empty dir at a nested entry's location is its mount stub, not data
func TestOrphanedAcrossStores(t *testing.T) {
	persist := t.TempDir()
	scratch := t.TempDir()
	write(t, persist,
		"home/a/.local/share/Steam/config",
		"home/a/.local/share/Steam/steamapps/common/game/bin",
	)
	if err := os.MkdirAll(filepath.Join(persist, "home/a/.local/share/Steam/steamapps/shadercache"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, scratch, "home/a/.local/share/Steam/steamapps/common/game/bin")

	steam := "/home/a/.local/share/Steam"
	mt := manifest.NewMatcher(&manifest.Manifest{
		Version: manifest.Version,
		Volumes: map[string]manifest.Volume{"persist": {Path: persist}, "scratch": {Path: scratch}},
		Entries: []manifest.Entry{
			{Volume: "persist", Kind: manifest.KindDir, Live: steam, Store: persist},
			{Volume: "scratch", Kind: manifest.KindDir, Live: steam + "/steamapps/common", Store: scratch},
			{Volume: "scratch", Kind: manifest.KindDir, Live: steam + "/steamapps/shadercache", Store: scratch},
		},
	})

	res, err := orphaned(mt, Options{Collapse: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{filepath.Join(persist, "home/a/.local/share/Steam/steamapps/common")})

	// once common is mounted the leftover copy under it is hidden and must be skipped
	points := map[string]bool{filepath.Join(persist, "home/a/.local/share/Steam/steamapps/common"): true}
	res, err = orphaned(mt, Options{Collapse: true}, points)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), nil)
}
