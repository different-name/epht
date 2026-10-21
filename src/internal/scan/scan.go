package scan

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/different-name/keep-nix/internal/manifest"
)

type Result struct {
	Path   string
	Kind   manifest.Kind
	Volume string
}

type Options struct {
	Collapse bool
}

func ResolveRoots(args []string) ([]string, error) {
	if len(args) == 0 {
		return []string{"/"}, nil
	}
	roots := make([]string, 0, len(args))
	for _, a := range args {
		abs, err := filepath.Abs(a)
		if err != nil {
			return nil, err
		}
		roots = append(roots, abs)
	}
	return roots, nil
}

// a boundary dir has nothing persisted or excluded below it, so sum it whole
func Size(r Result) int64 {
	if r.Kind == manifest.KindFile {
		if fi, err := os.Lstat(r.Path); err == nil {
			return fi.Size()
		}
		return 0
	}
	var total int64
	filepath.WalkDir(r.Path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				total += fi.Size()
			}
		}
		return nil
	})
	return total
}
