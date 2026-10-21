package check

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/different-name/keep-nix/internal/manifest"
	"github.com/different-name/keep-nix/internal/mounts"
)

type Problem struct {
	Entry   manifest.Entry
	Message string
}

func (p Problem) String() string {
	return p.Entry.Live + ": " + p.Message
}

func Run(m *manifest.Manifest) ([]Problem, error) {
	table, err := mounts.Read()
	if err != nil {
		return nil, err
	}
	return run(m, table, os.Lstat), nil
}

func run(m *manifest.Manifest, table mounts.Table, lstat func(string) (os.FileInfo, error)) []Problem {
	var out []Problem
	add := func(e manifest.Entry, format string, args ...any) {
		out = append(out, Problem{Entry: e, Message: fmt.Sprintf(format, args...)})
	}

	for _, e := range m.Entries {
		store := e.StorePath()
		fi, err := lstat(store)
		switch {
		case err != nil && e.Kind == manifest.KindDir:
			add(e, "store copy %s is missing", store)
			continue
		case err == nil:
			if e.Kind == manifest.KindDir && !fi.IsDir() {
				add(e, "store copy %s is not a directory", store)
				continue
			}
			if msg := ownership(e, fi); msg != "" {
				add(e, "%s", msg)
			}
		}

		if e.Kind == manifest.KindFile {
			// linked until the next boot binds it, whether or not the store copy exists yet
			if target, lerr := os.Readlink(e.Live); lerr == nil && target == store {
				continue
			}
			if err != nil {
				add(e, "not linked to %s", store)
				continue
			}
		}

		mounted, ok := table.At(e.Live)
		if !ok {
			add(e, "not mounted")
			continue
		}
		want, ok := expected(table, store)
		if !ok {
			add(e, "no filesystem holds %s", store)
			continue
		}
		if mounted.Device != want.Device || mounted.Root != want.Root {
			add(e, "mounted from %s instead of %s", mounted.Root, store)
		}
	}
	return out
}

// the mount a bind of storePath would show, root relative to the filesystem holding it
func expected(table mounts.Table, storePath string) (mounts.Mount, bool) {
	fs, ok := table.Containing(storePath)
	if !ok {
		return mounts.Mount{}, false
	}
	rel := strings.TrimPrefix(storePath, fs.Point)
	return mounts.Mount{Device: fs.Device, Root: filepath.Join(fs.Root, rel)}, true
}

func ownership(e manifest.Entry, fi os.FileInfo) string {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	if e.User != "" {
		if u, err := user.Lookup(e.User); err == nil && strconv.Itoa(int(st.Uid)) != u.Uid {
			return fmt.Sprintf("store copy owned by uid %d, declared %s", st.Uid, e.User)
		}
	}
	if e.Group != "" {
		if g, err := user.LookupGroup(e.Group); err == nil && strconv.Itoa(int(st.Gid)) != g.Gid {
			return fmt.Sprintf("store copy group is gid %d, declared %s", st.Gid, e.Group)
		}
	}
	if e.Mode != "" {
		if want, err := strconv.ParseUint(e.Mode, 8, 32); err == nil && uint64(st.Mode&0o7777) != want {
			return fmt.Sprintf("store copy mode is %04o, declared %s", st.Mode&0o7777, e.Mode)
		}
	}
	return ""
}
