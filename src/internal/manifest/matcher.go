package manifest

import "strings"

type Matcher struct {
	files   []string
	dirs    []string
	exclude []string
	stores  []string
}

func NewMatcher(m *Manifest) *Matcher {
	mt := &Matcher{
		exclude: append([]string(nil), m.Exclude...),
		// unioned manifests repeat stores, and the stale scan walks each one
		stores: unique(m.Stores),
	}
	for _, e := range m.Persisted {
		switch e.Kind {
		case KindFile:
			mt.files = append(mt.files, e.Live)
		case KindDir:
			mt.dirs = append(mt.dirs, e.Live)
		}
	}
	return mt
}

func (mt *Matcher) Stores() []string {
	return mt.stores
}

func (mt *Matcher) IsStoreRoot(path string) bool {
	for _, s := range mt.stores {
		if s == path {
			return true
		}
	}
	return false
}

// a file entry is persisted exactly, a dir entry covers its whole subtree
func (mt *Matcher) Covered(path string) bool {
	for _, f := range mt.files {
		if path == f {
			return true
		}
	}
	for _, d := range mt.dirs {
		if underOrEqual(d, path) {
			return true
		}
	}
	return false
}

func (mt *Matcher) Excluded(path string) bool {
	for _, e := range mt.exclude {
		if underOrEqual(e, path) {
			return true
		}
	}
	return false
}

func (mt *Matcher) UnderStore(path string) bool {
	for _, s := range mt.stores {
		if underOrEqual(s, path) {
			return true
		}
	}
	return false
}

func (mt *Matcher) HasPersistedDescendant(dir string) bool {
	for _, f := range mt.files {
		if strictlyUnder(dir, f) {
			return true
		}
	}
	for _, d := range mt.dirs {
		if strictlyUnder(dir, d) {
			return true
		}
	}
	return false
}

// false means the whole subtree is volatile and can be rolled up to one line
func (mt *Matcher) HasBoundaryBelow(dir string) bool {
	if mt.HasPersistedDescendant(dir) {
		return true
	}
	for _, e := range mt.exclude {
		if strictlyUnder(dir, e) {
			return true
		}
	}
	for _, s := range mt.stores {
		if strictlyUnder(dir, s) {
			return true
		}
	}
	return false
}

// component boundaries, not string prefixes, so /var/log never matches /var/logging
func strictlyUnder(prefix, path string) bool {
	if prefix == "/" {
		return path != "/"
	}
	return strings.HasPrefix(path, prefix+"/")
}

func underOrEqual(prefix, path string) bool {
	return prefix == path || strictlyUnder(prefix, path)
}

func unique(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
