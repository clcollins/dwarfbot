package dwarfbot

import "time"

// PlatformMetrics provides hooks for recording platform-level metrics
// without coupling to a specific metrics implementation.
type PlatformMetrics interface {
	RecordConnectionAttempt(platform, result string)
	RecordConnected(platform string)
	RecordDisconnected(platform, reason string)
	RecordConnectionDuration(platform string, duration time.Duration)
	RecordMessageReceived(platform string)
	RecordMessageSent(platform, result string)
	RecordCommandProcessed(platform, command, admin string)
}

// ChatPlatform abstracts a chat service (Twitch, Discord, etc.)
// so that command handlers can work across platforms.
type ChatPlatform interface {
	// SendMessage sends a message to the specified channel.
	SendMessage(channel, msg string) error

	// IsAdmin checks if the user has admin privileges on this platform.
	IsAdmin(channel, user string) bool

	// BotName returns the bot's display name.
	BotName() string

	// BotChannels returns the channels the bot is participating in.
	BotChannels() []string

	// Shutdown performs a clean shutdown of the platform connection.
	Shutdown(exitCode int)
}
