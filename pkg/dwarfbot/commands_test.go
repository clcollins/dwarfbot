package dwarfbot

import (
	"bufio"
	"net"
	"regexp"
	"strings"
	"testing"
	"time"
)

// --- contains Tests ---

func TestContains_Found(t *testing.T) {
	list := []string{"apple", "banana", "cherry"}
	if !contains(list, "banana") {
		t.Error("expected contains to find 'banana'")
	}
}

func TestContains_NotFound(t *testing.T) {
	list := []string{"apple", "banana", "cherry"}
	if contains(list, "grape") {
		t.Error("expected contains to not find 'grape'")
	}
}

func TestContains_EmptySlice(t *testing.T) {
	if contains([]string{}, "anything") {
		t.Error("expected contains to return false for empty slice")
	}
}

func TestContains_CaseSensitive(t *testing.T) {
	list := []string{"Apple", "Banana"}
	if contains(list, "apple") {
		t.Error("expected case-sensitive mismatch")
	}
}

// --- reContains Tests ---

func TestReContains_Match(t *testing.T) {
	re := regexp.MustCompile(`^hello.*`)
	list := []string{"goodbye", "hello world"}
	if !reContains(list, re) {
		t.Error("expected reContains to find match")
	}
}

func TestReContains_NoMatch(t *testing.T) {
	re := regexp.MustCompile(`^hello.*`)
	list := []string{"goodbye", "world"}
	if reContains(list, re) {
		t.Error("expected reContains to not find match")
	}
}

func TestReContains_EmptySlice(t *testing.T) {
	re := regexp.MustCompile(`.*`)
	if reContains([]string{}, re) {
		t.Error("expected reContains to return false for empty slice")
	}
}

// --- ping Tests ---

func TestPing_Default(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		ping(bot, "testchannel", []string{})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "Atari") {
		t.Errorf("expected default pong response with 'Atari', got %q", got)
	}
}

func TestPing_Heyo(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		ping(bot, "testchannel", []string{"heyo"})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "Heyo") {
		t.Errorf("expected heyo response, got %q", got)
	}
}

func TestPing_HeyoExtended(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		ping(bot, "testchannel", []string{"heyooo"})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "Heyo") {
		t.Errorf("expected heyo response for extended heyo, got %q", got)
	}
}

// --- channels Tests ---

func TestChannels(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		channels(bot, "testchannel", []string{})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "testbot") {
		t.Errorf("expected bot name in channels output, got %q", got)
	}
	if !strings.Contains(got, "channel1") {
		t.Errorf("expected 'channel1' in channels output, got %q", got)
	}
	if !strings.Contains(got, "channel2") {
		t.Errorf("expected 'channel2' in channels output, got %q", got)
	}
}

func TestChannels_NoExtra(t *testing.T) {
	server, client := net.Pipe()
	bot := &DwarfBot{
		Name:     "solobot",
		Channels: []string{},
		conn:     client,
		exitFunc: func(code int) {},
	}
	defer func() {
		client.Close()
		server.Close()
	}()

	go func() {
		channels(bot, "testchannel", []string{})
	}()

	server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 4096)
	n, _ := server.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "solobot") {
		t.Errorf("expected bot name, got %q", got)
	}
}

// --- parseCommand Tests ---

func TestParseCommand_Ping(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		parseCommand(bot, "testchannel", "someuser", "ping", []string{})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "PRIVMSG") {
		t.Errorf("expected PRIVMSG for ping command, got %q", got)
	}
}

func TestParseCommand_Channels(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		parseCommand(bot, "testchannel", "someuser", "channels", []string{})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "testbot") {
		t.Errorf("expected channels response, got %q", got)
	}
}

func TestParseCommand_Unknown(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		parseCommand(bot, "testchannel", "someuser", "unknowncmd", []string{})
	}()

	// Unknown command should not generate output
	server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 256)
	n, _ := server.Read(buf)
	if n > 0 {
		t.Errorf("expected no output for unknown command, got %q", string(buf[:n]))
	}
}

// --- parseAdminCommand Tests ---

func TestParseCommand_AdminFromOwner(t *testing.T) {
	shutdownCalled := false
	bot, server, cleanup := newTestBot(t)
	bot.exitFunc = func(code int) { shutdownCalled = true }
	defer cleanup()

	go func() {
		// userName == channelName triggers admin path
		parseCommand(bot, "owneruser", "owneruser", "shutdown", []string{})
	}()

	reader := bufio.NewReader(server)
	server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	got, _ := reader.ReadString('\n')
	if !strings.Contains(got, "Shuttin") {
		t.Errorf("expected shutdown message, got %q", got)
	}
	// Give time for Die to be called
	time.Sleep(50 * time.Millisecond)
	if !shutdownCalled {
		t.Error("expected Die/exitFunc to be called for admin shutdown")
	}
}

func TestParseCommand_AdminFromNonOwner(t *testing.T) {
	shutdownCalled := false
	bot, server, cleanup := newTestBot(t)
	bot.exitFunc = func(code int) { shutdownCalled = true }
	defer cleanup()

	go func() {
		// userName != channelName, should NOT trigger admin
		parseCommand(bot, "owneruser", "regularuser", "shutdown", []string{})
	}()

	// Should not produce any output (shutdown is admin-only)
	server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 256)
	n, _ := server.Read(buf)
	if n > 0 {
		t.Errorf("expected no output for non-admin shutdown, got %q", string(buf[:n]))
	}
	if shutdownCalled {
		t.Error("expected Die/exitFunc NOT to be called for non-admin user")
	}
}

func TestParseAdminCommand_UnknownAdmin(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		parseAdminCommand(bot, "testchannel", "unknownadmin", []string{})
	}()

	// Unknown admin command should not produce output
	server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 256)
	n, _ := server.Read(buf)
	if n > 0 {
		t.Errorf("expected no output for unknown admin command, got %q", string(buf[:n]))
	}
}
