package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const Version = 2

type Kind string

const (
	KindDir  Kind = "dir"
	KindFile Kind = "file"
)

type Volume struct {
	Path string `json:"path"`
}

type Entry struct {
	Volume string `json:"volume"`
	Kind   Kind   `json:"kind"`
	Live   string `json:"live"`
	Store  string `json:"store"`
	User   string `json:"user,omitempty"`
	Group  string `json:"group,omitempty"`
	Mode   string `json:"mode,omitempty"`
}

func (e Entry) StorePath() string {
	return filepath.Join(e.Store, e.Live)
}

type Manifest struct {
	Version int               `json:"version"`
	Volumes map[string]Volume `json:"volumes"`
	Entries []Entry           `json:"entries"`
	Exclude []string          `json:"exclude"`
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
	for name, v := range m.Volumes {
		v.Path = filepath.Clean(v.Path)
		m.Volumes[name] = v
	}
	for i := range m.Entries {
		m.Entries[i].Live = filepath.Clean(m.Entries[i].Live)
		m.Entries[i].Store = filepath.Clean(m.Entries[i].Store)
	}
	for i := range m.Exclude {
		m.Exclude[i] = filepath.Clean(m.Exclude[i])
	}
}

// duplicates are tolerated, the matcher is idempotent
func Union(ms ...*Manifest) *Manifest {
	out := &Manifest{Version: Version, Volumes: map[string]Volume{}}
	for _, m := range ms {
		if m == nil {
			continue
		}
		for name, v := range m.Volumes {
			if _, dup := out.Volumes[name]; !dup {
				out.Volumes[name] = v
			}
		}
		out.Entries = append(out.Entries, m.Entries...)
		out.Exclude = append(out.Exclude, m.Exclude...)
	}
	return out
}

func LiveFromStore(store, path string) (string, bool) {
	store = filepath.Clean(store)
	path = filepath.Clean(path)
	if !underOrEqual(store, path) {
		return "", false
	}
	if store == path {
		return "/", true
	}
	if store == "/" {
		return path, true
	}
	return path[len(store):], true
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
