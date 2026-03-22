package dwarfbot

import (
	"fmt"
	"strings"
	"testing"
)

// mockPlatform implements ChatPlatform for testing command routing
// without any real network connection.
type mockPlatform struct {
	name        string
	channels    []string
	messages    []mockMessage
	isAdminFunc func(channel, user string) bool
	shutdownLog []int
}

type mockMessage struct {
	channel string
	msg     string
}

func (m *mockPlatform) SendMessage(channel, msg string) error {
	m.messages = append(m.messages, mockMessage{channel: channel, msg: msg})
	return nil
}

func (m *mockPlatform) IsAdmin(channel, user string) bool {
	if m.isAdminFunc != nil {
		return m.isAdminFunc(channel, user)
	}
	return false
}

func (m *mockPlatform) BotName() string {
	return m.name
}

func (m *mockPlatform) BotChannels() []string {
	return m.channels
}

func (m *mockPlatform) Shutdown(exitCode int) {
	m.shutdownLog = append(m.shutdownLog, exitCode)
}

// --- Tests using mockPlatform ---

func TestMockPlatform_ParseCommand_Ping(t *testing.T) {
	mock := &mockPlatform{name: "testbot", channels: []string{"ch1"}}
	parseCommand(mock, "ch1", "someuser", "ping", []string{})

	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if !strings.Contains(mock.messages[0].msg, "Pong") && !strings.Contains(mock.messages[0].msg, "Atari") {
		t.Errorf("expected pong response, got %q", mock.messages[0].msg)
	}
}

func TestMockPlatform_ParseCommand_Channels(t *testing.T) {
	mock := &mockPlatform{name: "dwarfbot", channels: []string{"general", "gaming"}}
	parseCommand(mock, "general", "user1", "channels", []string{})

	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	msg := mock.messages[0].msg
	if !strings.Contains(msg, "dwarfbot") {
		t.Errorf("expected bot name in channels response, got %q", msg)
	}
	if !strings.Contains(msg, "general") || !strings.Contains(msg, "gaming") {
		t.Errorf("expected channel names in response, got %q", msg)
	}
}

func TestMockPlatform_AdminShutdown(t *testing.T) {
	mock := &mockPlatform{
		name:     "testbot",
		channels: []string{"ch1"},
		isAdminFunc: func(channel, user string) bool {
			return user == "admin_user"
		},
	}

	parseCommand(mock, "ch1", "admin_user", "shutdown", []string{})

	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 shutdown message, got %d messages", len(mock.messages))
	}
	if !strings.Contains(mock.messages[0].msg, "Shuttin") {
		t.Errorf("expected shutdown message, got %q", mock.messages[0].msg)
	}
	if len(mock.shutdownLog) != 1 || mock.shutdownLog[0] != 0 {
		t.Errorf("expected Shutdown(0) to be called, got %v", mock.shutdownLog)
	}
}

func TestMockPlatform_NonAdminCannotShutdown(t *testing.T) {
	mock := &mockPlatform{
		name:     "testbot",
		channels: []string{"ch1"},
		isAdminFunc: func(channel, user string) bool {
			return false
		},
	}

	parseCommand(mock, "ch1", "regular_user", "shutdown", []string{})

	if len(mock.shutdownLog) != 0 {
		t.Error("expected Shutdown NOT to be called for non-admin")
	}
}

func TestMockPlatform_PingHeyo(t *testing.T) {
	mock := &mockPlatform{name: "testbot", channels: []string{"ch1"}}
	parseCommand(mock, "ch1", "user1", "ping", []string{"heyo"})

	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if !strings.Contains(mock.messages[0].msg, "Heyo") {
		t.Errorf("expected Heyo response, got %q", mock.messages[0].msg)
	}
}

func TestMockPlatform_UnknownCommand(t *testing.T) {
	mock := &mockPlatform{name: "testbot", channels: []string{"ch1"}}
	parseCommand(mock, "ch1", "user1", "nonexistent", []string{})

	if len(mock.messages) != 0 {
		t.Errorf("expected no messages for unknown command, got %d", len(mock.messages))
	}
}

// Verify DwarfBot satisfies ChatPlatform at compile time
var _ ChatPlatform = (*DwarfBot)(nil)

// Verify DiscordBot satisfies ChatPlatform at compile time
var _ ChatPlatform = (*DiscordBot)(nil)

// Verify mockPlatform satisfies ChatPlatform at compile time
var _ ChatPlatform = (*mockPlatform)(nil)

// --- DwarfBot ChatPlatform method tests ---

func TestDwarfBot_IsAdmin(t *testing.T) {
	bot := &DwarfBot{Name: "testbot"}

	if !bot.IsAdmin("owner", "owner") {
		t.Error("expected IsAdmin to return true when user == channel")
	}
	if bot.IsAdmin("owner", "other") {
		t.Error("expected IsAdmin to return false when user != channel")
	}
}

func TestDwarfBot_BotName(t *testing.T) {
	bot := &DwarfBot{Name: "mybot"}
	if bot.BotName() != "mybot" {
		t.Errorf("expected 'mybot', got %q", bot.BotName())
	}
}

func TestDwarfBot_BotChannels(t *testing.T) {
	bot := &DwarfBot{Channels: []string{"a", "b"}}
	channels := bot.BotChannels()
	if len(channels) != 2 || channels[0] != "a" || channels[1] != "b" {
		t.Errorf("expected [a b], got %v", channels)
	}
}

func TestDwarfBot_SendMessage(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		err := bot.SendMessage("testchannel", "Hello via interface!")
		if err != nil {
			t.Errorf("SendMessage returned error: %v", err)
		}
	}()

	got := readFromConn(t, server)
	expected := "PRIVMSG #testchannel :Hello via interface!\r\n"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

// --- DiscordBot unit tests (no real Discord connection) ---

func TestDiscordBot_BotName(t *testing.T) {
	bot := &DiscordBot{Name: "discordbot"}
	if bot.BotName() != "discordbot" {
		t.Errorf("expected 'discordbot', got %q", bot.BotName())
	}
}

func TestDiscordBot_BotChannels(t *testing.T) {
	bot := &DiscordBot{ChannelIDs: []string{"123", "456"}}
	channels := bot.BotChannels()
	if len(channels) != 2 || channels[0] != "123" || channels[1] != "456" {
		t.Errorf("expected [123 456], got %v", channels)
	}
}

func TestDiscordBot_IsAdmin_NoSession(t *testing.T) {
	bot := &DiscordBot{AdminRole: "admin"}
	// Without a session, IsAdmin should return false
	if bot.IsAdmin("channel", "user") {
		t.Error("expected IsAdmin to return false without session")
	}
}

func TestDiscordBot_IsAdmin_NoRole(t *testing.T) {
	bot := &DiscordBot{}
	// Without an admin role configured, IsAdmin should return false
	if bot.IsAdmin("channel", "user") {
		t.Error("expected IsAdmin to return false without admin role")
	}
}

func TestDiscordBot_SendMessage_NoSession(t *testing.T) {
	bot := &DiscordBot{Name: "testbot"}
	err := bot.SendMessage("channel", "test")
	if err == nil {
		t.Error("expected error when sending without session")
	}
}

func TestDiscordBot_Shutdown(t *testing.T) {
	var exitCode int
	called := false
	bot := &DiscordBot{
		exitFunc: func(code int) {
			called = true
			exitCode = code
		},
	}

	bot.Shutdown(1)

	if !called {
		t.Error("expected exitFunc to be called")
	}
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestDiscordBot_Stop_NilSession(t *testing.T) {
	bot := &DiscordBot{}
	err := bot.Stop()
	if err != nil {
		fmt.Printf("expected no error stopping nil session, got %v", err)
	}
}
