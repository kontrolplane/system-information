package sysinfo

import "testing"

func TestParseMeminfo(t *testing.T) {
	const s = `MemTotal:       16384000 kB
MemFree:         1048576 kB
MemAvailable:    8192000 kB
Buffers:          100000 kB`
	total, avail := parseMeminfo(s)
	if total != 16384000*1024 {
		t.Errorf("total = %d, want %d", total, uint64(16384000*1024))
	}
	if avail != 8192000*1024 {
		t.Errorf("available = %d, want %d", avail, uint64(8192000*1024))
	}
}

func TestParseLoadavg(t *testing.T) {
	l1, l5, l15 := parseLoadavg("0.15 0.10 0.05 1/234 5678\n")
	if l1 != 0.15 || l5 != 0.10 || l15 != 0.05 {
		t.Errorf("loadavg = %v %v %v, want 0.15 0.10 0.05", l1, l5, l15)
	}
	// Garbage in must not panic and must read as zero.
	if a, b, c := parseLoadavg("nope"); a != 0 || b != 0 || c != 0 {
		t.Errorf("bad input = %v %v %v, want zeros", a, b, c)
	}
}

func TestParseOsRelease(t *testing.T) {
	const s = `PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
ID=debian
VERSION_ID="12"
VERSION_CODENAME=bookworm`
	distro, version := parseOsRelease(s)
	if distro != "debian" {
		t.Errorf("distro = %q, want debian", distro)
	}
	if version != "12" {
		t.Errorf("version = %q, want 12", version)
	}
}
