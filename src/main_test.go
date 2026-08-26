package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/different-name/epht/internal/manifest"
)

func fixture(t *testing.T) (root, manifestPath string) {
	t.Helper()
	root = t.TempDir()
	for _, f := range []string{"kept/a.txt", "loose.txt"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := manifest.Manifest{
		Version: manifest.Version,
		Stores:  []string{filepath.Join(root, "persist")},
		Persisted: []manifest.Entry{
			{Kind: manifest.KindDir, Live: filepath.Join(root, "kept"), Store: filepath.Join(root, "persist")},
		},
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath = filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, manifestPath
}

func run3(args ...string) (code int, out, errs string) {
	var o, e bytes.Buffer
	code = run(args, &o, &e)
	return code, o.String(), e.String()
}

func TestRunVolatileFound(t *testing.T) {
	root, mp := fixture(t)
	code, out, errs := run3("volatile", "--manifest", mp, root)
	if code != 1 {
		t.Fatalf("exit = %d (want 1), stderr: %s", code, errs)
	}
	want := filepath.Join(root, "loose.txt")
	if strings.TrimSpace(out) != want {
		t.Fatalf("out = %q, want %q", out, want)
	}
}

func TestRunVolatileClean(t *testing.T) {
	root, mp := fixture(t)
	// restrict to the persisted subtree: nothing volatile there
	code, out, _ := run3("volatile", "--manifest", mp, filepath.Join(root, "kept"))
	if code != 0 {
		t.Fatalf("exit = %d (want 0)", code)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
}

func TestRunJSON(t *testing.T) {
	root, mp := fixture(t)
	code, out, _ := run3("volatile", "--json", "--manifest", mp, root)
	if code != 1 {
		t.Fatalf("exit = %d", code)
	}
	var items []jsonItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(items) != 1 || items[0].Kind != "file" || items[0].Size != 5 {
		t.Fatalf("items = %+v", items)
	}
}

func TestRunSizeAndNull(t *testing.T) {
	root, mp := fixture(t)

	_, out, _ := run3("volatile", "--size", "--bytes", "--manifest", mp, root)
	if !strings.Contains(out, "5  ") || !strings.Contains(out, "total") {
		t.Fatalf("size output missing size/total: %q", out)
	}

	_, nout, _ := run3("volatile", "-0", "--manifest", mp, root)
	if !strings.HasSuffix(nout, "\x00") {
		t.Fatalf("null output not nul-terminated: %q", nout)
	}
}

func TestRunCollapseFlag(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"keep/a", "newdir/sub/f1", "newdir/f2"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := manifest.Manifest{
		Version:   manifest.Version,
		Stores:    []string{filepath.Join(root, "persist")},
		Persisted: []manifest.Entry{{Kind: manifest.KindDir, Live: filepath.Join(root, "keep"), Store: filepath.Join(root, "persist")}},
	}
	data, _ := json.Marshal(m)
	mp := filepath.Join(t.TempDir(), "m.json")
	if err := os.WriteFile(mp, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, out, _ := run3("volatile", "--manifest", mp, root)
	got := strings.Fields(out)
	want := []string{filepath.Join(root, "newdir/f2"), filepath.Join(root, "newdir/sub/f1")}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("default view = %q, want %q", got, want)
	}

	_, rout, _ := run3("volatile", "--collapse", "--manifest", mp, root)
	if strings.TrimSpace(rout) != filepath.Join(root, "newdir") {
		t.Fatalf("collapse view = %q, want %q", rout, filepath.Join(root, "newdir"))
	}
}

func TestRunStale(t *testing.T) {
	root := t.TempDir()
	store := filepath.Join(root, "persist")
	for _, f := range []string{"kept/a.txt", "orphan/b.txt"} {
		p := filepath.Join(store, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := manifest.Manifest{
		Version:   manifest.Version,
		Stores:    []string{store},
		Persisted: []manifest.Entry{{Kind: manifest.KindDir, Live: "/kept", Store: store}},
	}
	data, _ := json.Marshal(m)
	mp := filepath.Join(t.TempDir(), "m.json")
	if err := os.WriteFile(mp, data, 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, errs := run3("stale", "--manifest", mp)
	if code != 1 {
		t.Fatalf("exit = %d, stderr %s", code, errs)
	}
	if strings.TrimSpace(out) != filepath.Join(store, "orphan/b.txt") {
		t.Fatalf("stale out = %q", out)
	}

	_, rout, _ := run3("stale", "--collapse", "--manifest", mp)
	if strings.TrimSpace(rout) != filepath.Join(store, "orphan") {
		t.Fatalf("stale --collapse out = %q", rout)
	}
}

func TestRunErrors(t *testing.T) {
	os.Unsetenv("EPHT_MANIFEST")
	if code, _, _ := run3("volatile"); code != 2 {
		t.Errorf("no manifest exit = %d, want 2", code)
	}
	if code := run(nil, io.Discard, io.Discard); code != 2 {
		t.Errorf("no args exit = %d, want 2", code)
	}
	if code, _, _ := run3("bogus"); code != 2 {
		t.Errorf("unknown command exit = %d, want 2", code)
	}
	if code := run([]string{"--help"}, io.Discard, io.Discard); code != 0 {
		t.Errorf("help exit = %d, want 0", code)
	}
}
