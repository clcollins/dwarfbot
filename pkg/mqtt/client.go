package mqtt

import (
	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTTClient interface {
	Connect() pahomqtt.Token
	Disconnect(quiesce uint)
	Subscribe(topic string, qos byte, callback pahomqtt.MessageHandler) pahomqtt.Token
	IsConnected() bool
}

type ClientFactory func(cfg Config, onConnLost pahomqtt.ConnectionLostHandler, onConnect pahomqtt.OnConnectHandler) MQTTClient

func DefaultClientFactory(cfg Config, onConnLost pahomqtt.ConnectionLostHandler, onConnect pahomqtt.OnConnectHandler) MQTTClient {
	opts := pahomqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID).
		SetAutoReconnect(true).
		SetConnectionLostHandler(onConnLost).
		SetOnConnectHandler(onConnect)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}

	return pahomqtt.NewClient(opts)
}
