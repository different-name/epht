package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/different-name/epht/internal/manifest"
	"github.com/different-name/epht/internal/scan"
)

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ":") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type scanFlags struct {
	collapse bool
	size     bool
	bytes    bool
	json     bool
	null     bool

	manifests stringList
}

func (f *scanFlags) bind(fs *flag.FlagSet) {
	fs.BoolVar(&f.collapse, "collapse", false, "collapse fully-unpersisted dirs to one line instead of listing files")
	fs.BoolVar(&f.collapse, "c", false, "shorthand for --collapse")
	fs.BoolVar(&f.size, "size", false, "show on-disk size per entry plus a total")
	fs.BoolVar(&f.bytes, "bytes", false, "raw byte counts instead of human-readable")
	fs.BoolVar(&f.json, "json", false, "json array of {path, kind, size}")
	fs.BoolVar(&f.null, "null", false, "nul-separated output, for xargs -0")
	fs.BoolVar(&f.null, "0", false, "shorthand for --null")
	fs.Var(&f.manifests, "manifest", "manifest path, repeatable, else $EPHT_MANIFEST")
	fs.Var(&f.manifests, "m", "shorthand for --manifest")
}

func loadManifests(flagPaths []string) (*manifest.Manifest, error) {
	paths := flagPaths
	if len(paths) == 0 {
		env := os.Getenv("EPHT_MANIFEST")
		if env == "" {
			return nil, fmt.Errorf("no manifest: pass --manifest or set EPHT_MANIFEST")
		}
		paths = filepath.SplitList(env)
	}
	ms := make([]*manifest.Manifest, 0, len(paths))
	for _, p := range paths {
		m, err := manifest.Load(p)
		if err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return manifest.Union(ms...), nil
}

func cmdScan(cmd string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("epht "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f scanFlags
	f.bind(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	m, err := loadManifests(f.manifests)
	if err != nil {
		fmt.Fprintf(stderr, "epht: %v\n", err)
		return 2
	}
	mt := manifest.NewMatcher(m)
	opts := scan.Options{Collapse: f.collapse}

	var results []scan.Result
	switch cmd {
	case "volatile":
		roots, err := scan.ResolveRoots(fs.Args())
		if err != nil {
			fmt.Fprintf(stderr, "epht: %v\n", err)
			return 2
		}
		results, err = scan.Volatile(roots, mt, opts)
		if err != nil {
			fmt.Fprintf(stderr, "epht: %v\n", err)
			return 2
		}
	case "stale":
		results, err = scan.Stale(mt, opts)
		if err != nil {
			fmt.Fprintf(stderr, "epht: %v\n", err)
			return 2
		}
		if roots := fs.Args(); len(roots) > 0 {
			abs, err := scan.ResolveRoots(roots)
			if err != nil {
				fmt.Fprintf(stderr, "epht: %v\n", err)
				return 2
			}
			results = filterUnder(results, abs)
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	if err := emit(stdout, results, f); err != nil {
		fmt.Fprintf(stderr, "epht: %v\n", err)
		return 2
	}
	if len(results) > 0 {
		return 1
	}
	return 0
}

func cmdStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("epht status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var size, bytes bool
	var manifests stringList
	fs.BoolVar(&size, "size", false, "include total sizes")
	fs.BoolVar(&bytes, "bytes", false, "raw byte counts instead of human-readable")
	fs.Var(&manifests, "manifest", "manifest path, repeatable, else $EPHT_MANIFEST")
	fs.Var(&manifests, "m", "shorthand for --manifest")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	m, err := loadManifests(manifests)
	if err != nil {
		fmt.Fprintf(stderr, "epht: %v\n", err)
		return 2
	}
	mt := manifest.NewMatcher(m)

	vol, err := scan.Volatile([]string{"/"}, mt, scan.Options{})
	if err != nil {
		fmt.Fprintf(stderr, "epht: %v\n", err)
		return 2
	}
	st, err := scan.Stale(mt, scan.Options{})
	if err != nil {
		fmt.Fprintf(stderr, "epht: %v\n", err)
		return 2
	}

	for _, row := range []struct {
		name string
		rs   []scan.Result
	}{{"volatile", vol}, {"stale", st}} {
		if size {
			var total int64
			for _, r := range row.rs {
				total += scan.Size(r)
			}
			fmt.Fprintf(stdout, "%-9s %5d  %s\n", row.name, len(row.rs), sizeStr(total, bytes))
		} else {
			fmt.Fprintf(stdout, "%-9s %5d\n", row.name, len(row.rs))
		}
	}

	if len(vol)+len(st) > 0 {
		return 1
	}
	return 0
}

func filterUnder(results []scan.Result, roots []string) []scan.Result {
	out := results[:0:0]
	for _, r := range results {
		for _, root := range roots {
			if r.Path == root || root == "/" || strings.HasPrefix(r.Path, root+"/") {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
