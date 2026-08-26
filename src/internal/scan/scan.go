package scan

import (
	"path/filepath"

	"github.com/different-name/epht/internal/manifest"
)

type Result struct {
	Path string
	Kind manifest.Kind
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
