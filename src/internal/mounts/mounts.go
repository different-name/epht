package mounts

import (
	"os"
	"strconv"
	"strings"
)

type Mount struct {
	Point  string
	Root   string
	Device string
}

type Table []Mount

func Read() (Table, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	return Parse(string(data)), nil
}

func Parse(data string) Table {
	var t Table
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		t = append(t, Mount{
			Device: fields[2],
			Root:   unescape(fields[3]),
			Point:  unescape(fields[4]),
		})
	}
	return t
}

func (t Table) Points() map[string]bool {
	points := make(map[string]bool, len(t))
	for _, m := range t {
		points[m.Point] = true
	}
	return points
}

// the last mount at a point is the visible one
func (t Table) At(point string) (Mount, bool) {
	var found Mount
	ok := false
	for _, m := range t {
		if m.Point == point {
			found, ok = m, true
		}
	}
	return found, ok
}

// the mount a path lives on, the deepest point above it
func (t Table) Containing(path string) (Mount, bool) {
	var best Mount
	ok := false
	for _, m := range t {
		if m.Point == path || m.Point == "/" || strings.HasPrefix(path, m.Point+"/") {
			if !ok || len(m.Point) >= len(best.Point) {
				best, ok = m, true
			}
		}
	}
	return best, ok
}

// the kernel octal-escapes space, tab, newline and backslash in mount paths
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if c, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(c))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
