package manifest

type Matcher struct {
	files   []Entry
	dirs    []Entry
	exclude []string
	stores  []string
	volumes []string
	owner   map[string]string
}

func NewMatcher(m *Manifest) *Matcher {
	mt := &Matcher{
		exclude: append([]string(nil), m.Exclude...),
		owner:   map[string]string{},
	}
	for _, v := range m.Volumes {
		mt.volumes = append(mt.volumes, v.Path)
	}
	for _, e := range m.Entries {
		switch e.Kind {
		case KindFile:
			mt.files = append(mt.files, e)
		case KindDir:
			mt.dirs = append(mt.dirs, e)
		}
		mt.stores = append(mt.stores, e.Store)
		mt.owner[e.Store] = e.Volume
	}
	mt.stores = unique(mt.stores)
	return mt
}

func (mt *Matcher) Stores() []string {
	return mt.stores
}

func (mt *Matcher) VolumeOf(store string) string {
	return mt.owner[store]
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
		if path == f.Live {
			return true
		}
	}
	for _, d := range mt.dirs {
		if underOrEqual(d.Live, path) {
			return true
		}
	}
	return false
}

// the most specific covering entry decides which store backs a live path
func (mt *Matcher) OwnedBy(path, store string) bool {
	best := -1
	owned := false
	consider := func(e Entry) {
		switch n := len(e.Live); {
		case n > best:
			best, owned = n, e.Store == store
		case n == best && e.Store == store:
			owned = true
		}
	}
	for _, f := range mt.files {
		if path == f.Live {
			consider(f)
		}
	}
	for _, d := range mt.dirs {
		if underOrEqual(d.Live, path) {
			consider(d)
		}
	}
	return owned
}

func (mt *Matcher) Excluded(path string) bool {
	for _, e := range mt.exclude {
		if underOrEqual(e, path) {
			return true
		}
	}
	return false
}

// volume roots and store roots hold persisted data, never live data
func (mt *Matcher) UnderStore(path string) bool {
	for _, s := range append(mt.stores, mt.volumes...) {
		if underOrEqual(s, path) {
			return true
		}
	}
	return false
}

func (mt *Matcher) IsEntry(path string) bool {
	for _, es := range [][]Entry{mt.files, mt.dirs} {
		for _, e := range es {
			if e.Live == path {
				return true
			}
		}
	}
	return false
}

func (mt *Matcher) HasPersistedDescendant(dir string) bool {
	return mt.hasDescendant(dir, func(Entry) bool { return true })
}

func (mt *Matcher) HasDescendantIn(dir, store string) bool {
	return mt.hasDescendant(dir, func(e Entry) bool { return e.Store == store })
}

func (mt *Matcher) HasDescendantOutside(dir, store string) bool {
	return mt.hasDescendant(dir, func(e Entry) bool { return e.Store != store })
}

func (mt *Matcher) hasDescendant(dir string, match func(Entry) bool) bool {
	for _, es := range [][]Entry{mt.files, mt.dirs} {
		for _, e := range es {
			if strictlyUnder(dir, e.Live) && match(e) {
				return true
			}
		}
	}
	return false
}

// false means the whole subtree is unkept and can be rolled up to one line
func (mt *Matcher) HasBoundaryBelow(dir string) bool {
	if mt.HasPersistedDescendant(dir) {
		return true
	}
	for _, p := range append(append(mt.exclude, mt.stores...), mt.volumes...) {
		if strictlyUnder(dir, p) {
			return true
		}
	}
	return false
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

// the most specific entry whose live path covers path
func (mt *Matcher) Covering(path string) (Entry, bool) {
	var best Entry
	found := false
	consider := func(e Entry) {
		if !found || len(e.Live) > len(best.Live) {
			best, found = e, true
		}
	}
	for _, f := range mt.files {
		if path == f.Live {
			consider(f)
		}
	}
	for _, d := range mt.dirs {
		if underOrEqual(d.Live, path) {
			consider(d)
		}
	}
	return best, found
}
