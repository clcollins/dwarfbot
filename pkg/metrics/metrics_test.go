package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNew_CreatesRegistry(t *testing.T) {
	m := New()
	if m.Registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if m.PlatformConnected == nil {
		t.Error("expected PlatformConnected to be initialized")
	}
	if m.PlatformConnectionAttemptsTotal == nil {
		t.Error("expected PlatformConnectionAttemptsTotal to be initialized")
	}
	if m.Info == nil {
		t.Error("expected Info to be initialized")
	}
}

func TestNew_MetricsAppearAfterObservation(t *testing.T) {
	m := New()
	// Metrics only appear in Gather() after being observed
	m.PlatformConnected.WithLabelValues("twitch").Set(1)
	m.PlatformConnectionAttemptsTotal.WithLabelValues("twitch", "success").Inc()
	m.Info.WithLabelValues("test", "go1.25").Set(1)

	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := map[string]bool{}
	for _, fam := range families {
		found[fam.GetName()] = true
	}

	for _, name := range []string{
		"dwarfbot_platform_connected",
		"dwarfbot_platform_connection_attempts_total",
		"dwarfbot_info",
	} {
		if !found[name] {
			t.Errorf("expected metric %q after observation", name)
		}
	}
}

func TestInit_SetsInfoGauge(t *testing.T) {
	m := New()
	m.Init("v0.2.0", time.Now())

	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	for _, fam := range families {
		if fam.GetName() == "dwarfbot_info" {
			if len(fam.GetMetric()) == 0 {
				t.Fatal("expected info metric to have values")
			}
			if fam.GetMetric()[0].GetGauge().GetValue() != 1 {
				t.Errorf("expected info gauge value 1, got %f", fam.GetMetric()[0].GetGauge().GetValue())
			}
			// Verify version label is present
			for _, lp := range fam.GetMetric()[0].GetLabel() {
				if lp.GetName() == "version" && lp.GetValue() != "v0.2.0" {
					t.Errorf("expected version label 'v0.2.0', got %q", lp.GetValue())
				}
			}
			return
		}
	}
	t.Error("dwarfbot_info metric not found")
}

func TestInit_RegistersUptimeMetric(t *testing.T) {
	m := New()
	start := time.Now().Add(-10 * time.Second)
	m.Init("test", start)

	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	for _, fam := range families {
		if fam.GetName() == "dwarfbot_uptime_seconds" {
			val := fam.GetMetric()[0].GetGauge().GetValue()
			if val < 10 {
				t.Errorf("expected uptime >= 10s, got %f", val)
			}
			return
		}
	}
	t.Error("dwarfbot_uptime_seconds metric not found")
}

func TestSetConfigMetrics_BothConfigured(t *testing.T) {
	m := New()
	m.SetConfigMetrics("twitch-tok", "discord-tok", []string{"ch1"}, []string{"ch2"})

	if v := testutil.ToFloat64(m.PlatformTokenPresent.WithLabelValues("twitch")); v != 1 {
		t.Errorf("expected twitch token present=1, got %f", v)
	}
	if v := testutil.ToFloat64(m.PlatformTokenPresent.WithLabelValues("discord")); v != 1 {
		t.Errorf("expected discord token present=1, got %f", v)
	}
	if v := testutil.ToFloat64(m.PlatformConfigured.WithLabelValues("twitch")); v != 1 {
		t.Errorf("expected twitch configured=1, got %f", v)
	}
	if v := testutil.ToFloat64(m.PlatformConfigured.WithLabelValues("discord")); v != 1 {
		t.Errorf("expected discord configured=1, got %f", v)
	}
}

func TestSetConfigMetrics_NeitherConfigured(t *testing.T) {
	m := New()
	m.SetConfigMetrics("", "", nil, nil)

	if v := testutil.ToFloat64(m.PlatformTokenPresent.WithLabelValues("twitch")); v != 0 {
		t.Errorf("expected twitch token present=0, got %f", v)
	}
	if v := testutil.ToFloat64(m.PlatformConfigured.WithLabelValues("twitch")); v != 0 {
		t.Errorf("expected twitch configured=0, got %f", v)
	}
}

func TestSetConfigMetrics_TokenButNoChannels(t *testing.T) {
	m := New()
	m.SetConfigMetrics("tok", "", []string{}, nil)

	if v := testutil.ToFloat64(m.PlatformTokenPresent.WithLabelValues("twitch")); v != 1 {
		t.Errorf("expected twitch token present=1, got %f", v)
	}
	if v := testutil.ToFloat64(m.PlatformConfigured.WithLabelValues("twitch")); v != 0 {
		t.Errorf("expected twitch configured=0 (no channels), got %f", v)
	}
}

func TestPlatformConnected_SetAndRead(t *testing.T) {
	m := New()
	m.PlatformConnected.WithLabelValues("twitch").Set(1)
	m.PlatformConnected.WithLabelValues("discord").Set(0)

	if v := testutil.ToFloat64(m.PlatformConnected.WithLabelValues("twitch")); v != 1 {
		t.Errorf("expected twitch connected=1, got %f", v)
	}
	if v := testutil.ToFloat64(m.PlatformConnected.WithLabelValues("discord")); v != 0 {
		t.Errorf("expected discord connected=0, got %f", v)
	}
}

func TestConnectionAttempts_Increment(t *testing.T) {
	m := New()
	m.PlatformConnectionAttemptsTotal.WithLabelValues("twitch", "success").Inc()
	m.PlatformConnectionAttemptsTotal.WithLabelValues("twitch", "failure").Inc()
	m.PlatformConnectionAttemptsTotal.WithLabelValues("twitch", "failure").Inc()

	if v := testutil.ToFloat64(m.PlatformConnectionAttemptsTotal.WithLabelValues("twitch", "success")); v != 1 {
		t.Errorf("expected 1 success, got %f", v)
	}
	if v := testutil.ToFloat64(m.PlatformConnectionAttemptsTotal.WithLabelValues("twitch", "failure")); v != 2 {
		t.Errorf("expected 2 failures, got %f", v)
	}
}

func TestMessageMetrics(t *testing.T) {
	m := New()
	m.MessagesReceivedTotal.WithLabelValues("discord").Inc()
	m.MessagesSentTotal.WithLabelValues("discord", "success").Inc()
	m.MessagesSentTotal.WithLabelValues("discord", "failure").Inc()

	if v := testutil.ToFloat64(m.MessagesReceivedTotal.WithLabelValues("discord")); v != 1 {
		t.Errorf("expected 1 received, got %f", v)
	}
	if v := testutil.ToFloat64(m.MessagesSentTotal.WithLabelValues("discord", "success")); v != 1 {
		t.Errorf("expected 1 sent success, got %f", v)
	}
}

func TestCommandMetrics(t *testing.T) {
	m := New()
	m.CommandsProcessedTotal.WithLabelValues("twitch", "ping", "false").Inc()
	m.CommandsProcessedTotal.WithLabelValues("twitch", "shutdown", "true").Inc()

	if v := testutil.ToFloat64(m.CommandsProcessedTotal.WithLabelValues("twitch", "ping", "false")); v != 1 {
		t.Errorf("expected 1 ping command, got %f", v)
	}
	if v := testutil.ToFloat64(m.CommandsProcessedTotal.WithLabelValues("twitch", "shutdown", "true")); v != 1 {
		t.Errorf("expected 1 shutdown command, got %f", v)
	}
}
