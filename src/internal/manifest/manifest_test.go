package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseVersion(t *testing.T) {
	if _, err := Parse("t", []byte(`{"version":1,"stores":["/persist"]}`)); err != nil {
		t.Fatalf("v1 should parse: %v", err)
	}
	if _, err := Parse("t", []byte(`{"version":2}`)); err == nil {
		t.Fatal("unknown version must error")
	}
	if _, err := Parse("t", []byte(`{`)); err == nil {
		t.Fatal("malformed json must error")
	}
}

func TestParseNormalises(t *testing.T) {
	m, err := Parse("t", []byte(`{
		"version":1,
		"stores":["/persist/"],
		"persisted":[{"kind":"dir","live":"/var/log/","store":"/persist/"}],
		"exclude":["/nix/"]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Stores[0] != "/persist" {
		t.Errorf("store not cleaned: %q", m.Stores[0])
	}
	if m.Persisted[0].Live != "/var/log" {
		t.Errorf("live not cleaned: %q", m.Persisted[0].Live)
	}
	if m.Exclude[0] != "/nix" {
		t.Errorf("exclude not cleaned: %q", m.Exclude[0])
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "m.json")
	if err := os.WriteFile(p, []byte(`{"version":1,"stores":["/persist"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := Load(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("missing file must error")
	}
}

func TestUnion(t *testing.T) {
	a := &Manifest{
		Version:   Version,
		Stores:    []string{"/persist"},
		Persisted: []Entry{{Kind: KindDir, Live: "/var/log", Store: "/persist"}},
		Exclude:   []string{"/nix"},
	}
	b := &Manifest{
		Version:   Version,
		Stores:    []string{"/state"},
		Persisted: []Entry{{Kind: KindFile, Live: "/etc/machine-id", Store: "/state"}},
		Exclude:   []string{"/tmp"},
	}
	u := Union(a, nil, b)
	if len(u.Stores) != 2 || len(u.Persisted) != 2 || len(u.Exclude) != 2 {
		t.Fatalf("union sizes: stores=%d persisted=%d exclude=%d", len(u.Stores), len(u.Persisted), len(u.Exclude))
	}
	mt := NewMatcher(u)
	if !mt.Covered("/var/log/x") || !mt.Covered("/etc/machine-id") {
		t.Error("unioned manifest should cover entries from both sources")
	}
	if !mt.UnderStore("/state/foo") {
		t.Error("unioned manifest should know both stores")
	}
}

func TestMapping(t *testing.T) {
	if got := Backing("/persist", "/var/log"); got != "/persist/var/log" {
		t.Errorf("Backing = %q", got)
	}
	if got := Backing("/persist", "/home/diffy/.ssh"); got != "/persist/home/diffy/.ssh" {
		t.Errorf("Backing user store = %q", got)
	}

	cases := []struct {
		store, backing, wantLive string
		wantOK                   bool
	}{
		{"/persist", "/persist/var/log", "/var/log", true},
		{"/persist", "/persist", "/", true},
		{"/persist", "/state/var/log", "", false},
		{"/", "/var/log", "/var/log", true},
		{"/persist", "/persistent/x", "", false}, // component-aware
	}
	for _, c := range cases {
		live, ok := LiveFromBacking(c.store, c.backing)
		if ok != c.wantOK || live != c.wantLive {
			t.Errorf("LiveFromBacking(%q,%q) = (%q,%v), want (%q,%v)",
				c.store, c.backing, live, ok, c.wantLive, c.wantOK)
		}
	}

	for _, live := range []string{"/var/log", "/home/diffy/.ssh", "/etc/machine-id"} {
		b := Backing("/persist", live)
		got, ok := LiveFromBacking("/persist", b)
		if !ok || got != live {
			t.Errorf("round trip %q -> %q -> (%q,%v)", live, b, got, ok)
		}
	}
}
