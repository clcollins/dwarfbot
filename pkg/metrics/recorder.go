package metrics

import "time"

// Recorder implements PlatformMetrics using Prometheus metrics.
type Recorder struct {
	metrics *Metrics
}

// NewRecorder creates a Recorder backed by the given Metrics.
func NewRecorder(m *Metrics) *Recorder {
	return &Recorder{metrics: m}
}

func (r *Recorder) RecordConnectionAttempt(platform, result string) {
	r.metrics.PlatformConnectionAttemptsTotal.WithLabelValues(platform, result).Inc()
}

func (r *Recorder) RecordConnected(platform string) {
	r.metrics.PlatformConnected.WithLabelValues(platform).Set(1)
}

func (r *Recorder) RecordDisconnected(platform, reason string) {
	r.metrics.PlatformConnected.WithLabelValues(platform).Set(0)
	r.metrics.PlatformDisconnectionsTotal.WithLabelValues(platform, reason).Inc()
}

func (r *Recorder) RecordConnectionDuration(platform string, duration time.Duration) {
	r.metrics.PlatformConnectionDurationSeconds.WithLabelValues(platform).Observe(duration.Seconds())
}

func (r *Recorder) RecordMessageReceived(platform string) {
	r.metrics.MessagesReceivedTotal.WithLabelValues(platform).Inc()
}

func (r *Recorder) RecordMessageSent(platform, result string) {
	r.metrics.MessagesSentTotal.WithLabelValues(platform, result).Inc()
}

func (r *Recorder) RecordCommandProcessed(platform, command, admin string) {
	r.metrics.CommandsProcessedTotal.WithLabelValues(platform, command, admin).Inc()
}
