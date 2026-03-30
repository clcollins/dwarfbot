package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics holds all Prometheus metrics for the application.
type Metrics struct {
	Registry *prometheus.Registry

	// Connection metrics
	PlatformConnected                 *prometheus.GaugeVec
	PlatformConnectionAttemptsTotal   *prometheus.CounterVec
	PlatformDisconnectionsTotal       *prometheus.CounterVec
	PlatformConnectionDurationSeconds *prometheus.HistogramVec

	// Config metrics
	PlatformTokenPresent *prometheus.GaugeVec
	PlatformConfigured   *prometheus.GaugeVec

	// Message metrics
	MessagesReceivedTotal  *prometheus.CounterVec
	MessagesSentTotal      *prometheus.CounterVec
	CommandsProcessedTotal *prometheus.CounterVec

	// App metrics
	Info *prometheus.GaugeVec
}

// New creates and registers all metrics on a new registry.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{Registry: reg}

	m.PlatformConnected = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dwarfbot_platform_connected",
			Help: "Whether platform is currently connected (1) or not (0).",
		},
		[]string{"platform"},
	)

	m.PlatformConnectionAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dwarfbot_platform_connection_attempts_total",
			Help: "Total connection attempts by platform and result.",
		},
		[]string{"platform", "result"},
	)

	m.PlatformDisconnectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dwarfbot_platform_disconnections_total",
			Help: "Total disconnections by platform and reason.",
		},
		[]string{"platform", "reason"},
	)

	m.PlatformConnectionDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dwarfbot_platform_connection_duration_seconds",
			Help:    "Duration of platform connections before dropping.",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		},
		[]string{"platform"},
	)

	m.PlatformTokenPresent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dwarfbot_platform_token_present",
			Help: "Whether a token is configured for the platform.",
		},
		[]string{"platform"},
	)

	m.PlatformConfigured = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dwarfbot_platform_configured",
			Help: "Whether platform is fully configured (token + channels).",
		},
		[]string{"platform"},
	)

	m.MessagesReceivedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dwarfbot_messages_received_total",
			Help: "Total messages received by platform.",
		},
		[]string{"platform"},
	)

	m.MessagesSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dwarfbot_messages_sent_total",
			Help: "Total messages sent by platform and result.",
		},
		[]string{"platform", "result"},
	)

	m.CommandsProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dwarfbot_commands_processed_total",
			Help: "Total commands processed by platform, command name, and admin status.",
		},
		[]string{"platform", "command", "admin"},
	)

	m.Info = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dwarfbot_info",
			Help: "Build info metric, always 1.",
		},
		[]string{"version", "go_version"},
	)

	reg.MustRegister(
		m.PlatformConnected,
		m.PlatformConnectionAttemptsTotal,
		m.PlatformDisconnectionsTotal,
		m.PlatformConnectionDurationSeconds,
		m.PlatformTokenPresent,
		m.PlatformConfigured,
		m.MessagesReceivedTotal,
		m.MessagesSentTotal,
		m.CommandsProcessedTotal,
		m.Info,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return m
}

// Init sets the static info gauge and registers the uptime metric.
func (m *Metrics) Init(version string, startTime time.Time) {
	m.Info.WithLabelValues(version, runtime.Version()).Set(1)
	m.Registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "dwarfbot_uptime_seconds",
			Help: "Time since process start in seconds.",
		},
		func() float64 { return time.Since(startTime).Seconds() },
	))
}

// SetConfigMetrics reports which platforms have tokens and are fully configured.
func (m *Metrics) SetConfigMetrics(twitchToken, discordToken string, twitchChannels, discordChannels []string) {
	setPresent := func(platform, token string, channels []string) {
		if token != "" {
			m.PlatformTokenPresent.WithLabelValues(platform).Set(1)
		} else {
			m.PlatformTokenPresent.WithLabelValues(platform).Set(0)
		}
		if token != "" && len(channels) > 0 {
			m.PlatformConfigured.WithLabelValues(platform).Set(1)
		} else {
			m.PlatformConfigured.WithLabelValues(platform).Set(0)
		}
	}
	setPresent("twitch", twitchToken, twitchChannels)
	setPresent("discord", discordToken, discordChannels)

	// Initialize connected gauge to 0 so alerting rules work even
	// if a platform never connects.
	m.PlatformConnected.WithLabelValues("twitch").Set(0)
	m.PlatformConnected.WithLabelValues("discord").Set(0)
}
