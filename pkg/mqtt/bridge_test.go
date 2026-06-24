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

// --- Bridge lifecycle tests (with mock MQTT client) ---

func newTestBridge(t *testing.T, client *mockClient, collector *messageCollector) (*Bridge, *BridgeMetrics) {
	t.Helper()
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	cfg := Config{
		Enabled:          true,
		Broker:           "tcp://test:1883",
		ClientID:         "test",
		Topics:           []string{"home/#", "ai/#"},
		DiscordChannels:  []string{"ch1"},
		FlushSeconds:     5,
		MaxBuffer:        100,
		MaxPayloadBytes:  256,
		MaxPostsPerFlush: 5,
	}
	b := NewBridgeWithFactory(cfg, collector.post, m, mockFactory(client))
	return b, m
}

func TestNewBridge_SetsDefaults(t *testing.T) {
	collector := &messageCollector{}
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	cfg := Config{
		Enabled:          true,
		MaxBuffer:        50,
		MaxPayloadBytes:  128,
		MaxPostsPerFlush: 3,
	}
	b := NewBridge(cfg, collector.post, m)
	if b.buffer == nil {
		t.Error("expected buffer to be initialized")
	}
	if !b.enabled {
		t.Error("expected enabled=true from config")
	}
	if b.clientFactory == nil {
		t.Error("expected default client factory")
	}
}

func TestBridge_Start_ConnectsAndSubscribes(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)

	if err := b.Start(); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	defer b.Stop()

	if !client.IsConnected() {
		t.Error("expected client to be connected after Start()")
	}

	subs := client.getSubscriptions()
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(subs))
	}
	if subs[0] != "home/#" || subs[1] != "ai/#" {
		t.Errorf("expected [home/# ai/#], got %v", subs)
	}

	status := b.Status()
	if !status.Connected {
		t.Error("expected connected=true in status")
	}
	if !status.Enabled {
		t.Error("expected enabled=true in status")
	}
}

func TestBridge_Start_ConnectFailure_NotifiesDiscord(t *testing.T) {
	client := newMockClient(fmt.Errorf("connection refused"))
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)

	if err := b.Start(); err != nil {
		t.Fatalf("Start() should not return error even on connect failure: %v", err)
	}
	defer b.Stop()

	time.Sleep(50 * time.Millisecond)

	msgs := collector.getMessages()
	found := false
	for _, m := range msgs {
		if strings.Contains(m.msg, "failed to connect") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Discord notification about connection failure")
	}
}

func TestBridge_Stop_DisconnectsClient(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, m := newTestBridge(t, client, collector)

	_ = b.Start()
	b.Stop()

	if client.IsConnected() {
		t.Error("expected client disconnected after Stop()")
	}
	if client.getDisconnectCalls() != 1 {
		t.Errorf("expected 1 disconnect call, got %d", client.getDisconnectCalls())
	}
	if v := testutil.ToFloat64(m.Connected); v != 0 {
		t.Errorf("expected connected metric=0 after stop, got %f", v)
	}
}

func TestBridge_Stop_DoubleStopSafe(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)

	_ = b.Start()
	b.Stop()
	b.Stop()

	if client.getDisconnectCalls() != 1 {
		t.Errorf("expected 1 disconnect call on double stop, got %d", client.getDisconnectCalls())
	}
}

func TestBridge_MessageHandler_BuffersMessage(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, m := newTestBridge(t, client, collector)

	_ = b.Start()
	defer b.Stop()

	msg := &mockMessage{topic: "home/temp", payload: []byte("22.5")}
	b.messageHandler(nil, msg)

	if b.buffer.Len() != 1 {
		t.Errorf("expected 1 buffered message, got %d", b.buffer.Len())
	}
	if v := testutil.ToFloat64(m.MessagesReceived); v != 1 {
		t.Errorf("expected 1 received metric, got %f", v)
	}
}

func TestBridge_MessageHandler_DisabledDrops(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, m := newTestBridge(t, client, collector)

	_ = b.Start()
	defer b.Stop()

	b.Disable()
	msg := &mockMessage{topic: "home/temp", payload: []byte("22.5")}
	b.messageHandler(nil, msg)

	if b.buffer.Len() != 0 {
		t.Errorf("expected 0 buffered messages when disabled, got %d", b.buffer.Len())
	}
	if v := testutil.ToFloat64(m.MessagesReceived); v != 0 {
		t.Errorf("expected 0 received metric when disabled, got %f", v)
	}
}

func TestBridge_MessageHandler_TruncatesPayload(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)
	b.config.MaxPayloadBytes = 10

	_ = b.Start()
	defer b.Stop()

	msg := &mockMessage{topic: "t", payload: []byte("this is a long payload that should be truncated")}
	b.messageHandler(nil, msg)

	msgs := b.buffer.Flush()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if len(msgs[0].Payload) > 10+len("…") {
		t.Errorf("expected payload truncated to ~10 chars, got %d: %q", len(msgs[0].Payload), msgs[0].Payload)
	}
}

func TestBridge_MessageHandler_RecordsDropMetric(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, m := newTestBridge(t, client, collector)
	b.config.MaxBuffer = 2
	b.buffer = NewBuffer(2)

	_ = b.Start()
	defer b.Stop()

	for i := range 5 {
		msg := &mockMessage{topic: fmt.Sprintf("t/%d", i), payload: []byte("x")}
		b.messageHandler(nil, msg)
	}

	if v := testutil.ToFloat64(m.MessagesDropped); v != 3 {
		t.Errorf("expected 3 dropped, got %f", v)
	}
}

func TestBridge_Flush_PostsToDiscord(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, m := newTestBridge(t, client, collector)

	_ = b.Start()
	defer b.Stop()

	b.buffer.Add("home/temp", "22.5", time.Now())
	b.buffer.Add("home/humid", "55", time.Now())

	b.flush()

	msgs := collector.getMessages()
	if len(msgs) == 0 {
		t.Fatal("expected flush to post to Discord")
	}
	if msgs[0].channel != "ch1" {
		t.Errorf("expected channel 'ch1', got %q", msgs[0].channel)
	}
	if !strings.Contains(msgs[0].msg, "home/temp") {
		t.Errorf("expected message to contain topic, got %q", msgs[0].msg)
	}
	if v := testutil.ToFloat64(m.MessagesForwarded); v == 0 {
		t.Error("expected forwarded metric > 0")
	}
}

func TestBridge_Flush_WhenDisabled_NoOp(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)

	_ = b.Start()
	defer b.Stop()

	b.Disable()
	b.buffer.Add("home/temp", "22.5", time.Now())

	b.flush()

	msgs := collector.getMessages()
	if len(msgs) != 0 {
		t.Errorf("expected no flush when disabled, got %d messages", len(msgs))
	}
}

func TestBridge_Flush_EmptyBuffer_NoOp(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)

	_ = b.Start()
	defer b.Stop()

	b.flush()

	msgs := collector.getMessages()
	if len(msgs) != 0 {
		t.Errorf("expected no flush for empty buffer, got %d messages", len(msgs))
	}
}

func TestBridge_Flush_PostError_DoesNotPanic(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)
	b.postFunc = collector.postErr

	_ = b.Start()
	defer b.Stop()

	b.buffer.Add("home/temp", "22.5", time.Now())
	b.flush()
}

func TestBridge_NotifyDiscord_PostsToAllChannels(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)
	b.config.DiscordChannels = []string{"ch1", "ch2"}

	b.notifyDiscord("test alert")

	msgs := collector.getMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (one per channel), got %d", len(msgs))
	}
	if msgs[0].channel != "ch1" || msgs[1].channel != "ch2" {
		t.Errorf("expected channels [ch1, ch2], got [%s, %s]", msgs[0].channel, msgs[1].channel)
	}
}

func TestBridge_OnConnectionLost_SetsDisconnected(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, m := newTestBridge(t, client, collector)

	_ = b.Start()
	defer b.Stop()

	b.onConnectionLost(nil, fmt.Errorf("network error"))

	time.Sleep(50 * time.Millisecond)

	status := b.Status()
	if status.Connected {
		t.Error("expected connected=false after connection lost")
	}
	if v := testutil.ToFloat64(m.Connected); v != 0 {
		t.Errorf("expected connected metric=0, got %f", v)
	}

	msgs := collector.getMessages()
	found := false
	for _, msg := range msgs {
		if strings.Contains(msg.msg, "connection lost") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Discord notification about connection loss")
	}
}

func TestBridge_OnConnect_SetsConnectedAndSubscribes(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, m := newTestBridge(t, client, collector)

	_ = b.Start()
	defer b.Stop()

	subsBefore := len(client.getSubscriptions())
	b.onConnect(nil)

	if v := testutil.ToFloat64(m.Connected); v != 1 {
		t.Errorf("expected connected metric=1 after onConnect, got %f", v)
	}

	subsAfter := len(client.getSubscriptions())
	if subsAfter-subsBefore != 2 {
		t.Errorf("expected 2 new subscriptions on reconnect, got %d", subsAfter-subsBefore)
	}
}

func TestBridge_FlushLoop_StopsOnStopCh(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)
	b.config.FlushSeconds = 1

	_ = b.Start()

	time.Sleep(50 * time.Millisecond)
	b.Stop()
}

func TestBridge_Subscribe_RecordsTopics(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)
	b.client = client

	b.subscribe()

	subs := client.getSubscriptions()
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(subs))
	}
}

func TestBridge_Subscribe_HandleError(t *testing.T) {
	client := newMockClient(nil)
	client.subscribeErr = fmt.Errorf("subscribe failed")
	collector := &messageCollector{}
	b, _ := newTestBridge(t, client, collector)
	b.client = client

	b.subscribe()
}

func TestNewBridgeWithFactory_UsesCustomFactory(t *testing.T) {
	client := newMockClient(nil)
	collector := &messageCollector{}
	reg := prometheus.NewRegistry()
	m := NewBridgeMetrics(reg)
	cfg := Config{
		Enabled:          true,
		Broker:           "tcp://test:1883",
		MaxBuffer:        10,
		MaxPayloadBytes:  128,
		MaxPostsPerFlush: 3,
		FlushSeconds:     5,
	}
	b := NewBridgeWithFactory(cfg, collector.post, m, mockFactory(client))
	if b.clientFactory == nil {
		t.Error("expected custom factory to be set")
	}
}
