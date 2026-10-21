package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/different-name/keep-nix/internal/check"
	"github.com/different-name/keep-nix/internal/manifest"
	"github.com/different-name/keep-nix/internal/scan"
)

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ":") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type listFlags struct {
	files bool
	size  bool
	bytes bool
	json  bool
	null  bool

	manifests stringList
}

func (f *listFlags) bind(fs *flag.FlagSet) {
	fs.BoolVar(&f.files, "files", false, "every file instead of one line per fully unkept or orphaned dir")
	fs.BoolVar(&f.files, "f", false, "shorthand for --files")
	fs.BoolVar(&f.size, "size", false, "show on-disk size per entry plus a total")
	fs.BoolVar(&f.bytes, "bytes", false, "raw byte counts instead of human-readable")
	fs.BoolVar(&f.json, "json", false, "json array of {report, path, kind, volume, size}")
	fs.BoolVar(&f.null, "null", false, "nul-separated output, for xargs -0")
	fs.BoolVar(&f.null, "0", false, "shorthand for --null")
	bindManifest(fs, &f.manifests)
}

func bindManifest(fs *flag.FlagSet, manifests *stringList) {
	fs.Var(manifests, "manifest", "manifest path, repeatable, else $KEEP_MANIFEST")
	fs.Var(manifests, "m", "shorthand for --manifest")
}

func loadManifests(flagPaths []string) (*manifest.Manifest, error) {
	paths := flagPaths
	if len(paths) == 0 {
		env := os.Getenv("KEEP_MANIFEST")
		if env == "" {
			return nil, fmt.Errorf("no manifest: pass --manifest or set KEEP_MANIFEST")
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

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "keep: %v\n", err)
	return 2
}

func isReport(s string) bool {
	return s == "unkept" || s == "orphaned" || s == "declared"
}

func cmdList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("keep list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f listFlags
	f.bind(fs)

	// flags and the report name may come in any order
	what := ""
	var paths []string
	for rest := args; ; {
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		if what == "" && isReport(rest[0]) {
			what = rest[0]
		} else {
			paths = append(paths, rest[0])
		}
		rest = rest[1:]
	}

	m, err := loadManifests(f.manifests)
	if err != nil {
		return fail(stderr, err)
	}
	if what == "declared" {
		return emitDeclared(stdout, m, f)
	}

	mt := manifest.NewMatcher(m)
	opts := scan.Options{Collapse: !f.files}
	roots, err := scan.ResolveRoots(paths)
	if err != nil {
		return fail(stderr, err)
	}

	sections := []string{what}
	if what == "" {
		sections = []string{"unkept", "orphaned"}
	}
	var reports []report
	for _, section := range sections {
		var results []scan.Result
		switch section {
		case "unkept":
			results, err = scan.Unkept(roots, mt, opts)
		case "orphaned":
			results, err = scan.Orphaned(mt, opts)
			if err == nil && len(paths) > 0 {
				results = filterUnder(results, roots)
			}
		}
		if err != nil {
			return fail(stderr, err)
		}
		sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
		reports = append(reports, report{name: section, results: results})
	}
	if err := emit(stdout, reports, f, what == ""); err != nil {
		return fail(stderr, err)
	}
	for _, r := range reports {
		if len(r.results) > 0 {
			return 1
		}
	}
	return 0
}

func cmdCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("keep check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var manifests stringList
	bindManifest(fs, &manifests)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	m, err := loadManifests(manifests)
	if err != nil {
		return fail(stderr, err)
	}
	problems, err := check.Run(m)
	if err != nil {
		return fail(stderr, err)
	}
	for _, p := range problems {
		fmt.Fprintln(stdout, p)
	}
	if len(problems) > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "%d entries ok\n", len(m.Entries))
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
