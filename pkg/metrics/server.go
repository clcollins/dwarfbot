package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewServeMux creates an HTTP mux with /metrics and /healthz endpoints.
func NewServeMux(registry *prometheus.Registry) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	return mux
}

// NewServer creates an HTTP server for metrics and health endpoints.
func NewServer(addr string, registry *prometheus.Registry) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           NewServeMux(registry),
		ReadHeaderTimeout: 5 * time.Second,
	}
}
