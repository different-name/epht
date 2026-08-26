package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const Version = 1

type Kind string

const (
	KindDir  Kind = "dir"
	KindFile Kind = "file"
)

type Entry struct {
	Kind  Kind   `json:"kind"`
	Live  string `json:"live"`
	Store string `json:"store"`
}

type Manifest struct {
	Version      int      `json:"version"`
	Stores       []string `json:"stores"`
	Persisted    []Entry  `json:"persisted"`
	Exclude      []string `json:"exclude"`
	ExcludeGlobs []string `json:"exclude_globs"`
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, data)
}

func Parse(name string, data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest %s: %w", name, err)
	}
	if m.Version != Version {
		return nil, fmt.Errorf("manifest %s: unsupported version %d (want %d)", name, m.Version, Version)
	}
	m.normalize()
	return &m, nil
}

// the matcher assumes cleaned absolute paths
func (m *Manifest) normalize() {
	for i := range m.Stores {
		m.Stores[i] = filepath.Clean(m.Stores[i])
	}
	for i := range m.Persisted {
		m.Persisted[i].Live = filepath.Clean(m.Persisted[i].Live)
		m.Persisted[i].Store = filepath.Clean(m.Persisted[i].Store)
	}
	for i := range m.Exclude {
		m.Exclude[i] = filepath.Clean(m.Exclude[i])
	}
	// exclude_globs are patterns, not paths, so they are left untouched
}

// duplicates are tolerated, the matcher is idempotent
func Union(ms ...*Manifest) *Manifest {
	out := &Manifest{Version: Version}
	for _, m := range ms {
		if m == nil {
			continue
		}
		out.Stores = append(out.Stores, m.Stores...)
		out.Persisted = append(out.Persisted, m.Persisted...)
		out.Exclude = append(out.Exclude, m.Exclude...)
		out.ExcludeGlobs = append(out.ExcludeGlobs, m.ExcludeGlobs...)
	}
	return out
}

func Backing(store, live string) string {
	return filepath.Join(store, live)
}

func LiveFromBacking(store, backing string) (string, bool) {
	store = filepath.Clean(store)
	backing = filepath.Clean(backing)
	if !underOrEqual(store, backing) {
		return "", false
	}
	if store == backing {
		return "/", true
	}
	if store == "/" {
		return backing, true
	}
	return backing[len(store):], true
}
