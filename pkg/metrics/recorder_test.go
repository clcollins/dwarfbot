package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecorder_RecordConnectionAttempt(t *testing.T) {
	m := New()
	r := NewRecorder(m)

	r.RecordConnectionAttempt("twitch", "success")
	r.RecordConnectionAttempt("twitch", "failure")
	r.RecordConnectionAttempt("twitch", "failure")

	if v := testutil.ToFloat64(m.PlatformConnectionAttemptsTotal.WithLabelValues("twitch", "success")); v != 1 {
		t.Errorf("expected 1 success attempt, got %f", v)
	}
	if v := testutil.ToFloat64(m.PlatformConnectionAttemptsTotal.WithLabelValues("twitch", "failure")); v != 2 {
		t.Errorf("expected 2 failure attempts, got %f", v)
	}
}

func TestRecorder_RecordConnected(t *testing.T) {
	m := New()
	r := NewRecorder(m)

	r.RecordConnected("discord")

	if v := testutil.ToFloat64(m.PlatformConnected.WithLabelValues("discord")); v != 1 {
		t.Errorf("expected connected=1, got %f", v)
	}
}

func TestRecorder_RecordDisconnected(t *testing.T) {
	m := New()
	r := NewRecorder(m)

	r.RecordConnected("twitch")
	r.RecordDisconnected("twitch", "error")

	if v := testutil.ToFloat64(m.PlatformConnected.WithLabelValues("twitch")); v != 0 {
		t.Errorf("expected connected=0 after disconnect, got %f", v)
	}
	if v := testutil.ToFloat64(m.PlatformDisconnectionsTotal.WithLabelValues("twitch", "error")); v != 1 {
		t.Errorf("expected 1 disconnection, got %f", v)
	}
}

func TestRecorder_RecordConnectionDuration(t *testing.T) {
	m := New()
	r := NewRecorder(m)

	r.RecordConnectionDuration("twitch", 30*time.Second)

	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	for _, fam := range families {
		if fam.GetName() == "dwarfbot_platform_connection_duration_seconds" {
			if len(fam.GetMetric()) == 0 {
				t.Fatal("expected duration metric to have observations")
			}
			count := fam.GetMetric()[0].GetHistogram().GetSampleCount()
			if count != 1 {
				t.Errorf("expected 1 observation, got %d", count)
			}
			return
		}
	}
	t.Error("duration metric not found")
}

func TestRecorder_RecordMessageReceived(t *testing.T) {
	m := New()
	r := NewRecorder(m)

	r.RecordMessageReceived("discord")
	r.RecordMessageReceived("discord")

	if v := testutil.ToFloat64(m.MessagesReceivedTotal.WithLabelValues("discord")); v != 2 {
		t.Errorf("expected 2 received, got %f", v)
	}
}

func TestRecorder_RecordMessageSent(t *testing.T) {
	m := New()
	r := NewRecorder(m)

	r.RecordMessageSent("twitch", "success")
	r.RecordMessageSent("twitch", "failure")

	if v := testutil.ToFloat64(m.MessagesSentTotal.WithLabelValues("twitch", "success")); v != 1 {
		t.Errorf("expected 1 success, got %f", v)
	}
	if v := testutil.ToFloat64(m.MessagesSentTotal.WithLabelValues("twitch", "failure")); v != 1 {
		t.Errorf("expected 1 failure, got %f", v)
	}
}

func TestRecorder_RecordCommandProcessed(t *testing.T) {
	m := New()
	r := NewRecorder(m)

	r.RecordCommandProcessed("discord", "ping", "false")
	r.RecordCommandProcessed("discord", "shutdown", "true")

	if v := testutil.ToFloat64(m.CommandsProcessedTotal.WithLabelValues("discord", "ping", "false")); v != 1 {
		t.Errorf("expected 1 ping, got %f", v)
	}
	if v := testutil.ToFloat64(m.CommandsProcessedTotal.WithLabelValues("discord", "shutdown", "true")); v != 1 {
		t.Errorf("expected 1 shutdown, got %f", v)
	}
}
