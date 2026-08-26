package manifest

import "testing"

func testMatcher() *Matcher {
	return NewMatcher(&Manifest{
		Version: Version,
		Stores:  []string{"/persist", "/state"},
		Persisted: []Entry{
			{Kind: KindDir, Live: "/var/log", Store: "/persist"},
			{Kind: KindDir, Live: "/home/diffy/.ssh", Store: "/persist"},
			{Kind: KindFile, Live: "/etc/machine-id", Store: "/persist"},
		},
		Exclude: []string{"/nix", "/proc", "/tmp"},
	})
}

func TestCovered(t *testing.T) {
	mt := testMatcher()
	cases := []struct {
		path string
		want bool
	}{
		{"/var/log", true},           // dir entry itself
		{"/var/log/app/x.log", true}, // under a dir entry
		{"/var/logging", false},      // component-aware: not /var/log
		{"/var/lo", false},           // partial component
		{"/var", false},              // parent of a dir entry is not covered
		{"/etc/machine-id", true},    // exact file entry
		{"/etc/machine-id/x", false}, // a file entry covers nothing below it
		{"/etc", false},
		{"/home/diffy/.ssh/id_ed25519", true},
		{"/home/diffy/.sshd", false}, // component-aware
		{"/nope", false},
	}
	for _, c := range cases {
		if got := mt.Covered(c.path); got != c.want {
			t.Errorf("Covered(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestExcluded(t *testing.T) {
	mt := testMatcher()
	cases := []struct {
		path string
		want bool
	}{
		{"/nix", true},
		{"/nix/store/abc", true},
		{"/nixos", false}, // component-aware
		{"/proc/1", true},
		{"/tmp", true},
		{"/var/log", false},
		{"/", false},
	}
	for _, c := range cases {
		if got := mt.Excluded(c.path); got != c.want {
			t.Errorf("Excluded(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestUnderStore(t *testing.T) {
	mt := testMatcher()
	cases := []struct {
		path string
		want bool
	}{
		{"/persist", true},
		{"/persist/var/log", true},
		{"/persistent", false}, // component-aware
		{"/state/foo", true},
		{"/var/log", false},
	}
	for _, c := range cases {
		if got := mt.UnderStore(c.path); got != c.want {
			t.Errorf("UnderStore(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestHasPersistedDescendant(t *testing.T) {
	mt := testMatcher()
	cases := []struct {
		dir  string
		want bool
	}{
		{"/home", true},             // /home/diffy/.ssh sits below
		{"/home/diffy", true},       // .ssh below
		{"/home/diffy/.ssh", false}, // the entry itself, not strictly below
		{"/etc", true},              // machine-id file below
		{"/etc/machine-id", false},  // the entry itself
		{"/var", true},              // /var/log below
		{"/var/log", false},         // the entry itself
		{"/var/log/app", false},     // inside a persisted dir, nothing below it
		{"/srv", false},             // unrelated
		{"/", true},                 // root has descendants
	}
	for _, c := range cases {
		if got := mt.HasPersistedDescendant(c.dir); got != c.want {
			t.Errorf("HasPersistedDescendant(%q) = %v, want %v", c.dir, got, c.want)
		}
	}
}

func TestUnderOrEqualBoundary(t *testing.T) {
	if underOrEqual("/var/log", "/var/logging") {
		t.Error("/var/log must not cover /var/logging")
	}
	if !underOrEqual("/var/log", "/var/log") {
		t.Error("/var/log must cover itself")
	}
	if !underOrEqual("/var/log", "/var/log/app") {
		t.Error("/var/log must cover /var/log/app")
	}
	if !underOrEqual("/", "/anything") {
		t.Error("root must cover everything")
	}
}
