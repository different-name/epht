package manifest

import (
	"strings"
	"testing"
)

func TestParseNormalizes(t *testing.T) {
	m, err := Parse("test", []byte(`{
		"version": 2,
		"volumes": {"persist": {"path": "/persist/"}},
		"entries": [
			{"volume": "persist", "kind": "dir", "live": "/var/log/", "store": "/persist/system/"},
			{"volume": "persist", "kind": "file", "live": "/home/a/.token", "store": "/persist", "user": "a"}
		],
		"exclude": ["/tmp/"]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Volumes["persist"].Path; got != "/persist" {
		t.Errorf("volume path = %q", got)
	}
	if got := m.Entries[0].Live; got != "/var/log" {
		t.Errorf("live = %q", got)
	}
	if got := m.Entries[0].Store; got != "/persist/system" {
		t.Errorf("store = %q", got)
	}
	if got := m.Entries[0].StorePath(); got != "/persist/system/var/log" {
		t.Errorf("store path = %q", got)
	}
	if got := m.Entries[1].User; got != "a" {
		t.Errorf("user = %q", got)
	}
	if got := m.Exclude[0]; got != "/tmp" {
		t.Errorf("exclude = %q", got)
	}
}

func TestParseRejectsOtherVersions(t *testing.T) {
	_, err := Parse("old", []byte(`{"version": 1, "stores": [], "persisted": [], "exclude": []}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported version 1") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnionKeepsFirstVolume(t *testing.T) {
	a := &Manifest{Version: Version, Volumes: map[string]Volume{"persist": {Path: "/persist"}}, Entries: []Entry{{Kind: KindDir, Live: "/a", Store: "/persist/system"}}}
	b := &Manifest{Version: Version, Volumes: map[string]Volume{"persist": {Path: "/other"}}, Entries: []Entry{{Kind: KindDir, Live: "/b", Store: "/persist"}}}
	u := Union(a, nil, b)
	if u.Volumes["persist"].Path != "/persist" {
		t.Errorf("volume = %q", u.Volumes["persist"].Path)
	}
	if len(u.Entries) != 2 {
		t.Errorf("entries = %d", len(u.Entries))
	}
}

func TestLiveFromStore(t *testing.T) {
	cases := []struct {
		store, path, want string
		ok                bool
	}{
		{"/persist", "/persist/home/a/x", "/home/a/x", true},
		{"/persist", "/persist", "/", true},
		{"/persist", "/persistent/x", "", false},
		{"/", "/etc/x", "/etc/x", true},
	}
	for _, c := range cases {
		got, ok := LiveFromStore(c.store, c.path)
		if got != c.want || ok != c.ok {
			t.Errorf("LiveFromStore(%q, %q) = %q, %v", c.store, c.path, got, ok)
		}
	}
}
