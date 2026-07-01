package mqtt

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type PostFunc func(channelID, msg string) error

type BridgeStatus struct {
	Enabled     bool
	Connected   bool
	BufferDepth int
	Topics      []string
}

type Bridge struct {
	config         Config
	buffer         *Buffer
	metrics        *BridgeMetrics
	postFunc       PostFunc
	clientFactory  ClientFactory
	client         MQTTClient
	mu             sync.Mutex
	enabled        bool
	connected      bool
	stopCh         chan struct{}
	stopped        bool
	reconnecting   bool
	lastNotifyTime time.Time
	nowFunc        func() time.Time
}

func NewBridge(cfg Config, postFunc PostFunc, metrics *BridgeMetrics) *Bridge {
	return &Bridge{
		config:        cfg,
		buffer:        NewBuffer(cfg.MaxBuffer),
		metrics:       metrics,
		postFunc:      postFunc,
		clientFactory: DefaultClientFactory,
		enabled:       cfg.Enabled,
		nowFunc:       time.Now,
	}
}

func NewBridgeWithFactory(cfg Config, postFunc PostFunc, metrics *BridgeMetrics, factory ClientFactory) *Bridge {
	return &Bridge{
		config:        cfg,
		buffer:        NewBuffer(cfg.MaxBuffer),
		metrics:       metrics,
		postFunc:      postFunc,
		clientFactory: factory,
		enabled:       cfg.Enabled,
		nowFunc:       time.Now,
	}
}

func (b *Bridge) Start() error {
	b.mu.Lock()
	b.stopCh = make(chan struct{})
	b.stopped = false
	if b.enabled && b.metrics != nil {
		b.metrics.SetEnabled(true)
	}
	b.mu.Unlock()

	if err := b.connect(); err != nil {
		log.Printf("MQTT bridge: initial connection failed: %v", err)
		b.notifyDiscordThrottled(fmt.Sprintf("MQTT bridge failed to connect: %v — will retry", err))
		go b.reconnectLoop()
	}

	go b.flushLoop()
	return nil
}

func (b *Bridge) Stop() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	b.stopped = true
	close(b.stopCh)
	client := b.client
	b.mu.Unlock()

	if client != nil && client.IsConnected() {
		client.Disconnect(250)
	}

	b.mu.Lock()
	b.connected = false
	if b.metrics != nil {
		b.metrics.SetConnected(false)
	}
	b.mu.Unlock()

	log.Println("MQTT bridge stopped")
}

func (b *Bridge) connect() error {
	b.mu.Lock()
	if b.client == nil {
		b.client = b.clientFactory(b.config, b.onConnectionLost, b.onConnect)
	}
	client := b.client
	b.mu.Unlock()

	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT connect: %w", err)
	}

	b.mu.Lock()
	b.connected = true
	if b.metrics != nil {
		b.metrics.SetConnected(true)
	}
	b.mu.Unlock()

	if err := b.subscribe(); err != nil {
		return fmt.Errorf("MQTT subscribe: %w", err)
	}
	log.Printf("MQTT bridge connected to %s", b.config.Broker)
	return nil
}

func (b *Bridge) subscribe() error {
	b.mu.Lock()
	client := b.client
	b.mu.Unlock()
	if client == nil {
		return fmt.Errorf("no MQTT client")
	}

	var errs []error
	for _, topic := range b.config.Topics {
		token := client.Subscribe(topic, 0, b.messageHandler)
		token.Wait()
		if err := token.Error(); err != nil {
			log.Printf("MQTT bridge: failed to subscribe to %s: %v", topic, err)
			errs = append(errs, fmt.Errorf("topic %s: %w", topic, err))
		} else {
			log.Printf("MQTT bridge: subscribed to %s", topic)
		}
	}
	return errors.Join(errs...)
}

func (b *Bridge) messageHandler(_ pahomqtt.Client, msg pahomqtt.Message) {
	if !b.IsEnabled() {
		return
	}

	payload := TruncatePayload(string(msg.Payload()), b.config.MaxPayloadBytes)
	dropped := b.buffer.Add(msg.Topic(), payload, time.Now())

	if b.metrics != nil {
		b.metrics.RecordReceived()
		if dropped > 0 {
			b.metrics.RecordDropped(dropped)
		}
		b.metrics.SetBufferDepth(b.buffer.Len())
	}
}

func (b *Bridge) onConnectionLost(_ pahomqtt.Client, err error) {
	log.Printf("MQTT bridge: connection lost: %v", err)

	b.mu.Lock()
	b.connected = false
	if b.metrics != nil {
		b.metrics.SetConnected(false)
	}
	stopped := b.stopped
	b.mu.Unlock()

	if !stopped {
		b.notifyDiscordThrottled(fmt.Sprintf("MQTT bridge connection lost: %v — will reconnect", err))
		go b.reconnectLoop()
	}
}

func (b *Bridge) onConnect(_ pahomqtt.Client) {
	b.mu.Lock()
	b.connected = true
	if b.metrics != nil {
		b.metrics.SetConnected(true)
	}
	b.mu.Unlock()

	if err := b.subscribe(); err != nil {
		log.Printf("MQTT bridge: resubscribe after reconnect failed: %v", err)
	}
	log.Println("MQTT bridge: reconnected")
}

func (b *Bridge) reconnectLoop() {
	b.mu.Lock()
	if b.reconnecting {
		b.mu.Unlock()
		return
	}
	b.reconnecting = true
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.reconnecting = false
		b.mu.Unlock()
	}()

	maxRetries := 10
	for attempt := range maxRetries {
		b.mu.Lock()
		stopped := b.stopped
		stopCh := b.stopCh
		b.mu.Unlock()

		if stopped {
			return
		}

		backoff := time.Duration(attempt+1) * 5 * time.Second
		log.Printf("MQTT bridge: reconnect attempt %d/%d in %v", attempt+1, maxRetries, backoff)

		select {
		case <-stopCh:
			return
		case <-time.After(backoff):
		}

		if err := b.connect(); err != nil {
			log.Printf("MQTT bridge: reconnect attempt %d failed: %v", attempt+1, err)
			continue
		}
		return
	}

	log.Println("MQTT bridge: exhausted reconnect attempts")
	b.notifyDiscord("MQTT bridge: exhausted reconnect attempts — bridge is offline")
}

func (b *Bridge) flushLoop() {
	ticker := time.NewTicker(time.Duration(b.config.FlushSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.flush()
		}
	}
}

func (b *Bridge) flush() {
	if !b.IsEnabled() {
		return
	}

	msgs := b.buffer.Flush()
	if len(msgs) == 0 {
		return
	}

	if b.metrics != nil {
		b.metrics.SetBufferDepth(0)
	}

	chunks := FormatDigest(msgs, 1900, b.config.MaxPostsPerFlush)

	forwarded := 0
	for _, chunk := range chunks {
		for _, ch := range b.config.DiscordChannels {
			if err := b.postFunc(ch, chunk); err != nil {
				log.Printf("MQTT bridge: failed to post to Discord channel %s: %v", ch, err)
			}
		}
		forwarded += len(msgs)
	}

	if b.metrics != nil {
		b.metrics.RecordForwarded(forwarded)
	}
}

const notifyCooldown = 5 * time.Minute

func (b *Bridge) notifyDiscordThrottled(msg string) {
	b.mu.Lock()
	now := b.nowFunc()
	if !b.lastNotifyTime.IsZero() && now.Sub(b.lastNotifyTime) < notifyCooldown {
		b.mu.Unlock()
		log.Printf("MQTT bridge: suppressing Discord notification (cooldown): %s", msg)
		if b.metrics != nil {
			b.metrics.RecordSuppressed(1)
		}
		return
	}
	b.lastNotifyTime = now
	b.mu.Unlock()

	b.notifyDiscord(msg)
}

func (b *Bridge) notifyDiscord(msg string) {
	for _, ch := range b.config.DiscordChannels {
		if err := b.postFunc(ch, msg); err != nil {
			log.Printf("MQTT bridge: failed to notify Discord: %v", err)
		}
	}
}

func (b *Bridge) IsEnabled() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.enabled
}

func (b *Bridge) Enable() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.enabled = true
	if b.metrics != nil {
		b.metrics.SetEnabled(true)
	}
}

func (b *Bridge) Disable() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.enabled = false
	if b.metrics != nil {
		b.metrics.SetEnabled(false)
	}
}

func (b *Bridge) Status() BridgeStatus {
	b.mu.Lock()
	defer b.mu.Unlock()

	depth := 0
	if b.buffer != nil {
		depth = b.buffer.Len()
	}

	topics := make([]string, len(b.config.Topics))
	copy(topics, b.config.Topics)

	return BridgeStatus{
		Enabled:     b.enabled,
		Connected:   b.connected,
		BufferDepth: depth,
		Topics:      topics,
	}
}
