//go:build linux

package sysinfo

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// enrich fills the Linux-only fields from /proc, /etc/os-release and statfs.
// Every source is best-effort: a missing or unreadable file just leaves its
// fields zero rather than failing the whole collection.
func enrich(info *Info) {
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		info.MemTotalBytes, info.MemAvailableBytes = parseMeminfo(string(b))
	}
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		info.Load1, info.Load5, info.Load15 = parseLoadavg(string(b))
	}
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		info.Distro, info.DistroVersion = parseOsRelease(string(b))
	}
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		info.Kernel = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		if f := strings.Fields(string(b)); len(f) > 0 {
			info.UptimeSeconds, _ = strconv.ParseFloat(f[0], 64)
		}
	}

	var fs syscall.Statfs_t
	if err := syscall.Statfs("/", &fs); err == nil {
		bs := uint64(fs.Bsize)
		info.DiskTotalBytes = fs.Blocks * bs
		info.DiskFreeBytes = fs.Bavail * bs
	}
}
