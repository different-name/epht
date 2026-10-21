package manifest

import "testing"

func testMatcher() *Matcher {
	return NewMatcher(&Manifest{
		Version: Version,
		Volumes: map[string]Volume{"persist": {Path: "/persist"}, "scratch": {Path: "/scratch"}},
		Entries: []Entry{
			{Volume: "persist", Kind: KindDir, Live: "/var/log", Store: "/persist/system"},
			{Volume: "persist", Kind: KindFile, Live: "/etc/machine-id", Store: "/persist/system"},
			{Volume: "persist", Kind: KindDir, Live: "/home/a/.local/share/Steam", Store: "/persist"},
			{Volume: "scratch", Kind: KindDir, Live: "/home/a/.local/share/Steam/steamapps/common", Store: "/scratch"},
		},
		Exclude: []string{"/proc", "/persist", "/scratch"},
	})
}

func TestCoveredUsesComponents(t *testing.T) {
	mt := testMatcher()
	if !mt.Covered("/var/log/x") || !mt.Covered("/var/log") {
		t.Error("dir entry should cover itself and its subtree")
	}
	if mt.Covered("/var/logging") {
		t.Error("/var/logging is not under /var/log")
	}
	if !mt.Covered("/etc/machine-id") || mt.Covered("/etc/machine-id/x") {
		t.Error("file entry covers exactly itself")
	}
}

func TestOwnedByMostSpecific(t *testing.T) {
	mt := testMatcher()
	steam := "/home/a/.local/share/Steam"
	if !mt.OwnedBy(steam+"/config", "/persist") {
		t.Error("Steam/config belongs to persist")
	}
	if mt.OwnedBy(steam+"/steamapps/common/game", "/persist") {
		t.Error("common is nested in scratch, persist must not claim it")
	}
	if !mt.OwnedBy(steam+"/steamapps/common/game", "/scratch") {
		t.Error("common belongs to scratch")
	}
	if mt.OwnedBy("/unrelated", "/persist") {
		t.Error("nothing covers /unrelated")
	}
}

func TestStoresAndVolumes(t *testing.T) {
	mt := testMatcher()
	if got := mt.Stores(); len(got) != 3 {
		t.Errorf("stores = %v", got)
	}
	if mt.VolumeOf("/scratch") != "scratch" || mt.VolumeOf("/persist/system") != "persist" {
		t.Error("store roots map to their volume")
	}
	if !mt.UnderStore("/persist/anything") || !mt.UnderStore("/scratch") {
		t.Error("volume paths count as store")
	}
	if !mt.IsStoreRoot("/persist") || mt.IsStoreRoot("/persist/system/var") {
		t.Error("only roots are roots")
	}
}

func TestBoundaries(t *testing.T) {
	mt := testMatcher()
	if !mt.HasBoundaryBelow("/var") || !mt.HasBoundaryBelow("/") {
		t.Error("entries and excludes below are boundaries")
	}
	if mt.HasBoundaryBelow("/opt") {
		t.Error("/opt has nothing below")
	}
	if !mt.HasDescendantOutside("/home/a/.local/share/Steam", "/persist") {
		t.Error("scratch nests inside the Steam entry")
	}
	if mt.HasDescendantIn("/home/a/.local/share/Steam", "/persist") {
		t.Error("no persist entry below Steam")
	}
}
