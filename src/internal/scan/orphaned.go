package scan

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/different-name/keep-nix/internal/manifest"
	"github.com/different-name/keep-nix/internal/mounts"
)

// store data no entry claims, reported by its store path since that is what gets deleted
func Orphaned(mt *manifest.Matcher, opts Options) ([]Result, error) {
	ms, err := mounts.Read()
	if err != nil {
		return nil, err
	}
	return orphaned(mt, opts, ms.Points())
}

func orphaned(mt *manifest.Matcher, opts Options, points map[string]bool) ([]Result, error) {
	var out []Result
	for _, store := range mt.Stores() {
		volume := mt.VolumeOf(store)
		if _, err := os.Lstat(store); err != nil {
			// unmounted or never created
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err := filepath.WalkDir(store, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			live, ok := manifest.LiveFromStore(store, path)
			if !ok {
				return nil
			}

			// a live bind mount from another store propagates back into this tree
			if path != store && points[path] {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				// a store nested inside another one gets its own walk
				if path != store && mt.IsStoreRoot(path) {
					return fs.SkipDir
				}
				if mt.OwnedBy(live, store) {
					if mt.HasDescendantOutside(live, store) {
						return nil
					}
					return fs.SkipDir
				}
				if path == store || mt.HasDescendantIn(live, store) {
					return nil
				}
				if opts.Collapse {
					if mt.IsEntry(live) && emptyStub(path, d) {
						return fs.SkipDir
					}
					out = append(out, Result{Path: path, Kind: manifest.KindDir, Volume: volume})
					return fs.SkipDir
				}
				return nil
			}

			if mt.OwnedBy(live, store) || (mt.IsEntry(live) && emptyStub(path, d)) {
				return nil
			}
			out = append(out, Result{Path: path, Kind: manifest.KindFile, Volume: volume})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// the mount target for a nested entry from another store is an empty dir or file
func emptyStub(path string, d fs.DirEntry) bool {
	if !d.IsDir() {
		fi, err := d.Info()
		return err == nil && fi.Size() == 0
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err == io.EOF
}
