package mqtt

import (
	"fmt"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type mockToken struct {
	err error
}

func (t *mockToken) Wait() bool                       { return true }
func (t *mockToken) WaitTimeout(d time.Duration) bool { return true }
func (t *mockToken) Done() <-chan struct{}            { ch := make(chan struct{}); close(ch); return ch }
func (t *mockToken) Error() error                     { return t.err }

type mockClient struct {
	mu              sync.Mutex
	connected       bool
	connectErr      error
	subscribeErr    error
	subscriptions   []string
	disconnectCalls int
}

func newMockClient(connectErr error) *mockClient {
	return &mockClient{connectErr: connectErr}
}

func (c *mockClient) Connect() pahomqtt.Token {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connectErr == nil {
		c.connected = true
	}
	return &mockToken{err: c.connectErr}
}

func (c *mockClient) Disconnect(quiesce uint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	c.disconnectCalls++
}

func (c *mockClient) Subscribe(topic string, qos byte, callback pahomqtt.MessageHandler) pahomqtt.Token {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions = append(c.subscriptions, topic)
	return &mockToken{err: c.subscribeErr}
}

func (c *mockClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *mockClient) getSubscriptions() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.subscriptions))
	copy(out, c.subscriptions)
	return out
}

func (c *mockClient) getDisconnectCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.disconnectCalls
}

type mockMessage struct {
	topic   string
	payload []byte
}

func (m *mockMessage) Duplicate() bool   { return false }
func (m *mockMessage) Qos() byte         { return 0 }
func (m *mockMessage) Retained() bool    { return false }
func (m *mockMessage) Topic() string     { return m.topic }
func (m *mockMessage) MessageID() uint16 { return 0 }
func (m *mockMessage) Payload() []byte   { return m.payload }
func (m *mockMessage) Ack()              {}

func mockFactory(client *mockClient) ClientFactory {
	return func(cfg Config, onConnLost pahomqtt.ConnectionLostHandler, onConnect pahomqtt.OnConnectHandler) MQTTClient {
		return client
	}
}

var _ MQTTClient = (*mockClient)(nil)
var _ pahomqtt.Message = (*mockMessage)(nil)

type countingFactory struct {
	client *mockClient
	mu     sync.Mutex
	calls  int
}

func newCountingFactory(client *mockClient) *countingFactory {
	return &countingFactory{client: client}
}

func (f *countingFactory) factory() ClientFactory {
	return func(cfg Config, onConnLost pahomqtt.ConnectionLostHandler, onConnect pahomqtt.OnConnectHandler) MQTTClient {
		f.mu.Lock()
		f.calls++
		f.mu.Unlock()
		return f.client
	}
}

func (f *countingFactory) getCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type messageCollector struct {
	mu       sync.Mutex
	messages []struct{ channel, msg string }
}

func (c *messageCollector) post(channelID, msg string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, struct{ channel, msg string }{channelID, msg})
	return nil
}

func (c *messageCollector) postErr(channelID, msg string) error {
	return fmt.Errorf("post failed")
}

func (c *messageCollector) getMessages() []struct{ channel, msg string } {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]struct{ channel, msg string }, len(c.messages))
	copy(out, c.messages)
	return out
}
