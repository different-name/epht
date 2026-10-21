package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUsageAndUnknown(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 2 || !strings.Contains(errb.String(), "usage:") {
		t.Errorf("no args: code %d, stderr %q", code, errb.String())
	}
	errb.Reset()
	if code := run([]string{"bogus"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "unknown command") {
		t.Errorf("bogus: code %d", code)
	}
	out.Reset()
	if code := run([]string{"help"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "keep list") {
		t.Errorf("help: code %d", code)
	}
}

func TestListDeclared(t *testing.T) {
	dir := t.TempDir()
	mf := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(mf, []byte(`{
		"version": 2,
		"volumes": {"persist": {"path": "/persist"}, "scratch": {"path": "/scratch"}},
		"entries": [
			{"volume": "scratch", "kind": "dir", "live": "/var/cache", "store": "/scratch/system"},
			{"volume": "persist", "kind": "file", "live": "/etc/machine-id", "store": "/persist/system"},
			{"volume": "persist", "kind": "dir", "live": "/var/log", "store": "/persist/system"}
		],
		"exclude": []
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	want := "persist (/persist)\n  file /etc/machine-id\n  dir  /var/log\n\nscratch (/scratch)\n  dir  /var/cache\n"
	// the report name may follow the flags
	for _, args := range [][]string{{"list", "declared", "-m", mf}, {"list", "-m", mf, "declared"}} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 0 {
			t.Fatalf("%v: code %d: %s", args, code, errb.String())
		}
		if out.String() != want {
			t.Fatalf("%v: got\n%s\nwant\n%s", args, out.String(), want)
		}
	}
}
