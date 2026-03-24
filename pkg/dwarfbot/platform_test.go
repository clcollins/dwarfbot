package dwarfbot

import (
	"strings"
	"testing"
)

// mockPlatform implements ChatPlatform for testing command routing
// without any real network connection or external dependencies.
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

func newMockPlatform(name string, channels []string) *mockPlatform {
	return &mockPlatform{
		name:     name,
		channels: channels,
	}
}

func newMockPlatformWithAdmin(name string, channels []string, adminFunc func(string, string) bool) *mockPlatform {
	return &mockPlatform{
		name:        name,
		channels:    channels,
		isAdminFunc: adminFunc,
	}
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

// --- Compile-time interface compliance ---

var _ ChatPlatform = (*DwarfBot)(nil)
var _ ChatPlatform = (*DiscordBot)(nil)
var _ ChatPlatform = (*mockPlatform)(nil)

// --- mockPlatform method tests ---

func TestMockPlatform_SendMessage(t *testing.T) {
	mock := newMockPlatform("bot", nil)
	err := mock.SendMessage("ch1", "hello")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if mock.messages[0].channel != "ch1" || mock.messages[0].msg != "hello" {
		t.Errorf("unexpected message: %+v", mock.messages[0])
	}
}

func TestMockPlatform_IsAdmin_Default(t *testing.T) {
	mock := newMockPlatform("bot", nil)
	if mock.IsAdmin("ch", "user") {
		t.Error("expected default IsAdmin to return false")
	}
}

func TestMockPlatform_IsAdmin_Custom(t *testing.T) {
	mock := newMockPlatformWithAdmin("bot", nil, func(ch, user string) bool {
		return user == "admin"
	})
	if !mock.IsAdmin("ch", "admin") {
		t.Error("expected admin user to be admin")
	}
	if mock.IsAdmin("ch", "regular") {
		t.Error("expected regular user to not be admin")
	}
}

func TestMockPlatform_BotName(t *testing.T) {
	mock := newMockPlatform("mybot", nil)
	if mock.BotName() != "mybot" {
		t.Errorf("expected 'mybot', got %q", mock.BotName())
	}
}

func TestMockPlatform_BotChannels(t *testing.T) {
	mock := newMockPlatform("bot", []string{"a", "b"})
	ch := mock.BotChannels()
	if len(ch) != 2 || ch[0] != "a" || ch[1] != "b" {
		t.Errorf("expected [a b], got %v", ch)
	}
}

func TestMockPlatform_BotChannels_Empty(t *testing.T) {
	mock := newMockPlatform("bot", nil)
	if mock.BotChannels() != nil {
		t.Errorf("expected nil channels, got %v", mock.BotChannels())
	}
}

func TestMockPlatform_Shutdown(t *testing.T) {
	mock := newMockPlatform("bot", nil)
	mock.Shutdown(0)
	mock.Shutdown(1)
	if len(mock.shutdownLog) != 2 {
		t.Fatalf("expected 2 shutdown calls, got %d", len(mock.shutdownLog))
	}
	if mock.shutdownLog[0] != 0 || mock.shutdownLog[1] != 1 {
		t.Errorf("expected [0, 1], got %v", mock.shutdownLog)
	}
}

// --- Command routing via mockPlatform ---

func TestMockPlatform_ParseCommand_Ping(t *testing.T) {
	mock := newMockPlatform("testbot", []string{"ch1"})
	_ = parseCommand(mock, "ch1", "someuser", "ping", []string{})

	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if !strings.Contains(mock.messages[0].msg, "Pong") && !strings.Contains(mock.messages[0].msg, "Atari") {
		t.Errorf("expected pong response, got %q", mock.messages[0].msg)
	}
	if mock.messages[0].channel != "ch1" {
		t.Errorf("expected channel 'ch1', got %q", mock.messages[0].channel)
	}
}

func TestMockPlatform_ParseCommand_PingHeyo(t *testing.T) {
	mock := newMockPlatform("testbot", []string{"ch1"})
	_ = parseCommand(mock, "ch1", "user1", "ping", []string{"heyo"})

	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if !strings.Contains(mock.messages[0].msg, "Heyo") {
		t.Errorf("expected Heyo response, got %q", mock.messages[0].msg)
	}
}

func TestMockPlatform_ParseCommand_PingHeyoExtended(t *testing.T) {
	mock := newMockPlatform("testbot", []string{"ch1"})
	_ = parseCommand(mock, "ch1", "user1", "ping", []string{"heyoooo"})

	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if !strings.Contains(mock.messages[0].msg, "Heyo") {
		t.Errorf("expected Heyo response for heyoooo, got %q", mock.messages[0].msg)
	}
}

func TestMockPlatform_ParseCommand_Channels(t *testing.T) {
	mock := newMockPlatform("dwarfbot", []string{"general", "gaming"})
	_ = parseCommand(mock, "general", "user1", "channels", []string{})

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

func TestMockPlatform_ParseCommand_Channels_EmptyList(t *testing.T) {
	mock := newMockPlatform("solobot", []string{})
	_ = parseCommand(mock, "ch1", "user1", "channels", []string{})

	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if !strings.Contains(mock.messages[0].msg, "solobot") {
		t.Errorf("expected bot name, got %q", mock.messages[0].msg)
	}
}

func TestMockPlatform_ParseCommand_Unknown(t *testing.T) {
	mock := newMockPlatform("testbot", []string{"ch1"})
	_ = parseCommand(mock, "ch1", "user1", "nonexistent", []string{})

	if len(mock.messages) != 0 {
		t.Errorf("expected no messages for unknown command, got %d", len(mock.messages))
	}
}

func TestMockPlatform_AdminShutdown(t *testing.T) {
	mock := newMockPlatformWithAdmin("testbot", []string{"ch1"}, func(ch, user string) bool {
		return user == "admin_user"
	})

	_ = parseCommand(mock, "ch1", "admin_user", "shutdown", []string{})

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
	mock := newMockPlatformWithAdmin("testbot", []string{"ch1"}, func(ch, user string) bool {
		return false
	})

	_ = parseCommand(mock, "ch1", "regular_user", "shutdown", []string{})

	if len(mock.shutdownLog) != 0 {
		t.Error("expected Shutdown NOT to be called for non-admin")
	}
	if len(mock.messages) != 0 {
		t.Errorf("expected no messages for non-admin shutdown, got %d", len(mock.messages))
	}
}

func TestMockPlatform_AdminPingStillWorks(t *testing.T) {
	mock := newMockPlatformWithAdmin("testbot", []string{"ch1"}, func(ch, user string) bool {
		return user == "admin_user"
	})

	_ = parseCommand(mock, "ch1", "admin_user", "ping", []string{})

	// Admin users can also use regular commands
	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if !strings.Contains(mock.messages[0].msg, "Pong") && !strings.Contains(mock.messages[0].msg, "Atari") {
		t.Errorf("expected ping response for admin, got %q", mock.messages[0].msg)
	}
}

// --- DiscordBot unit tests (no real Discord connection, fully mocked) ---

func TestDiscordBot_BotName(t *testing.T) {
	bot := &DiscordBot{Name: "discordbot"}
	if bot.BotName() != "discordbot" {
		t.Errorf("expected 'discordbot', got %q", bot.BotName())
	}
}

func TestDiscordBot_BotName_Empty(t *testing.T) {
	bot := &DiscordBot{}
	if bot.BotName() != "" {
		t.Errorf("expected empty name, got %q", bot.BotName())
	}
}

func TestDiscordBot_BotChannels(t *testing.T) {
	bot := &DiscordBot{ChannelIDs: []string{"123", "456"}}
	channels := bot.BotChannels()
	if len(channels) != 2 || channels[0] != "123" || channels[1] != "456" {
		t.Errorf("expected [123 456], got %v", channels)
	}
}

func TestDiscordBot_BotChannels_Empty(t *testing.T) {
	bot := &DiscordBot{}
	if len(bot.BotChannels()) != 0 {
		t.Errorf("expected empty channels, got %v", bot.BotChannels())
	}
}

func TestDiscordBot_IsAdmin_NoSession(t *testing.T) {
	bot := &DiscordBot{AdminRole: "admin"}
	if bot.IsAdmin("channel", "user") {
		t.Error("expected IsAdmin to return false without session")
	}
}

func TestDiscordBot_IsAdmin_NoRole(t *testing.T) {
	bot := &DiscordBot{}
	if bot.IsAdmin("channel", "user") {
		t.Error("expected IsAdmin to return false without admin role")
	}
}

func TestDiscordBot_IsAdmin_EmptyRole(t *testing.T) {
	bot := &DiscordBot{AdminRole: ""}
	if bot.IsAdmin("channel", "user") {
		t.Error("expected IsAdmin to return false with empty admin role")
	}
}

func TestDiscordBot_SendMessage_NoSession(t *testing.T) {
	bot := &DiscordBot{Name: "testbot"}
	err := bot.SendMessage("channel", "test")
	if err == nil {
		t.Error("expected error when sending without session")
	}
	if !strings.Contains(err.Error(), "session not initialized") {
		t.Errorf("expected 'session not initialized' error, got %q", err.Error())
	}
}

func TestDiscordBot_Shutdown_WithExitFunc(t *testing.T) {
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

func TestDiscordBot_Shutdown_ZeroExitCode(t *testing.T) {
	var exitCode int
	bot := &DiscordBot{
		exitFunc: func(code int) {
			exitCode = code
		},
	}

	bot.Shutdown(0)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestDiscordBot_Shutdown_WithCustomExitFunc(t *testing.T) {
	// Verify that Shutdown calls the provided exitFunc when set.
	called := false
	bot := &DiscordBot{
		exitFunc: func(code int) { called = true },
	}
	bot.Shutdown(0)
	if !called {
		t.Error("expected exitFunc to be called")
	}
}

func TestDiscordBot_Stop_NilSession(t *testing.T) {
	bot := &DiscordBot{}
	err := bot.Stop()
	if err != nil {
		t.Errorf("expected no error stopping nil session, got %v", err)
	}
}

func TestDiscordBot_Fields(t *testing.T) {
	bot := &DiscordBot{
		Token:      "testtoken",
		ChannelIDs: []string{"111", "222"},
		AdminRole:  "admin-role",
		Name:       "mybot",
	}
	if bot.Token != "testtoken" {
		t.Errorf("expected 'testtoken', got %q", bot.Token)
	}
	if len(bot.ChannelIDs) != 2 {
		t.Errorf("expected 2 channel IDs, got %d", len(bot.ChannelIDs))
	}
	if bot.AdminRole != "admin-role" {
		t.Errorf("expected 'admin-role', got %q", bot.AdminRole)
	}
	if bot.Name != "mybot" {
		t.Errorf("expected 'mybot', got %q", bot.Name)
	}
}

// --- ChatPlatform interface contract tests ---

func TestChatPlatform_Contract_MockPlatform(t *testing.T) {
	var p ChatPlatform = newMockPlatform("bot", []string{"ch"})

	if p.BotName() != "bot" {
		t.Errorf("BotName: expected 'bot', got %q", p.BotName())
	}
	if len(p.BotChannels()) != 1 {
		t.Errorf("BotChannels: expected 1, got %d", len(p.BotChannels()))
	}
	if p.IsAdmin("ch", "user") {
		t.Error("IsAdmin: expected false for default mock")
	}
	if err := p.SendMessage("ch", "hi"); err != nil {
		t.Errorf("SendMessage: unexpected error %v", err)
	}
}

func TestChatPlatform_Contract_DiscordBot(t *testing.T) {
	var p ChatPlatform = &DiscordBot{
		Name:       "dbot",
		ChannelIDs: []string{"123"},
		AdminRole:  "admin",
	}

	if p.BotName() != "dbot" {
		t.Errorf("BotName: expected 'dbot', got %q", p.BotName())
	}
	if len(p.BotChannels()) != 1 {
		t.Errorf("BotChannels: expected 1, got %d", len(p.BotChannels()))
	}
	// No session, so IsAdmin should be false
	if p.IsAdmin("123", "user") {
		t.Error("IsAdmin: expected false without session")
	}
	// No session, so SendMessage should error
	if err := p.SendMessage("123", "hi"); err == nil {
		t.Error("SendMessage: expected error without session")
	}
}
