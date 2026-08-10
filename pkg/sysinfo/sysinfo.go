// Package sysinfo collects system information about the host the process runs
// on. The rich fields (memory, disk, load, kernel, distro) are Linux-only and
// filled by enrich in sysinfo_linux.go; on other platforms they stay zero so
// the app still builds and runs for local development.
package sysinfo

import (
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Info is the system information reported at GET /.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`

	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	CPUs     int    `json:"cpus"`

	Kernel        string `json:"kernel,omitempty"`
	Distro        string `json:"distro,omitempty"`
	DistroVersion string `json:"distro_version,omitempty"`

	UptimeSeconds float64 `json:"uptime_seconds"`
	Load1         float64 `json:"load1"`
	Load5         float64 `json:"load5"`
	Load15        float64 `json:"load15"`

	MemTotalBytes     uint64 `json:"mem_total_bytes"`
	MemAvailableBytes uint64 `json:"mem_available_bytes"`
	DiskTotalBytes    uint64 `json:"disk_total_bytes"`
	DiskFreeBytes     uint64 `json:"disk_free_bytes"`
}

// Collect gathers system information. version and commit are the build-time
// identifiers injected into main.
func Collect(version, commit string) Info {
	host, _ := os.Hostname()
	info := Info{
		Version:   version,
		Commit:    commit,
		GoVersion: runtime.Version(),
		Hostname:  host,
		IP:        primaryIP(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		CPUs:      runtime.NumCPU(),
	}
	enrich(&info) // platform-specific; no-op off Linux
	return info
}

// primaryIP returns the source IP the kernel would use to reach the outside
// world. The UDP "connect" sends no packets; it just resolves routing so we
// pick the primary interface without scanning them all. Empty on failure.
func primaryIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// parseMeminfo pulls MemTotal and MemAvailable (reported in kB) out of the
// contents of /proc/meminfo and returns them as bytes.
func parseMeminfo(s string) (total, available uint64) {
	for _, line := range strings.Split(s, "\n") {
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest) // e.g. "16384000 kB"
		if len(fields) == 0 {
			continue
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "MemTotal":
			total = kb * 1024
		case "MemAvailable":
			available = kb * 1024
		}
	}
	return total, available
}

// parseLoadavg reads the three load averages from the contents of
// /proc/loadavg (e.g. "0.15 0.10 0.05 1/234 5678").
func parseLoadavg(s string) (l1, l5, l15 float64) {
	f := strings.Fields(s)
	if len(f) < 3 {
		return 0, 0, 0
	}
	l1, _ = strconv.ParseFloat(f[0], 64)
	l5, _ = strconv.ParseFloat(f[1], 64)
	l15, _ = strconv.ParseFloat(f[2], 64)
	return l1, l5, l15
}

// parseOsRelease extracts ID and VERSION_ID from the contents of
// /etc/os-release.
func parseOsRelease(s string) (distro, version string) {
	for _, line := range strings.Split(s, "\n") {
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"`)
		switch strings.TrimSpace(key) {
		case "ID":
			distro = val
		case "VERSION_ID":
			version = val
		}
	}
	return distro, version
}
