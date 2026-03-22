package dwarfbot

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// newTestBot creates a DwarfBot wired to a net.Pipe for testing.
// Returns the bot, the "server" side of the pipe, and a cleanup function.
func newTestBot(t *testing.T) (*DwarfBot, net.Conn, func()) {
	t.Helper()
	server, client := net.Pipe()
	bot := &DwarfBot{
		Name:     "testbot",
		Channels: []string{"channel1", "channel2"},
		conn:     client,
		Credentials: &OAuthCreds{
			Name:  "testbot",
			Token: "oauth_test_token",
		},
		exitFunc: func(code int) {}, // no-op for tests
	}
	cleanup := func() {
		client.Close()
		server.Close()
	}
	return bot, server, cleanup
}

// readFromConn reads everything available from conn until it blocks, with a short deadline.
func readFromConn(t *testing.T, conn net.Conn) string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	return string(buf[:n])
}

// --- Regex Tests ---

func TestMsgRegex_ValidPRIVMSG(t *testing.T) {
	line := ":username!username@username.tmi.twitch.tv PRIVMSG #channel :Hello world"
	matches := msgRegex.FindStringSubmatch(line)
	if matches == nil {
		t.Fatal("expected msgRegex to match valid PRIVMSG line")
	}
	if matches[1] != "username" {
		t.Errorf("expected username, got %q", matches[1])
	}
	if matches[2] != "PRIVMSG" {
		t.Errorf("expected PRIVMSG, got %q", matches[2])
	}
	if matches[3] != "channel" {
		t.Errorf("expected channel, got %q", matches[3])
	}
	if matches[4] != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", matches[4])
	}
}

func TestMsgRegex_NoMatch(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"ping", "PING :tmi.twitch.tv"},
		{"empty", ""},
		{"random", "some random text"},
		{"partial", ":user!user@user.tmi.twitch.tv NOTICE #channel :msg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if msgRegex.FindStringSubmatch(tc.line) != nil {
				t.Errorf("expected no match for %q", tc.line)
			}
		})
	}
}

func TestCmdRegex_ValidCommand(t *testing.T) {
	tests := []struct {
		name      string
		msg       string
		wantCmd   string
		wantArg   string
		wantExtra string
	}{
		{"simple command", "!dwarfbot ping", "dwarfbot", "ping", ""},
		{"command with arg", "!dwarfbot channels list", "dwarfbot", "channels", "list"},
		{"command with multiple args", "!dwarfbot say hello world", "dwarfbot", "say", "hello world"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matches := cmdRegex.FindStringSubmatch(tc.msg)
			if matches == nil {
				t.Fatalf("expected cmdRegex to match %q", tc.msg)
			}
			if matches[1] != tc.wantCmd {
				t.Errorf("command: got %q, want %q", matches[1], tc.wantCmd)
			}
			if matches[2] != tc.wantArg {
				t.Errorf("arg: got %q, want %q", matches[2], tc.wantArg)
			}
		})
	}
}

func TestCmdRegex_NoMatch(t *testing.T) {
	tests := []string{
		"hello world",
		"no command here",
		"",
	}
	for _, msg := range tests {
		if cmdRegex.FindStringSubmatch(msg) != nil {
			t.Errorf("expected no match for %q", msg)
		}
	}
}

// --- Say Tests ---

func TestSay_Success(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		err := bot.Say("testchannel", "Hello!")
		if err != nil {
			t.Errorf("Say returned error: %v", err)
		}
	}()

	got := readFromConn(t, server)
	expected := "PRIVMSG #testchannel :Hello!\r\n"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestSay_EmptyMessage(t *testing.T) {
	bot, _, cleanup := newTestBot(t)
	defer cleanup()

	err := bot.Say("channel", "")
	if err == nil {
		t.Error("expected error for empty message")
	}
}

// --- Authenticate Tests ---

func TestAuthenticate(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go bot.Authenticate()

	// Read both PASS and NICK commands — they may arrive in separate reads
	var all string
	for i := 0; i < 2; i++ {
		server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		buf := make([]byte, 4096)
		n, _ := server.Read(buf)
		all += string(buf[:n])
	}
	if !strings.Contains(all, "PASS oauth:oauth_test_token\r\n") {
		t.Errorf("expected PASS command, got %q", all)
	}
	if !strings.Contains(all, "NICK testbot\r\n") {
		t.Errorf("expected NICK command, got %q", all)
	}
}

// --- JoinChannel Tests ---

func TestJoinChannel(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go bot.JoinChannel("TestChannel")

	got := readFromConn(t, server)
	expected := "JOIN #testchannel\r\n"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestJoinChannel_Empty(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	bot.JoinChannel("")

	// Should not write anything to the connection
	server.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	buf := make([]byte, 256)
	n, err := server.Read(buf)
	if n > 0 {
		t.Errorf("expected no data for empty channel, got %q", string(buf[:n]))
	}
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// --- PartChannel Tests ---

func TestPartChannel(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go bot.PartChannel("TestChannel")

	got := readFromConn(t, server)
	expected := "PART #testchannel\r\n"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestPartChannel_Empty(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	bot.PartChannel("")

	server.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	buf := make([]byte, 256)
	n, _ := server.Read(buf)
	if n > 0 {
		t.Errorf("expected no data for empty channel, got %q", string(buf[:n]))
	}
}

// --- Die Tests ---

func TestDie_CallsExitFunc(t *testing.T) {
	var exitCode int
	called := false
	bot := &DwarfBot{
		exitFunc: func(code int) {
			called = true
			exitCode = code
		},
	}

	bot.Die(42)

	if !called {
		t.Error("expected exitFunc to be called")
	}
	if exitCode != 42 {
		t.Errorf("expected exit code 42, got %d", exitCode)
	}
}

// --- Disconnect Tests ---

func TestDisconnect(t *testing.T) {
	_, client := net.Pipe()
	bot := &DwarfBot{
		conn:      client,
		startTime: time.Now(),
	}

	// Should not panic
	bot.Disconnect()
}

// --- HandleChat Tests ---

func TestHandleChat_Ping(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	// Write a PING line followed by closing the connection
	go func() {
		server.Write([]byte("PING :tmi.twitch.tv\r\n"))
		// Read the PONG response
		buf := make([]byte, 4096)
		server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _ := server.Read(buf)
		response := string(buf[:n])
		if !strings.Contains(response, "PONG :tmi.twitch.tv") {
			t.Errorf("expected PONG response, got %q", response)
		}
		// Close to trigger HandleChat to return
		server.Close()
	}()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

func TestHandleChat_PrivmsgCommand(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	// Send a PRIVMSG with a command, then close
	go func() {
		line := ":someuser!someuser@someuser.tmi.twitch.tv PRIVMSG #testchannel :!dwarfbot ping\r\n"
		server.Write([]byte(line))

		// Read the bot's response
		reader := bufio.NewReader(server)
		server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		response, _ := reader.ReadString('\n')
		if !strings.Contains(response, "PRIVMSG #testchannel") {
			t.Errorf("expected PRIVMSG response, got %q", response)
		}
		if !strings.Contains(response, "Pong") && !strings.Contains(response, "pong") && !strings.Contains(response, "Atari") {
			t.Errorf("expected ping response, got %q", response)
		}
		server.Close()
	}()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

func TestHandleChat_NonMatchingLine(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		// Send a line that doesn't match any pattern
		server.Write([]byte("some random IRC line\r\n"))
		// Then close
		time.Sleep(50 * time.Millisecond)
		server.Close()
	}()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

// --- Aliases Test ---

func TestAliases(t *testing.T) {
	expected := []string{"hammerdwarfbot", "dwarfbot"}
	if len(aliases) != len(expected) {
		t.Fatalf("expected %d aliases, got %d", len(expected), len(aliases))
	}
	for i, alias := range aliases {
		if alias != expected[i] {
			t.Errorf("alias[%d]: got %q, want %q", i, alias, expected[i])
		}
	}
}

// --- OAuthCreds Test ---

func TestOAuthCreds(t *testing.T) {
	creds := &OAuthCreds{
		Name:  "botname",
		Token: "secret",
	}
	if creds.Name != "botname" {
		t.Errorf("expected name 'botname', got %q", creds.Name)
	}
	if creds.Token != "secret" {
		t.Errorf("expected token 'secret', got %q", creds.Token)
	}
}

// --- Say with closed connection ---

func TestSay_ClosedConn(t *testing.T) {
	server, client := net.Pipe()
	bot := &DwarfBot{
		conn: client,
		Name: "testbot",
	}
	client.Close()
	server.Close()

	err := bot.Say("channel", "test message")
	if err == nil {
		t.Error("expected error writing to closed connection")
	}
}

// --- HandleChat with command directed at wrong bot ---

func TestHandleChat_WrongBotAlias(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		// Command directed at a different bot - should be ignored
		line := ":someuser!someuser@someuser.tmi.twitch.tv PRIVMSG #testchannel :!otherbot ping\r\n"
		server.Write([]byte(line))
		// Give the bot time to process
		time.Sleep(50 * time.Millisecond)
		// Close to end test
		server.Close()
	}()

	// Bot should not respond but should eventually error on connection close
	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

// --- HandleChat with verbose mode ---

func TestHandleChat_Verbose(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	bot.Verbose = true
	defer cleanup()

	go func() {
		line := ":user!user@user.tmi.twitch.tv PRIVMSG #testchannel :!dwarfbot ping\r\n"
		server.Write([]byte(line))
		// Read response
		server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		io.ReadAll(server)
		server.Close()
	}()

	err := bot.HandleChat()
	// Should still work in verbose mode
	if err == nil {
		t.Error("expected error after connection close")
	}
}

// --- Integration: full PRIVMSG with channels command ---

func TestHandleChat_ChannelsCommand(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		line := ":someuser!someuser@someuser.tmi.twitch.tv PRIVMSG #testchannel :!dwarfbot channels\r\n"
		server.Write([]byte(line))

		reader := bufio.NewReader(server)
		server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		response, _ := reader.ReadString('\n')
		if !strings.Contains(response, "testbot") {
			t.Errorf("expected bot name in channels response, got %q", response)
		}
		if !strings.Contains(response, "channel1") || !strings.Contains(response, "channel2") {
			t.Errorf("expected channels in response, got %q", response)
		}
		server.Close()
	}()

	bot.HandleChat()
}

// --- HandleChat with admin shutdown ---

func TestHandleChat_AdminShutdown(t *testing.T) {
	shutdownCalled := false
	bot, server, cleanup := newTestBot(t)
	bot.exitFunc = func(code int) {
		shutdownCalled = true
	}
	defer cleanup()

	go func() {
		// Admin command: userName matches channelName
		line := fmt.Sprintf(":%s!%s@%s.tmi.twitch.tv PRIVMSG #%s :!dwarfbot shutdown\r\n",
			"testchannel", "testchannel", "testchannel", "testchannel")
		server.Write([]byte(line))

		// Read the goodbye message
		reader := bufio.NewReader(server)
		server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		response, _ := reader.ReadString('\n')
		if !strings.Contains(response, "Shuttin") {
			t.Errorf("expected shutdown message, got %q", response)
		}
		server.Close()
	}()

	bot.HandleChat()

	if !shutdownCalled {
		t.Error("expected shutdown/Die to be called for admin shutdown command")
	}
}
