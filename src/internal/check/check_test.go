package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/different-name/keep-nix/internal/manifest"
	"github.com/different-name/keep-nix/internal/mounts"
)

const table = `22 1 0:22 /root / rw - btrfs /dev/mapper/rootfs rw
40 22 0:22 /persist /persist rw - btrfs /dev/mapper/rootfs rw
41 22 0:22 /persist/system/var/log /var/log rw - btrfs /dev/mapper/rootfs rw
42 22 0:22 /persist/home/a/Documents /home/a/Documents rw - btrfs /dev/mapper/rootfs rw
43 22 0:22 /persist/system/var/lib/systemd/random-seed /var/lib/systemd/random-seed rw - btrfs /dev/mapper/rootfs rw
44 22 0:22 /scratch /scratch rw - btrfs /dev/mapper/rootfs rw
`

const cacheMount = "45 22 0:22 /scratch/system/var/cache /var/cache rw - btrfs /dev/mapper/rootfs rw\n"

type fakeInfo struct{ dir bool }

func (fakeInfo) Name() string { return "" }
func (fakeInfo) Size() int64  { return 0 }
func (f fakeInfo) Mode() os.FileMode {
	if f.dir {
		return os.ModeDir | 0o755
	}
	return 0o644
}
func (fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool      { return f.dir }
func (fakeInfo) Sys() any           { return nil }

func lstat(existing map[string]bool) func(string) (os.FileInfo, error) {
	return func(path string) (os.FileInfo, error) {
		if dir, ok := existing[path]; ok {
			return fakeInfo{dir: dir}, nil
		}
		return nil, os.ErrNotExist
	}
}

func entries() []manifest.Entry {
	return []manifest.Entry{
		{Volume: "persist", Kind: manifest.KindDir, Live: "/var/log", Store: "/persist/system"},
		{Volume: "persist", Kind: manifest.KindDir, Live: "/home/a/Documents", Store: "/persist"},
		{Volume: "persist", Kind: manifest.KindFile, Live: "/var/lib/systemd/random-seed", Store: "/persist/system"},
		{Volume: "scratch", Kind: manifest.KindDir, Live: "/var/cache", Store: "/scratch/system"},
	}
}

func TestAllMounted(t *testing.T) {
	existing := map[string]bool{
		"/persist/system/var/log":                     true,
		"/persist/home/a/Documents":                   true,
		"/persist/system/var/lib/systemd/random-seed": false,
		"/scratch/system/var/cache":                   true,
	}
	problems := run(&manifest.Manifest{Entries: entries()}, mounts.Parse(table+cacheMount), lstat(existing))
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

// a file written through its first-boot link stays linked until the next boot binds it
func TestLinkedFileWithStoreCopy(t *testing.T) {
	dir := t.TempDir()
	e := manifest.Entry{Volume: "persist", Kind: manifest.KindFile, Live: filepath.Join(dir, "token"), Store: filepath.Join(dir, "store")}
	if err := os.Symlink(e.StorePath(), e.Live); err != nil {
		t.Fatal(err)
	}
	es := []manifest.Entry{e}
	for _, exists := range []bool{false, true} {
		existing := map[string]bool{}
		if exists {
			existing[e.StorePath()] = false
		}
		problems := run(&manifest.Manifest{Entries: es}, mounts.Parse(table), lstat(existing))
		if len(problems) != 0 {
			t.Fatalf("store copy exists %v: problems = %v", exists, problems)
		}
	}
}

func TestWrongSourceAndMissing(t *testing.T) {
	existing := map[string]bool{
		"/persist/system/var/log":   true,
		"/persist/home/a/Documents": true,
		"/scratch/system/var/cache": true,
	}
	wrong := strings.Replace(table, "/persist/home/a/Documents /home/a/Documents", "/scratch/home/a/Documents /home/a/Documents", 1)
	problems := run(&manifest.Manifest{Entries: entries()}, mounts.Parse(wrong), lstat(existing))
	msgs := make([]string, len(problems))
	for i, p := range problems {
		msgs[i] = p.String()
	}
	want := []string{
		"/home/a/Documents: mounted from /scratch/home/a/Documents instead of /persist/home/a/Documents",
		"/var/lib/systemd/random-seed: not linked to /persist/system/var/lib/systemd/random-seed",
		"/var/cache: not mounted",
	}
	if strings.Join(msgs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(msgs, "\n"), strings.Join(want, "\n"))
	}
}
