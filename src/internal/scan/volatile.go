package scan

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/different-name/epht/internal/manifest"
)

func Volatile(roots []string, mt *manifest.Matcher, opts Options) ([]Result, error) {
	var out []Result
	for _, root := range roots {
		root = filepath.Clean(root)
		if _, err := os.Lstat(root); err != nil {
			return nil, err
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// unreadable or raced away, keep walking the rest
				return nil
			}

			if d.IsDir() {
				if mt.Excluded(path) || mt.UnderStore(path) || mt.Covered(path) {
					return fs.SkipDir
				}
				if mt.HasBoundaryBelow(path) {
					return nil
				}
				if opts.Collapse {
					// a dir of only symlinks or special files holds nothing that would be lost
					if hasRegularFile(path) {
						out = append(out, Result{Path: path, Kind: manifest.KindDir})
					}
					return fs.SkipDir
				}
				return nil
			}

			// only regular files are data, symlinks are nix-store managed and recreated
			if !d.Type().IsRegular() {
				return nil
			}
			if mt.Excluded(path) || mt.UnderStore(path) || mt.Covered(path) {
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

func hasRegularFile(root string) bool {
	found := false
	filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}
