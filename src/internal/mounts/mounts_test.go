package mounts

import "testing"

const sample = `22 1 0:22 / / rw,relatime - btrfs /dev/mapper/rootfs rw,subvol=/root
40 22 0:22 /persist /persist rw,relatime - btrfs /dev/mapper/rootfs rw,subvol=/
41 22 0:22 /persist/system/var/log /var/log rw,relatime - btrfs /dev/mapper/rootfs rw
42 22 0:22 /persist/home/a/.config/Proton\040Pass /home/a/.config/Proton\040Pass rw - btrfs /dev/mapper/rootfs rw
43 22 0:30 / /boot rw - vfat /dev/vda1 rw
`

func TestParse(t *testing.T) {
	table := Parse(sample)
	if len(table) != 5 {
		t.Fatalf("mounts = %d", len(table))
	}
	m, ok := table.At("/home/a/.config/Proton Pass")
	if !ok || m.Root != "/persist/home/a/.config/Proton Pass" {
		t.Errorf("escaped mount = %+v, %v", m, ok)
	}
	if !table.Points()["/var/log"] {
		t.Error("points should include /var/log")
	}
}

func TestContaining(t *testing.T) {
	table := Parse(sample)
	cases := map[string]string{
		"/persist/system/var/lib": "/persist",
		"/boot/EFI":               "/boot",
		"/var/log/x":              "/var/log",
		"/var/lib":                "/",
		"/persistent":             "/",
	}
	for path, want := range cases {
		m, ok := table.Containing(path)
		if !ok || m.Point != want {
			t.Errorf("Containing(%q) = %q, want %q", path, m.Point, want)
		}
	}
}
