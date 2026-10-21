package transfer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/different-name/keep-nix/internal/manifest"
	"github.com/different-name/keep-nix/internal/mounts"
)

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func cp(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("cp")
	if err != nil {
		t.Skip("no cp")
	}
	return p
}

type fixture struct {
	live, safe, loose string
	m                 *manifest.Manifest
}

func setup(t *testing.T, entries ...manifest.Entry) fixture {
	t.Helper()
	tmp := t.TempDir()
	f := fixture{
		live:  filepath.Join(tmp, "live"),
		safe:  filepath.Join(tmp, "safe"),
		loose: filepath.Join(tmp, "loose"),
	}
	f.m = &manifest.Manifest{
		Version: manifest.Version,
		Volumes: map[string]manifest.Volume{"safe": {Path: f.safe}, "loose": {Path: f.loose}},
	}
	for _, e := range entries {
		e.Live = filepath.Join(f.live, e.Live)
		switch e.Store {
		case "safe":
			e.Store = filepath.Join(f.safe, "system")
		case "loose":
			e.Store = filepath.Join(f.loose, "system")
		}
		f.m.Entries = append(f.m.Entries, e)
	}
	// the temp dir's root-owned ancestors already exist in a real store
	for _, root := range []string{f.safe, f.loose, filepath.Join(f.safe, "system"), filepath.Join(f.loose, "system")} {
		if err := os.MkdirAll(filepath.Join(root, tmp), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f fixture) env() Env {
	return Env{Manifest: f.m}
}

func TestAdoptSystemDir(t *testing.T) {
	f := setup(t)
	write(t, filepath.Join(f.live, "var/lib/thing/state"), "data")
	if err := os.Chmod(filepath.Join(f.live, "var/lib"), 0o700); err != nil {
		t.Fatal(err)
	}

	p, err := Adopt(f.env(), filepath.Join(f.live, "var/lib/thing"), "safe")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(f.safe, "system", f.live, "var/lib/thing")
	if p.Target != want || p.Clear {
		t.Fatalf("plan %+v, want target %s", p, want)
	}
	if err := Execute(p, cp(t)); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(want, "state")); got != "data" {
		t.Fatalf("copied %q", got)
	}
	fi, err := os.Stat(filepath.Dir(want))
	if err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("parent mirrors the live parent's mode, got %v %v", fi.Mode(), err)
	}
	if got := read(t, filepath.Join(f.live, "var/lib/thing/state")); got != "data" {
		t.Fatal("adopt must leave the live data in place")
	}
}

func TestAdoptHomeFileAtVolumeRoot(t *testing.T) {
	f := setup(t)
	home := filepath.Join(f.live, "home/alice")
	write(t, filepath.Join(home, ".token"), "secret")
	env := f.env()
	env.Homes = []string{home}

	p, err := Adopt(env, filepath.Join(home, ".token"), "safe")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(f.safe, home, ".token"); p.Target != want || p.Kind != manifest.KindFile {
		t.Fatalf("plan %+v, want file target %s", p, want)
	}
	if err := Execute(p, cp(t)); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p.Target); got != "secret" {
		t.Fatalf("copied %q", got)
	}
}

func TestAdoptRefusals(t *testing.T) {
	f := setup(t, manifest.Entry{Volume: "safe", Kind: manifest.KindDir, Live: "var/log", Store: "safe"})
	write(t, filepath.Join(f.live, "var/log/x"), "x")
	write(t, filepath.Join(f.live, "var/lib/a/x"), "x")
	write(t, filepath.Join(f.live, "var/lib/b/x"), "x")
	write(t, filepath.Join(f.safe, "system", f.live, "var/lib/a/old"), "old")
	if err := os.MkdirAll(filepath.Join(f.safe, "system", f.live, "var/lib/b"), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		path, volume, err string
	}{
		{"var/log/x", "safe", "already kept"},
		{"var/lib/a", "safe", "already exists"},
		{"var/lib/a", "nope", "unknown volume"},
	}
	for _, c := range cases {
		_, err := Adopt(f.env(), filepath.Join(f.live, c.path), c.volume)
		if err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s in %s: got %v, want %q", c.path, c.volume, err, c.err)
		}
	}
	if _, err := Adopt(f.env(), filepath.Join(f.live, "var/lib/b"), "safe"); err != nil {
		t.Errorf("an empty store dir is a stub to fill: %v", err)
	}

	env := f.env()
	env.Mounts = mounts.Table{{Point: filepath.Join(f.live, "var/lib/b")}}
	if _, err := Adopt(env, filepath.Join(f.live, "var/lib/b"), "safe"); err == nil || !strings.Contains(err.Error(), "mount point") {
		t.Errorf("a mounted live path: got %v", err)
	}
}

func TestMoveNestedEmptiesTheSource(t *testing.T) {
	f := setup(t, manifest.Entry{Volume: "safe", Kind: manifest.KindDir, Live: "steam", Store: "safe"})
	src := filepath.Join(f.safe, "system", f.live, "steam/common")
	write(t, filepath.Join(src, "game/data"), "game")
	write(t, filepath.Join(f.safe, "system", f.live, "steam/config"), "cfg")

	p, err := Move(f.env(), filepath.Join(f.live, "steam/common"), "loose")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(f.loose, "system", f.live, "steam/common")
	if p.Source != src || p.Target != want || !p.Clear {
		t.Fatalf("plan %+v", p)
	}
	if err := Execute(p, cp(t)); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(want, "game/data")); got != "game" {
		t.Fatalf("copied %q", got)
	}
	if entries, err := os.ReadDir(src); err != nil || len(entries) != 0 {
		t.Fatalf("source left as an empty mount point, got %v %v", entries, err)
	}
	if got := read(t, filepath.Join(f.safe, "system", f.live, "steam/config")); got != "cfg" {
		t.Fatal("the rest of the outer entry is untouched")
	}
}

func TestMoveEntryKeepsTheSource(t *testing.T) {
	f := setup(t, manifest.Entry{Volume: "safe", Kind: manifest.KindDir, Live: "cache", Store: "safe"})
	src := filepath.Join(f.safe, "system", f.live, "cache")
	write(t, filepath.Join(src, "x"), "x")

	p, err := Move(f.env(), filepath.Join(f.live, "cache"), "loose")
	if err != nil {
		t.Fatal(err)
	}
	if p.Clear {
		t.Fatal("an entry's own copy becomes an orphan, not a mount point")
	}
	if err := Execute(p, cp(t)); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(p.Target, "x")) != "x" || read(t, filepath.Join(src, "x")) != "x" {
		t.Fatal("both copies hold the data")
	}
}

func TestMoveRefusals(t *testing.T) {
	f := setup(t,
		manifest.Entry{Volume: "safe", Kind: manifest.KindDir, Live: "cache", Store: "safe"},
		manifest.Entry{Volume: "loose", Kind: manifest.KindDir, Live: "tmp", Store: "loose"},
	)
	write(t, filepath.Join(f.safe, "system", f.live, "cache/x"), "x")
	write(t, filepath.Join(f.loose, "system", f.live, "cache/y"), "y")

	cases := []struct {
		path, volume, err string
	}{
		{"cache", "safe", "already kept in safe"},
		{"elsewhere", "loose", "not kept"},
		{"cache", "loose", "already exists"},
	}
	for _, c := range cases {
		_, err := Move(f.env(), filepath.Join(f.live, c.path), c.volume)
		if err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s to %s: got %v, want %q", c.path, c.volume, err, c.err)
		}
	}

	env := f.env()
	src := filepath.Join(f.safe, "system", f.live, "cache")
	env.Mounts = mounts.Table{{Point: src}}
	if _, err := Move(env, filepath.Join(f.live, "cache"), "loose"); err == nil || !strings.Contains(err.Error(), "mount point") {
		t.Errorf("a source hidden under a mount: got %v", err)
	}
}

func TestHomes(t *testing.T) {
	m := &manifest.Manifest{
		Volumes: map[string]manifest.Volume{"safe": {Path: "/persist"}},
		Entries: []manifest.Entry{
			{Volume: "safe", Kind: manifest.KindDir, Live: "/home/alice/.config/app", Store: "/persist"},
			{Volume: "safe", Kind: manifest.KindDir, Live: "/root/.ssh", Store: "/persist/system"},
		},
	}
	passwd := "root:x:0:0::/root:/bin/sh\nalice:x:1000:100::/home/alice:/bin/sh\nnobody:x:65534:65534::/:/bin/false\n"
	got := Homes(m, passwd)
	if len(got) != 1 || got[0] != "/home/alice" {
		t.Fatalf("got %v", got)
	}
}
