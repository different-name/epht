package transfer

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/different-name/keep-nix/internal/manifest"
	"github.com/different-name/keep-nix/internal/mounts"
)

type Plan struct {
	Live   string
	Kind   manifest.Kind
	Source string
	Target string
	// missing directories below the target root mirror their live counterparts, else the source store's
	TargetRoot string
	SourceRoot string
	// the source is inside another entry of its volume and becomes the mount point there
	Clear bool
}

type Env struct {
	Manifest *manifest.Manifest
	Mounts   mounts.Table
	// home directories of users with home entries, their paths are stored at the volume root
	Homes []string
}

func (env Env) volumePath(name string) (string, error) {
	v, ok := env.Manifest.Volumes[name]
	if !ok {
		names := make([]string, 0, len(env.Manifest.Volumes))
		for n := range env.Manifest.Volumes {
			names = append(names, n)
		}
		sort.Strings(names)
		return "", fmt.Errorf("unknown volume %q, the volumes are %s", name, strings.Join(names, ", "))
	}
	if fi, err := os.Stat(v.Path); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("volume %s has no directory at %s", name, v.Path)
	}
	return v.Path, nil
}

func (env Env) isMountPoint(path string) bool {
	_, ok := env.Mounts.At(path)
	return ok
}

func under(prefix, path string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func kindOf(path string) (manifest.Kind, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	switch {
	case fi.IsDir():
		return manifest.KindDir, nil
	case fi.Mode().IsRegular():
		return manifest.KindFile, nil
	default:
		return "", fmt.Errorf("%s is neither a directory nor a regular file", path)
	}
}

func (env Env) checkTarget(target string) error {
	if env.isMountPoint(target) {
		return fmt.Errorf("%s is a mount point, its contents belong to another volume", target)
	}
	fi, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.IsDir() && isEmptyDir(target) {
		return nil
	}
	return fmt.Errorf("%s already exists", target)
}

func Adopt(env Env, live, volume string) (Plan, error) {
	live = filepath.Clean(live)
	root, err := env.volumePath(volume)
	if err != nil {
		return Plan{}, err
	}
	if e, ok := manifest.NewMatcher(env.Manifest).Covering(live); ok {
		return Plan{}, fmt.Errorf("%s is already kept in %s by %s", live, e.Volume, e.Live)
	}
	if env.isMountPoint(live) {
		return Plan{}, fmt.Errorf("%s is a mount point, adopt it before it is declared", live)
	}
	kind, err := kindOf(live)
	if err != nil {
		return Plan{}, err
	}

	system := true
	for _, h := range env.Homes {
		if under(h, live) {
			system = false
		}
	}
	if system {
		root = filepath.Join(root, "system")
	}
	target := filepath.Join(root, live)
	if err := env.checkTarget(target); err != nil {
		return Plan{}, err
	}
	return Plan{Live: live, Kind: kind, Source: live, Target: target, TargetRoot: root}, nil
}

func Move(env Env, live, volume string) (Plan, error) {
	live = filepath.Clean(live)
	targetVolume, err := env.volumePath(volume)
	if err != nil {
		return Plan{}, err
	}
	covering, ok := manifest.NewMatcher(env.Manifest).Covering(live)
	if !ok {
		return Plan{}, fmt.Errorf("%s is not kept, use adopt", live)
	}
	if covering.Volume == volume {
		return Plan{}, fmt.Errorf("%s is already kept in %s", live, volume)
	}

	// system entries sit under the volume's system directory, home entries at its root
	suffix := strings.TrimPrefix(covering.Store, env.Manifest.Volumes[covering.Volume].Path)
	root := targetVolume + suffix
	source := filepath.Join(covering.Store, live)
	target := filepath.Join(root, live)

	if env.isMountPoint(source) {
		return Plan{}, fmt.Errorf("%s is a mount point, its own contents cannot be read", source)
	}
	kind, err := kindOf(source)
	if err != nil {
		return Plan{}, err
	}
	if err := env.checkTarget(target); err != nil {
		return Plan{}, err
	}
	return Plan{
		Live:       live,
		Kind:       kind,
		Source:     source,
		Target:     target,
		TargetRoot: root,
		SourceRoot: covering.Store,
		Clear:      live != covering.Live,
	}, nil
}

// cp keeps ownership, modes, timestamps, xattrs and hard links, and reflinks within one filesystem
func Execute(p Plan, cp string) error {
	if err := mirrorParents(p); err != nil {
		return err
	}
	cmd := exec.Command(cp, "-a", "--reflink=auto", "-T", p.Source, p.Target)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copying %s: %v: %s", p.Source, err, strings.TrimSpace(string(out)))
	}
	src, err := tally(p.Source)
	if err != nil {
		return err
	}
	dst, err := tally(p.Target)
	if err != nil {
		return err
	}
	if src != dst {
		return fmt.Errorf("copy of %s differs from the source (%d files, %d bytes against %d files, %d bytes), nothing was cleared", p.Source, dst.files, dst.bytes, src.files, src.bytes)
	}
	if !p.Clear {
		return nil
	}
	if p.Kind == manifest.KindFile {
		return os.Truncate(p.Source, 0)
	}
	entries, err := os.ReadDir(p.Source)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(p.Source, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func mirrorParents(p Plan) error {
	if err := os.MkdirAll(p.TargetRoot, 0o755); err != nil {
		return err
	}
	var missing []string
	for dir := filepath.Dir(p.Target); dir != p.TargetRoot && under(p.TargetRoot, dir); dir = filepath.Dir(dir) {
		if _, err := os.Lstat(dir); err == nil {
			break
		}
		missing = append(missing, dir)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		dir := missing[i]
		mode, uid, gid := fs.FileMode(0o755), os.Getuid(), os.Getgid()
		live, _ := manifest.LiveFromStore(p.TargetRoot, dir)
		candidates := []string{live}
		if p.SourceRoot != "" {
			candidates = append(candidates, filepath.Join(p.SourceRoot, live))
		}
		for _, c := range candidates {
			fi, err := os.Lstat(c)
			if err != nil || !fi.IsDir() {
				continue
			}
			if st, ok := fi.Sys().(*syscall.Stat_t); ok {
				mode, uid, gid = fi.Mode().Perm(), int(st.Uid), int(st.Gid)
				break
			}
		}
		if err := os.Mkdir(dir, mode); err != nil {
			return err
		}
		if err := os.Chmod(dir, mode); err != nil {
			return err
		}
		if uid != os.Getuid() || gid != os.Getgid() {
			if err := os.Lchown(dir, uid, gid); err != nil {
				return err
			}
		}
	}
	return nil
}

type count struct{ files, bytes int64 }

func tally(path string) (count, error) {
	var c count
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		c.files++
		if fi.Mode().IsRegular() {
			c.bytes += fi.Size()
		}
		return nil
	})
	return c, err
}

func isEmptyDir(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) == 0
}

// passwd homes that hold at least one home entry, root's /root stays a system path otherwise
func Homes(m *manifest.Manifest, passwd string) []string {
	var homes []string
	for _, line := range strings.Split(passwd, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 6 || fields[5] == "" || fields[5] == "/" {
			continue
		}
		home := filepath.Clean(fields[5])
		for _, e := range m.Entries {
			if v, ok := m.Volumes[e.Volume]; ok && e.Store == v.Path && under(home, e.Live) {
				homes = append(homes, home)
				break
			}
		}
	}
	return homes
}
