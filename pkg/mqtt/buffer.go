package mqtt

import (
	"sync"
	"time"
)

type Message struct {
	Topic     string
	Payload   string
	Timestamp time.Time
}

type Buffer struct {
	mu       sync.Mutex
	messages []Message
	maxSize  int
}

func NewBuffer(maxSize int) *Buffer {
	return &Buffer{
		messages: make([]Message, 0, maxSize),
		maxSize:  maxSize,
	}
}

func (b *Buffer) Add(topic, payload string, ts time.Time) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	dropped := 0
	for len(b.messages) >= b.maxSize {
		b.messages = b.messages[1:]
		dropped++
	}

	b.messages = append(b.messages, Message{
		Topic:     topic,
		Payload:   payload,
		Timestamp: ts,
	})

	return dropped
}

func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.messages)
}

func (b *Buffer) Flush() []Message {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.messages) == 0 {
		return nil
	}

	out := make([]Message, len(b.messages))
	copy(out, b.messages)
	b.messages = b.messages[:0]
	return out
}

func TruncatePayload(payload string, maxBytes int) string {
	if len(payload) <= maxBytes {
		return payload
	}
	return payload[:maxBytes] + "…"
}
