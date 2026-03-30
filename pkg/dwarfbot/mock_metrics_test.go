package dwarfbot

import (
	"sync"
	"time"
)

// mockMetricsRecorder records all metric calls for test assertions.
type mockMetricsRecorder struct {
	mu                  sync.Mutex
	connectionAttempts  []mockAttempt
	connected           []string
	disconnected        []mockDisconnect
	connectionDurations []mockDuration
	messagesReceived    []string
	messagesSent        []mockSent
	commandsProcessed   []mockCommand
}

type mockAttempt struct {
	platform, result string
}

type mockDisconnect struct {
	platform, reason string
}

type mockDuration struct {
	platform string
	duration time.Duration
}

type mockSent struct {
	platform, result string
}

type mockCommand struct {
	platform, command, admin string
}

func newMockMetricsRecorder() *mockMetricsRecorder {
	return &mockMetricsRecorder{}
}

func (m *mockMetricsRecorder) RecordConnectionAttempt(platform, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectionAttempts = append(m.connectionAttempts, mockAttempt{platform, result})
}

func (m *mockMetricsRecorder) RecordConnected(platform string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = append(m.connected, platform)
}

func (m *mockMetricsRecorder) RecordDisconnected(platform, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnected = append(m.disconnected, mockDisconnect{platform, reason})
}

func (m *mockMetricsRecorder) RecordConnectionDuration(platform string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectionDurations = append(m.connectionDurations, mockDuration{platform, duration})
}

func (m *mockMetricsRecorder) RecordMessageReceived(platform string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messagesReceived = append(m.messagesReceived, platform)
}

func (m *mockMetricsRecorder) RecordMessageSent(platform, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messagesSent = append(m.messagesSent, mockSent{platform, result})
}

func (m *mockMetricsRecorder) RecordCommandProcessed(platform, command, admin string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commandsProcessed = append(m.commandsProcessed, mockCommand{platform, command, admin})
}

// Verify mockMetricsRecorder satisfies PlatformMetrics at compile time
var _ PlatformMetrics = (*mockMetricsRecorder)(nil)
