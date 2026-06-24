package mqtt

import "fmt"

type Config struct {
	Enabled          bool
	Broker           string
	Username         string
	Password         string
	ClientID         string
	Topics           []string
	DiscordChannels  []string
	FlushSeconds     int
	MaxBuffer        int
	MaxPayloadBytes  int
	MaxPostsPerFlush int
}

func ValidateConfig(cfg Config, discordConfigured bool) error {
	if !cfg.Enabled {
		return nil
	}

	if !discordConfigured {
		return fmt.Errorf("mqtt_enabled=true requires Discord to be configured: the mouthpiece has nowhere to speak")
	}

	if cfg.Broker == "" {
		return fmt.Errorf("mqtt_broker must be set when mqtt_enabled=true")
	}

	if cfg.FlushSeconds < 5 || cfg.FlushSeconds > 86400 {
		return fmt.Errorf("mqtt_flush_seconds must be between 5 and 86400, got %d", cfg.FlushSeconds)
	}

	if cfg.MaxBuffer <= 0 {
		return fmt.Errorf("mqtt_max_buffer must be > 0, got %d", cfg.MaxBuffer)
	}

	if cfg.MaxPayloadBytes <= 0 {
		return fmt.Errorf("mqtt_max_payload_bytes must be > 0, got %d", cfg.MaxPayloadBytes)
	}

	if cfg.MaxPostsPerFlush <= 0 {
		return fmt.Errorf("mqtt_max_posts_per_flush must be > 0, got %d", cfg.MaxPostsPerFlush)
	}

	return nil
}
