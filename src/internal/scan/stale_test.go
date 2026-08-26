package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/different-name/epht/internal/manifest"
)

func buildStore(t *testing.T) (string, *manifest.Matcher) {
	t.Helper()
	store := t.TempDir()
	for _, f := range []string{
		"var/log/old.log",
		"var/logging/app.log",
		"etc/machine-id",
		"etc/other.conf",
		"old/cache/x",
	} {
		p := filepath.Join(store, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := &manifest.Manifest{
		Version: manifest.Version,
		Stores:  []string{store},
		Persisted: []manifest.Entry{
			{Kind: manifest.KindDir, Live: "/var/log", Store: store},
			{Kind: manifest.KindFile, Live: "/etc/machine-id", Store: store},
		},
	}
	return store, manifest.NewMatcher(m)
}

func TestStaleCollapse(t *testing.T) {
	store, mt := buildStore(t)
	a := func(p string) string { return filepath.Join(store, p) }

	res, err := Stale(mt, Options{Collapse: true})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("etc/other.conf"), // orphan file beside a persisted sibling
		a("old"),            // whole orphan subtree rolled up
		a("var/logging"),    // component-aware: not covered by /var/log
	})
	for _, r := range res {
		wantDir := r.Path != a("etc/other.conf")
		if (r.Kind == manifest.KindDir) != wantDir {
			t.Errorf("%q kind = %v", r.Path, r.Kind)
		}
	}
}

func TestStaleDefaultFiles(t *testing.T) {
	store, mt := buildStore(t)
	a := func(p string) string { return filepath.Join(store, p) }

	res, err := Stale(mt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		a("etc/other.conf"),
		a("old/cache/x"),
		a("var/logging/app.log"),
	})
	for _, r := range res {
		if r.Kind != manifest.KindFile {
			t.Errorf("default view should emit files, got dir %q", r.Path)
		}
	}
}

func TestStaleMissingStoreSkipped(t *testing.T) {
	m := &manifest.Manifest{
		Version: manifest.Version,
		Stores:  []string{filepath.Join(t.TempDir(), "does-not-exist")},
	}
	res, err := Stale(manifest.NewMatcher(m), Options{})
	if err != nil {
		t.Fatalf("missing store should be skipped, got %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected no results, got %v", res)
	}
}

func TestStaleNestedStore(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "persist")
	system := filepath.Join(home, "system")
	for _, f := range []string{
		"home/diffy/nixxy/f",
		"home/diffy/junk",
		"system/var/log/l",
		"system/var/junk",
	} {
		p := filepath.Join(home, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := &manifest.Manifest{
		Version: manifest.Version,
		Stores:  []string{home, system, home},
		Persisted: []manifest.Entry{
			{Kind: manifest.KindDir, Live: "/home/diffy/nixxy", Store: home},
			{Kind: manifest.KindDir, Live: "/var/log", Store: system},
		},
	}

	res, err := Stale(manifest.NewMatcher(m), Options{})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, paths(res), []string{
		filepath.Join(home, "home/diffy/junk"),
		filepath.Join(system, "var/junk"),
	})
}
