package scan

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/different-name/epht/internal/manifest"
)

// the emitted path is the backing one, that is what you delete to reclaim space
func Stale(mt *manifest.Matcher, opts Options) ([]Result, error) {
	var out []Result
	for _, store := range mt.Stores() {
		store = filepath.Clean(store)
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
			live, ok := manifest.LiveFromBacking(store, path)
			if !ok {
				return nil
			}

			if d.IsDir() {
				if mt.Covered(live) {
					return fs.SkipDir
				}
				if mt.HasPersistedDescendant(live) {
					return nil
				}
				if opts.Collapse {
					out = append(out, Result{Path: path, Kind: manifest.KindDir})
					return fs.SkipDir
				}
				return nil
			}

			if mt.Covered(live) {
				return nil
			}
			out = append(out, Result{Path: path, Kind: manifest.KindFile})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
