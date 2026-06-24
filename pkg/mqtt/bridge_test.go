package mqtt

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// --- Config validation tests ---

func TestValidateConfig_Disabled_NoError(t *testing.T) {
	cfg := Config{Enabled: false}
	if err := ValidateConfig(cfg, true); err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

func TestValidateConfig_Enabled_NoDiscord_Error(t *testing.T) {
	cfg := Config{Enabled: true, Broker: "tcp://broker:1883"}
	err := ValidateConfig(cfg, false)
	if err == nil {
		t.Fatal("expected error when MQTT enabled without Discord")
	}
	if !strings.Contains(err.Error(), "Discord") {
		t.Errorf("expected Discord-related error, got %q", err.Error())
	}
}

func TestValidateConfig_Enabled_NoBroker_Error(t *testing.T) {
	cfg := Config{Enabled: true, Broker: ""}
	err := ValidateConfig(cfg, true)
	if err == nil {
		t.Fatal("expected error when broker is empty")
	}
}

func TestValidateConfig_FlushSeconds_TooLow(t *testing.T) {
	cfg := Config{
		Enabled:      true,
		Broker:       "tcp://broker:1883",
		FlushSeconds: 4,
	}
	err := ValidateConfig(cfg, true)
	if err == nil {
		t.Fatal("expected error for flush_seconds < 5")
	}
	if !strings.Contains(err.Error(), "5") {
		t.Errorf("expected error mentioning bounds, got %q", err.Error())
	}
}

func TestValidateConfig_FlushSeconds_TooHigh(t *testing.T) {
	cfg := Config{
		Enabled:      true,
		Broker:       "tcp://broker:1883",
		FlushSeconds: 86401,
	}
	err := ValidateConfig(cfg, true)
	if err == nil {
		t.Fatal("expected error for flush_seconds > 86400")
	}
}

func TestValidateConfig_FlushSeconds_MinBound(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		Broker:           "tcp://broker:1883",
		FlushSeconds:     5,
		MaxBuffer:        100,
		MaxPayloadBytes:  256,
		MaxPostsPerFlush: 5,
	}
	if err := ValidateConfig(cfg, true); err != nil {
		t.Errorf("expected flush_seconds=5 to be valid, got %v", err)
	}
}

func TestValidateConfig_FlushSeconds_MaxBound(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		Broker:           "tcp://broker:1883",
		FlushSeconds:     86400,
		MaxBuffer:        100,
		MaxPayloadBytes:  256,
		MaxPostsPerFlush: 5,
	}
	if err := ValidateConfig(cfg, true); err != nil {
		t.Errorf("expected flush_seconds=86400 to be valid, got %v", err)
	}
}

func TestValidateConfig_EmptyTopics_Allowed(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		Broker:           "tcp://broker:1883",
		FlushSeconds:     30,
		Topics:           []string{},
		MaxBuffer:        100,
		MaxPayloadBytes:  256,
		MaxPostsPerFlush: 5,
	}
	if err := ValidateConfig(cfg, true); err != nil {
		t.Errorf("expected empty topics to be allowed, got %v", err)
	}
}

func TestValidateConfig_MaxBuffer_Zero_Error(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		Broker:           "tcp://broker:1883",
		FlushSeconds:     30,
		MaxBuffer:        0,
		MaxPayloadBytes:  256,
		MaxPostsPerFlush: 5,
	}
	err := ValidateConfig(cfg, true)
	if err == nil {
		t.Fatal("expected error for max_buffer=0")
	}
}

func TestValidateConfig_MaxPayloadBytes_Zero_Error(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		Broker:           "tcp://broker:1883",
		FlushSeconds:     30,
		MaxBuffer:        100,
		MaxPayloadBytes:  0,
		MaxPostsPerFlush: 5,
	}
	err := ValidateConfig(cfg, true)
	if err == nil {
		t.Fatal("expected error for max_payload_bytes=0")
	}
}

func TestValidateConfig_MaxPostsPerFlush_Zero_Error(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		Broker:           "tcp://broker:1883",
		FlushSeconds:     30,
		MaxBuffer:        100,
		MaxPayloadBytes:  256,
		MaxPostsPerFlush: 0,
	}
	err := ValidateConfig(cfg, true)
	if err == nil {
		t.Fatal("expected error for max_posts_per_flush=0")
	}
}

// --- Payload truncation tests ---

func TestTruncatePayload_Short(t *testing.T) {
	got := TruncatePayload("hello", 256)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestTruncatePayload_ExactLimit(t *testing.T) {
	s := strings.Repeat("a", 256)
	got := TruncatePayload(s, 256)
	if got != s {
		t.Errorf("expected exact-length string unchanged")
	}
}

func TestTruncatePayload_OverLimit(t *testing.T) {
	s := strings.Repeat("a", 300)
	got := TruncatePayload(s, 256)
	if len(got) != 256+len("…") {
		t.Errorf("expected truncated length %d, got %d", 256+len("…"), len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected truncated string to end with ellipsis, got %q", got)
	}
}

func TestTruncatePayload_Empty(t *testing.T) {
	got := TruncatePayload("", 256)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// --- Buffer tests ---

func TestBuffer_Add_Single(t *testing.T) {
	b := NewBuffer(10)
	b.Add("home/temp", "22.5", time.Now())
	if b.Len() != 1 {
		t.Errorf("expected 1 message, got %d", b.Len())
	}
}

func TestBuffer_Add_Multiple(t *testing.T) {
	b := NewBuffer(10)
	for i := range 5 {
		b.Add(fmt.Sprintf("topic/%d", i), "payload", time.Now())
	}
	if b.Len() != 5 {
		t.Errorf("expected 5 messages, got %d", b.Len())
	}
}

func TestBuffer_Overflow_DropsOldest(t *testing.T) {
	b := NewBuffer(3)
	now := time.Now()
	b.Add("topic/0", "first", now)
	b.Add("topic/1", "second", now.Add(time.Second))
	b.Add("topic/2", "third", now.Add(2*time.Second))
	dropped := b.Add("topic/3", "fourth", now.Add(3*time.Second))

	if dropped != 1 {
		t.Errorf("expected 1 dropped, got %d", dropped)
	}
	if b.Len() != 3 {
		t.Errorf("expected 3 messages after overflow, got %d", b.Len())
	}

	msgs := b.Flush()
	if msgs[0].Payload != "second" {
		t.Errorf("expected oldest dropped; first message should be 'second', got %q", msgs[0].Payload)
	}
	if msgs[2].Payload != "fourth" {
		t.Errorf("expected newest kept; last message should be 'fourth', got %q", msgs[2].Payload)
	}
}

func TestBuffer_Overflow_MultipleDrop(t *testing.T) {
	b := NewBuffer(2)
	now := time.Now()

	b.Add("t/0", "a", now)
	b.Add("t/1", "b", now.Add(time.Second))
	dropped1 := b.Add("t/2", "c", now.Add(2*time.Second))
	dropped2 := b.Add("t/3", "d", now.Add(3*time.Second))

	if dropped1 != 1 || dropped2 != 1 {
		t.Errorf("expected 1 drop each, got %d and %d", dropped1, dropped2)
	}

	msgs := b.Flush()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Payload != "c" || msgs[1].Payload != "d" {
		t.Errorf("expected newest messages [c, d], got [%q, %q]", msgs[0].Payload, msgs[1].Payload)
	}
}

func TestBuffer_Flush_EmptiesBuffer(t *testing.T) {
	b := NewBuffer(10)
	b.Add("topic/1", "data", time.Now())
	b.Add("topic/2", "data", time.Now())

	msgs := b.Flush()
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages from flush, got %d", len(msgs))
	}
	if b.Len() != 0 {
		t.Errorf("expected buffer empty after flush, got %d", b.Len())
	}
}

func TestBuffer_Flush_Empty(t *testing.T) {
	b := NewBuffer(10)
	msgs := b.Flush()
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages from empty flush, got %d", len(msgs))
	}
}

func TestBuffer_Concurrent(t *testing.T) {
	b := NewBuffer(100)
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			b.Add(fmt.Sprintf("topic/%d", n), "data", time.Now())
		}(i)
	}
	wg.Wait()
	if b.Len() != 50 {
		t.Errorf("expected 50 messages after concurrent adds, got %d", b.Len())
	}
}

// --- Digest formatting tests ---

func TestFormatDigest_SingleMessage(t *testing.T) {
	msgs := []Message{
		{Topic: "home/temp", Payload: "22.5", Timestamp: time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)},
	}
	chunks := FormatDigest(msgs, 2000, 5)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], "home/temp") {
		t.Errorf("expected topic in digest, got %q", chunks[0])
	}
	if !strings.Contains(chunks[0], "22.5") {
		t.Errorf("expected payload in digest, got %q", chunks[0])
	}
}

func TestFormatDigest_RateCap(t *testing.T) {
	msgs := make([]Message, 20)
	now := time.Now()
	for i := range 20 {
		msgs[i] = Message{
			Topic:     fmt.Sprintf("topic/%d", i),
			Payload:   strings.Repeat("x", 50),
			Timestamp: now,
		}
	}
	// Use a small chunk size so messages span many chunks, triggering the rate cap
	chunks := FormatDigest(msgs, 100, 3)
	if len(chunks) > 3 {
		t.Errorf("expected at most 3 chunks (rate cap), got %d", len(chunks))
	}
	lastChunk := chunks[len(chunks)-1]
	if !strings.Contains(lastChunk, "suppressed") {
		t.Errorf("expected suppression notice in last chunk, got %q", lastChunk)
	}
}

func TestFormatDigest_Empty(t *testing.T) {
	chunks := FormatDigest(nil, 2000, 5)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty messages, got %d", len(chunks))
	}
}

func TestFormatDigest_LongPayloadChunking(t *testing.T) {
	msgs := make([]Message, 100)
	now := time.Now()
	for i := range 100 {
		msgs[i] = Message{
			Topic:     fmt.Sprintf("long/topic/%d", i),
			Payload:   strings.Repeat("x", 50),
			Timestamp: now,
		}
	}
	chunks := FormatDigest(msgs, 200, 100)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for long digest, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if len(chunk) > 2200 {
			t.Errorf("chunk exceeds Discord limit (2200 chars safety), got %d", len(chunk))
		}
	}
}

// --- Metrics tests ---

func TestBridgeMetrics_Registration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	if m == nil {
		t.Fatal("expected non-nil BridgeMetrics")
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather: %v", err)
	}
	if len(families) == 0 {
		t.Error("expected registered metrics families")
	}
}

func TestBridgeMetrics_RecordReceived(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	m.RecordReceived()
	m.RecordReceived()

	if v := testutil.ToFloat64(m.MessagesReceived); v != 2 {
		t.Errorf("expected 2 received, got %f", v)
	}
}

func TestBridgeMetrics_RecordForwarded(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	m.RecordForwarded(5)

	if v := testutil.ToFloat64(m.MessagesForwarded); v != 5 {
		t.Errorf("expected 5 forwarded, got %f", v)
	}
}

func TestBridgeMetrics_RecordDropped(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	m.RecordDropped(3)

	if v := testutil.ToFloat64(m.MessagesDropped); v != 3 {
		t.Errorf("expected 3 dropped, got %f", v)
	}
}

func TestBridgeMetrics_RecordSuppressed(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	m.RecordSuppressed(7)

	if v := testutil.ToFloat64(m.MessagesSuppressed); v != 7 {
		t.Errorf("expected 7 suppressed, got %f", v)
	}
}

func TestBridgeMetrics_SetBufferDepth(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	m.SetBufferDepth(42)

	if v := testutil.ToFloat64(m.BufferDepth); v != 42 {
		t.Errorf("expected buffer depth 42, got %f", v)
	}
}

func TestBridgeMetrics_SetEnabled(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	m.SetEnabled(true)
	if v := testutil.ToFloat64(m.BridgeEnabled); v != 1 {
		t.Errorf("expected enabled=1, got %f", v)
	}

	m.SetEnabled(false)
	if v := testutil.ToFloat64(m.BridgeEnabled); v != 0 {
		t.Errorf("expected enabled=0, got %f", v)
	}
}

func TestBridgeMetrics_SetConnected(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	m.SetConnected(true)
	if v := testutil.ToFloat64(m.Connected); v != 1 {
		t.Errorf("expected connected=1, got %f", v)
	}

	m.SetConnected(false)
	if v := testutil.ToFloat64(m.Connected); v != 0 {
		t.Errorf("expected connected=0, got %f", v)
	}
}

// --- Bridge lifecycle tests (with mocks) ---

func TestBridge_DisabledByDefault(t *testing.T) {
	b := &Bridge{}
	if b.IsEnabled() {
		t.Error("expected bridge disabled by default")
	}
}

func TestBridge_Enable_Disable(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	b := &Bridge{metrics: m}
	b.Enable()
	if !b.IsEnabled() {
		t.Error("expected bridge enabled after Enable()")
	}
	b.Disable()
	if b.IsEnabled() {
		t.Error("expected bridge disabled after Disable()")
	}
}

func TestBridge_Status(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	buf := NewBuffer(100)
	b := &Bridge{
		metrics: m,
		buffer:  buf,
		config: Config{
			Topics: []string{"home/#", "ai/#"},
		},
	}
	b.Enable()

	status := b.Status()
	if !status.Enabled {
		t.Error("expected enabled=true in status")
	}
	if status.BufferDepth != 0 {
		t.Errorf("expected buffer depth 0, got %d", status.BufferDepth)
	}
	if len(status.Topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(status.Topics))
	}
}

func TestBridge_Status_WithBufferedMessages(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	buf := NewBuffer(100)
	buf.Add("home/temp", "22", time.Now())
	buf.Add("home/humid", "55", time.Now())
	b := &Bridge{
		metrics: m,
		buffer:  buf,
		config:  Config{Topics: []string{"home/#"}},
	}

	status := b.Status()
	if status.BufferDepth != 2 {
		t.Errorf("expected buffer depth 2, got %d", status.BufferDepth)
	}
}
