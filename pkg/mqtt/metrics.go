package mqtt

import "github.com/prometheus/client_golang/prometheus"

type BridgeMetrics struct {
	MessagesReceived   prometheus.Counter
	MessagesForwarded  prometheus.Counter
	MessagesDropped    prometheus.Counter
	MessagesSuppressed prometheus.Counter
	BufferDepth        prometheus.Gauge
	BridgeEnabled      prometheus.Gauge
	Connected          prometheus.Gauge
}

func NewBridgeMetrics(reg prometheus.Registerer) *BridgeMetrics {
	m := &BridgeMetrics{
		MessagesReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dwarfbot_mqtt_messages_received_total",
			Help: "Total MQTT messages received by the bridge.",
		}),
		MessagesForwarded: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dwarfbot_mqtt_messages_forwarded_total",
			Help: "Total MQTT messages forwarded to Discord.",
		}),
		MessagesDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dwarfbot_mqtt_messages_dropped_total",
			Help: "Total MQTT messages dropped due to buffer overflow.",
		}),
		MessagesSuppressed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dwarfbot_mqtt_messages_suppressed_total",
			Help: "Total MQTT messages suppressed by the outbound rate cap.",
		}),
		BufferDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwarfbot_mqtt_buffer_depth",
			Help: "Current number of messages in the digest buffer.",
		}),
		BridgeEnabled: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwarfbot_mqtt_bridge_enabled",
			Help: "Whether the MQTT bridge is enabled (1) or disabled (0).",
		}),
		Connected: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwarfbot_mqtt_connected",
			Help: "Whether the MQTT client is connected to the broker (1) or not (0).",
		}),
	}

	reg.MustRegister(
		m.MessagesReceived,
		m.MessagesForwarded,
		m.MessagesDropped,
		m.MessagesSuppressed,
		m.BufferDepth,
		m.BridgeEnabled,
		m.Connected,
	)

	return m
}

func (m *BridgeMetrics) RecordReceived() {
	m.MessagesReceived.Inc()
}

func (m *BridgeMetrics) RecordForwarded(n int) {
	m.MessagesForwarded.Add(float64(n))
}

func (m *BridgeMetrics) RecordDropped(n int) {
	m.MessagesDropped.Add(float64(n))
}

func (m *BridgeMetrics) RecordSuppressed(n int) {
	m.MessagesSuppressed.Add(float64(n))
}

func (m *BridgeMetrics) SetBufferDepth(n int) {
	m.BufferDepth.Set(float64(n))
}

func (m *BridgeMetrics) SetEnabled(enabled bool) {
	if enabled {
		m.BridgeEnabled.Set(1)
	} else {
		m.BridgeEnabled.Set(0)
	}
}

func (m *BridgeMetrics) SetConnected(connected bool) {
	if connected {
		m.Connected.Set(1)
	} else {
		m.Connected.Set(0)
	}
}
