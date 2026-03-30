package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeMux_MetricsEndpoint(t *testing.T) {
	m := New()
	m.PlatformConnected.WithLabelValues("twitch").Set(1)

	mux := NewServeMux(m.Registry)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	bodyStr := string(body)

	// Only check for metrics that have been observed (twitch connected was set above)
	expectedMetrics := []string{
		"dwarfbot_platform_connected",
	}
	for _, name := range expectedMetrics {
		if !strings.Contains(bodyStr, name) {
			t.Errorf("expected /metrics to contain %q", name)
		}
	}
}

func TestServeMux_HealthzEndpoint(t *testing.T) {
	m := New()
	mux := NewServeMux(m.Registry)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if strings.TrimSpace(string(body)) != "ok" {
		t.Errorf("expected 'ok', got %q", string(body))
	}
}

func TestServeMux_MetricsContainsGoCollector(t *testing.T) {
	m := New()
	mux := NewServeMux(m.Registry)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !strings.Contains(string(body), "go_goroutines") {
		t.Error("expected Go runtime metrics (go_goroutines) in output")
	}
}
