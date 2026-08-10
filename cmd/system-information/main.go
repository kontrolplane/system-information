// Command system-information is a tiny HTTP server that reports the system
// information of the machine it runs on. It is the reference application for
// the kontrolplane fleet: deployed onto a VM by konfig and used to check that
// the host converged as expected.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/kontrolplane/system-information/pkg/sysinfo"
)

// Build-time identifiers, set with -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "none"
)

func main() {
	port := flag.Int("port", 9898, "HTTP port to listen on")
	flag.Parse()

	srv := newServer()
	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", *port),
		Handler:           srv.logging(srv.mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM: stop accepting, drain in-flight.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Listening on %s (version %s)", httpSrv.Addr, version)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Printf("Shutdown failed: %v", err)
	}
}

type server struct {
	mux   *http.ServeMux
	ready atomic.Bool

	mu       sync.Mutex
	requests map[int]int64 // HTTP responses by status code, for /metrics
}

func newServer() *server {
	s := &server{mux: http.NewServeMux(), requests: map[int]int64{}}
	s.ready.Store(true)

	s.mux.HandleFunc("GET /", s.handleInfo)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "OK")
	})
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
	s.mux.HandleFunc("POST /readyz/{state}", s.handleReadyzToggle)
	s.mux.HandleFunc("GET /version", s.handleVersion)
	s.mux.HandleFunc("GET /metrics", s.handleMetrics)
	return s
}

func (s *server) handleInfo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sysinfo.Collect(version, commit))
}

func (s *server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if !s.ready.Load() {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "OK")
}

// handleReadyzToggle flips readiness so the readiness probe can be exercised:
// POST /readyz/disable -> 503 on /readyz, POST /readyz/enable -> 200.
func (s *server) handleReadyzToggle(w http.ResponseWriter, r *http.Request) {
	switch r.PathValue("state") {
	case "enable":
		s.ready.Store(true)
	case "disable":
		s.ready.Store(false)
	default:
		http.Error(w, "state must be enable or disable", http.StatusBadRequest)
		return
	}
	fmt.Fprintln(w, "OK")
}

func (s *server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"version": version,
		"commit":  commit,
		"go":      sysinfo.Collect(version, commit).GoVersion,
	})
}

// handleMetrics writes Prometheus text exposition by hand — no client library,
// matching konfig's controlplane.
func (s *server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	i := sysinfo.Collect(version, commit)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	metric(w, "system_information_up", "gauge", "1 if the process is serving", 1)
	metric(w, "system_information_cpus", "gauge", "number of logical CPUs", float64(i.CPUs))
	metric(w, "system_information_uptime_seconds", "gauge", "host uptime in seconds", i.UptimeSeconds)
	metric(w, "system_information_load1", "gauge", "1-minute load average", i.Load1)
	metric(w, "system_information_mem_total_bytes", "gauge", "total memory in bytes", float64(i.MemTotalBytes))
	metric(w, "system_information_mem_available_bytes", "gauge", "available memory in bytes", float64(i.MemAvailableBytes))
	metric(w, "system_information_disk_total_bytes", "gauge", "total disk bytes on /", float64(i.DiskTotalBytes))
	metric(w, "system_information_disk_free_bytes", "gauge", "free disk bytes on /", float64(i.DiskFreeBytes))

	fmt.Fprintln(w, "# HELP system_information_http_requests_total HTTP responses by status code")
	fmt.Fprintln(w, "# TYPE system_information_http_requests_total counter")
	s.mu.Lock()
	codes := make([]int, 0, len(s.requests))
	for c := range s.requests {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	for _, c := range codes {
		fmt.Fprintf(w, "system_information_http_requests_total{code=\"%d\"} %d\n", c, s.requests[c])
	}
	s.mu.Unlock()
}

func metric(w http.ResponseWriter, name, typ, help string, v float64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %g\n", name, help, name, typ, name, v)
}

// logging logs each request and records its status code for /metrics.
func (s *server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)

		s.mu.Lock()
		s.requests[sw.status]++
		s.mu.Unlock()

		log.Printf("%s %s %d %dms", r.Method, r.URL.Path, sw.status, time.Since(start).Milliseconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
