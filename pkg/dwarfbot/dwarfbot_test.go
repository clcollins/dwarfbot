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
		Server:   "localhost",
		Port:     "6667",
		exitFunc: func(code int) {}, // no-op for tests
	}
	cleanup := func() {
		_ = client.Close()
		_ = server.Close()
	}
	return bot, server, cleanup
}

// readFromConn reads everything available from conn until it blocks, with a short deadline.
func readFromConn(t *testing.T, conn net.Conn) string {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	return string(buf[:n])
}

// readAllFromConn reads multiple chunks until timeout, concatenating them.
func readAllFromConn(t *testing.T, conn net.Conn, reads int) string {
	t.Helper()
	var all string
	for i := 0; i < reads; i++ {
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		all += string(buf[:n])
	}
	return all
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

func TestMsgRegex_ValidPRIVMSG_WithCommand(t *testing.T) {
	line := ":user123!user123@user123.tmi.twitch.tv PRIVMSG #mychan :!dwarfbot ping"
	matches := msgRegex.FindStringSubmatch(line)
	if matches == nil {
		t.Fatal("expected msgRegex to match PRIVMSG with command")
	}
	if matches[1] != "user123" {
		t.Errorf("expected user123, got %q", matches[1])
	}
	if matches[3] != "mychan" {
		t.Errorf("expected mychan, got %q", matches[3])
	}
	if matches[4] != "!dwarfbot ping" {
		t.Errorf("expected '!dwarfbot ping', got %q", matches[4])
	}
}

func TestMsgRegex_EmptyMessage(t *testing.T) {
	// PRIVMSG with no message content (just colon)
	line := ":user!user@user.tmi.twitch.tv PRIVMSG #channel :"
	matches := msgRegex.FindStringSubmatch(line)
	if matches == nil {
		t.Fatal("expected msgRegex to match PRIVMSG with empty message body")
	}
	if matches[4] != "" {
		t.Errorf("expected empty message, got %q", matches[4])
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
		{"notice", ":user!user@user.tmi.twitch.tv NOTICE #channel :msg"},
		{"wrong domain", ":user!user@user.other.server PRIVMSG #channel :msg"},
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
		name    string
		msg     string
		wantCmd string
		wantArg string
	}{
		{"simple command", "!dwarfbot ping", "dwarfbot", "ping"},
		{"command with arg", "!dwarfbot channels list", "dwarfbot", "channels"},
		{"command with multiple args", "!dwarfbot say hello world", "dwarfbot", "say"},
		{"hammerdwarfbot alias", "!hammerdwarfbot ping", "hammerdwarfbot", "ping"},
		{"uppercase ignored", "!DwarfBot Ping", "DwarfBot", "Ping"},
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
	tests := []struct {
		name string
		msg  string
	}{
		{"plain text", "hello world"},
		{"no exclamation", "dwarfbot ping"},
		{"empty", ""},
		{"just exclamation", "!"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if cmdRegex.FindStringSubmatch(tc.msg) != nil {
				t.Errorf("expected no match for %q", tc.msg)
			}
		})
	}
}

// --- Aliases Tests ---

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

func TestAliases_ContainsDwarfbot(t *testing.T) {
	if !contains(aliases, "dwarfbot") {
		t.Error("expected aliases to contain 'dwarfbot'")
	}
}

func TestAliases_ContainsHammerdwarfbot(t *testing.T) {
	if !contains(aliases, "hammerdwarfbot") {
		t.Error("expected aliases to contain 'hammerdwarfbot'")
	}
}

// --- OAuthCreds Tests ---

func TestOAuthCreds_Fields(t *testing.T) {
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

func TestOAuthCreds_EmptyFields(t *testing.T) {
	creds := &OAuthCreds{}
	if creds.Name != "" {
		t.Errorf("expected empty name, got %q", creds.Name)
	}
	if creds.Token != "" {
		t.Errorf("expected empty token, got %q", creds.Token)
	}
}

// --- DwarfBot struct Tests ---

func TestDwarfBot_DefaultFields(t *testing.T) {
	bot := &DwarfBot{}
	if bot.Name != "" {
		t.Errorf("expected empty name, got %q", bot.Name)
	}
	if bot.Server != "" {
		t.Errorf("expected empty server, got %q", bot.Server)
	}
	if bot.Port != "" {
		t.Errorf("expected empty port, got %q", bot.Port)
	}
	if bot.Verbose {
		t.Error("expected Verbose to be false by default")
	}
	if len(bot.Channels) != 0 {
		t.Errorf("expected empty channels, got %v", bot.Channels)
	}
}

func TestDwarfBot_Fields(t *testing.T) {
	bot := &DwarfBot{
		Name:     "testbot",
		Server:   "irc.test.tv",
		Port:     "1234",
		Verbose:  true,
		Channels: []string{"ch1", "ch2"},
		Credentials: &OAuthCreds{
			Name:  "testbot",
			Token: "tok",
		},
	}
	if bot.Name != "testbot" {
		t.Errorf("expected 'testbot', got %q", bot.Name)
	}
	if bot.Server != "irc.test.tv" {
		t.Errorf("expected 'irc.test.tv', got %q", bot.Server)
	}
	if bot.Port != "1234" {
		t.Errorf("expected '1234', got %q", bot.Port)
	}
	if !bot.Verbose {
		t.Error("expected Verbose to be true")
	}
	if len(bot.Channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(bot.Channels))
	}
	if bot.Credentials.Token != "tok" {
		t.Errorf("expected token 'tok', got %q", bot.Credentials.Token)
	}
}

// --- Say Tests ---

func TestSay_Success(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		if err := bot.Say("testchannel", "Hello!"); err != nil {
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
	if err.Error() != "msg was empty" {
		t.Errorf("expected 'msg was empty', got %q", err.Error())
	}
}

func TestSay_SpecialCharacters(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		_ = bot.Say("testchannel", "Hello! @user #channel :colon")
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "Hello! @user #channel :colon") {
		t.Errorf("expected special characters preserved, got %q", got)
	}
}

func TestSay_ClosedConn(t *testing.T) {
	server, client := net.Pipe()
	bot := &DwarfBot{
		conn: client,
		Name: "testbot",
	}
	_ = client.Close()
	_ = server.Close()

	err := bot.Say("channel", "test message")
	if err == nil {
		t.Error("expected error writing to closed connection")
	}
}

// --- Authenticate Tests ---

func TestAuthenticate(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go bot.Authenticate()

	all := readAllFromConn(t, server, 2)
	if !strings.Contains(all, "PASS oauth:oauth_test_token\r\n") {
		t.Errorf("expected PASS command, got %q", all)
	}
	if !strings.Contains(all, "NICK testbot\r\n") {
		t.Errorf("expected NICK command, got %q", all)
	}
}

func TestAuthenticate_TokenFormat(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	bot.Credentials.Token = "myspecialtoken123"
	defer cleanup()

	go bot.Authenticate()

	all := readAllFromConn(t, server, 2)
	if !strings.Contains(all, "PASS oauth:myspecialtoken123\r\n") {
		t.Errorf("expected token in PASS command, got %q", all)
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

func TestJoinChannel_AlreadyLowercase(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go bot.JoinChannel("lowerchannel")

	got := readFromConn(t, server)
	if got != "JOIN #lowerchannel\r\n" {
		t.Errorf("expected lowercase channel, got %q", got)
	}
}

func TestJoinChannel_MixedCase(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go bot.JoinChannel("MiXeDcAsE")

	got := readFromConn(t, server)
	if got != "JOIN #mixedcase\r\n" {
		t.Errorf("expected lowercase 'mixedcase', got %q", got)
	}
}

func TestJoinChannel_Empty(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	bot.JoinChannel("")

	_ = server.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	buf := make([]byte, 256)
	n, _ := server.Read(buf)
	if n > 0 {
		t.Errorf("expected no data for empty channel, got %q", string(buf[:n]))
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

func TestPartChannel_Lowercase(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go bot.PartChannel("UPPER")

	got := readFromConn(t, server)
	if got != "PART #upper\r\n" {
		t.Errorf("expected lowercase, got %q", got)
	}
}

func TestPartChannel_Empty(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	bot.PartChannel("")

	_ = server.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
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

func TestDie_ZeroExitCode(t *testing.T) {
	var exitCode int
	bot := &DwarfBot{
		exitFunc: func(code int) {
			exitCode = code
		},
	}
	bot.Die(0)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestDie_NonZeroExitCode(t *testing.T) {
	var exitCode int
	bot := &DwarfBot{
		exitFunc: func(code int) {
			exitCode = code
		},
	}
	bot.Die(1)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
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

func TestDisconnect_RecordsTime(t *testing.T) {
	_, client := net.Pipe()
	startTime := time.Now().Add(-5 * time.Second)
	bot := &DwarfBot{
		conn:      client,
		startTime: startTime,
	}
	// Should complete without panic; elapsed time will be logged
	bot.Disconnect()
}

// --- HandleChat Tests ---

func TestHandleChat_Ping(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		_, _ = server.Write([]byte("PING :tmi.twitch.tv\r\n"))
		buf := make([]byte, 4096)
		_ = server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _ := server.Read(buf)
		response := string(buf[:n])
		if !strings.Contains(response, "PONG :tmi.twitch.tv") {
			t.Errorf("expected PONG response, got %q", response)
		}
		_ = server.Close()
	}()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

func TestHandleChat_PrivmsgCommand(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		line := ":someuser!someuser@someuser.tmi.twitch.tv PRIVMSG #testchannel :!dwarfbot ping\r\n"
		_, _ = server.Write([]byte(line))

		reader := bufio.NewReader(server)
		_ = server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		response, _ := reader.ReadString('\n')
		if !strings.Contains(response, "PRIVMSG #testchannel") {
			t.Errorf("expected PRIVMSG response, got %q", response)
		}
		if !strings.Contains(response, "Atari") {
			t.Errorf("expected ping response containing 'Atari', got %q", response)
		}
		_ = server.Close()
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
		_, _ = server.Write([]byte("some random IRC line\r\n"))
		time.Sleep(50 * time.Millisecond)
		_ = server.Close()
	}()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

func TestHandleChat_WrongBotAlias(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		line := ":someuser!someuser@someuser.tmi.twitch.tv PRIVMSG #testchannel :!otherbot ping\r\n"
		_, _ = server.Write([]byte(line))
		time.Sleep(50 * time.Millisecond)
		_ = server.Close()
	}()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

func TestHandleChat_Verbose(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	bot.Verbose = true
	defer cleanup()

	go func() {
		line := ":user!user@user.tmi.twitch.tv PRIVMSG #testchannel :!dwarfbot ping\r\n"
		_, _ = server.Write([]byte(line))
		_ = server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, _ = io.ReadAll(server)
		_ = server.Close()
	}()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

func TestHandleChat_ErrorMessage(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	// Close immediately to trigger error
	_ = server.Close()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
	// Error message should always include the underlying error
	if !strings.Contains(err.Error(), "failed to read line from channel") {
		t.Errorf("expected disconnect error message, got %q", err.Error())
	}
}

func TestHandleChat_ChannelsCommand(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		line := ":someuser!someuser@someuser.tmi.twitch.tv PRIVMSG #testchannel :!dwarfbot channels\r\n"
		_, _ = server.Write([]byte(line))

		reader := bufio.NewReader(server)
		_ = server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		response, _ := reader.ReadString('\n')
		if !strings.Contains(response, "testbot") {
			t.Errorf("expected bot name in channels response, got %q", response)
		}
		if !strings.Contains(response, "channel1") || !strings.Contains(response, "channel2") {
			t.Errorf("expected channels in response, got %q", response)
		}
		_ = server.Close()
	}()

	_ = bot.HandleChat()
}

func TestHandleChat_AdminShutdown(t *testing.T) {
	shutdownCalled := false
	bot, server, cleanup := newTestBot(t)
	bot.exitFunc = func(code int) {
		shutdownCalled = true
	}
	defer cleanup()

	go func() {
		line := fmt.Sprintf(":%s!%s@%s.tmi.twitch.tv PRIVMSG #%s :!dwarfbot shutdown\r\n",
			"testchannel", "testchannel", "testchannel", "testchannel")
		_, _ = server.Write([]byte(line))

		reader := bufio.NewReader(server)
		_ = server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		response, _ := reader.ReadString('\n')
		if !strings.Contains(response, "Shuttin") {
			t.Errorf("expected shutdown message, got %q", response)
		}
		_ = server.Close()
	}()

	_ = bot.HandleChat()

	if !shutdownCalled {
		t.Error("expected shutdown/Die to be called for admin shutdown command")
	}
}

func TestHandleChat_MultiplePings(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		// Send two pings
		_, _ = server.Write([]byte("PING :tmi.twitch.tv\r\n"))
		buf := make([]byte, 4096)
		_ = server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, _ = server.Read(buf) // read first PONG

		_, _ = server.Write([]byte("PING :tmi.twitch.tv\r\n"))
		_ = server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _ := server.Read(buf)
		response := string(buf[:n])
		if !strings.Contains(response, "PONG") {
			t.Errorf("expected second PONG, got %q", response)
		}
		_ = server.Close()
	}()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

func TestHandleChat_HammerdwarfbotAlias(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		line := ":someuser!someuser@someuser.tmi.twitch.tv PRIVMSG #testchannel :!hammerdwarfbot ping\r\n"
		_, _ = server.Write([]byte(line))

		reader := bufio.NewReader(server)
		_ = server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		response, _ := reader.ReadString('\n')
		if !strings.Contains(response, "PRIVMSG #testchannel") {
			t.Errorf("expected PRIVMSG response for hammerdwarfbot alias, got %q", response)
		}
		_ = server.Close()
	}()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

// --- ChatPlatform interface implementation Tests ---

func TestDwarfBot_SendMessage(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		if err := bot.SendMessage("testchannel", "Hello via interface!"); err != nil {
			t.Errorf("SendMessage returned error: %v", err)
		}
	}()

	got := readFromConn(t, server)
	expected := "PRIVMSG #testchannel :Hello via interface!\r\n"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestDwarfBot_SendMessage_Empty(t *testing.T) {
	bot, _, cleanup := newTestBot(t)
	defer cleanup()

	err := bot.SendMessage("ch", "")
	if err == nil {
		t.Error("expected error for empty message via SendMessage")
	}
}

func TestDwarfBot_IsAdmin(t *testing.T) {
	bot := &DwarfBot{Name: "testbot"}

	if !bot.IsAdmin("owner", "owner") {
		t.Error("expected IsAdmin to return true when user == channel")
	}
	if bot.IsAdmin("owner", "other") {
		t.Error("expected IsAdmin to return false when user != channel")
	}
	// Note: empty == empty is true by design, matching Twitch behavior
	if !bot.IsAdmin("", "") {
		t.Error("expected IsAdmin to return true when both are empty (user == channel)")
	}
}

func TestDwarfBot_BotName(t *testing.T) {
	bot := &DwarfBot{Name: "mybot"}
	if bot.BotName() != "mybot" {
		t.Errorf("expected 'mybot', got %q", bot.BotName())
	}
}

func TestDwarfBot_BotName_Empty(t *testing.T) {
	bot := &DwarfBot{}
	if bot.BotName() != "" {
		t.Errorf("expected empty name, got %q", bot.BotName())
	}
}

func TestDwarfBot_BotChannels(t *testing.T) {
	bot := &DwarfBot{Channels: []string{"a", "b", "c"}}
	channels := bot.BotChannels()
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	if channels[0] != "a" || channels[1] != "b" || channels[2] != "c" {
		t.Errorf("expected [a b c], got %v", channels)
	}
}

func TestDwarfBot_BotChannels_Empty(t *testing.T) {
	bot := &DwarfBot{}
	if len(bot.BotChannels()) != 0 {
		t.Errorf("expected empty channels, got %v", bot.BotChannels())
	}
}

func TestDwarfBot_Shutdown(t *testing.T) {
	exitCalled := false
	var exitCode int
	_, client := net.Pipe()
	bot := &DwarfBot{
		conn:      client,
		startTime: time.Now(),
		exitFunc: func(code int) {
			exitCalled = true
			exitCode = code
		},
	}

	bot.Shutdown(0)

	if !exitCalled {
		t.Error("expected exitFunc to be called during Shutdown")
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

// --- Disconnect edge cases ---

func TestDisconnect_NilConn(t *testing.T) {
	bot := &DwarfBot{}
	// Should not panic with nil conn
	bot.Disconnect()
}

func TestDisconnect_DoubleClose(t *testing.T) {
	_, client := net.Pipe()
	bot := &DwarfBot{
		conn:      client,
		startTime: time.Now(),
	}
	// First close succeeds
	bot.Disconnect()
	// Second close on already-closed conn should log error but not panic
	bot.conn = client // restore (already closed)
	bot.Disconnect()
}

// --- Authenticate error paths ---

func TestAuthenticate_ClosedConn(t *testing.T) {
	_, client := net.Pipe()
	_ = client.Close()
	bot := &DwarfBot{
		conn: client,
		Name: "testbot",
		Credentials: &OAuthCreds{
			Token: "test_token",
		},
	}
	// Should log errors but not panic
	bot.Authenticate()
}

// --- JoinChannel error paths ---

func TestJoinChannel_ClosedConn(t *testing.T) {
	_, client := net.Pipe()
	_ = client.Close()
	bot := &DwarfBot{
		conn: client,
		Name: "testbot",
	}
	// Should log error and return without panic
	bot.JoinChannel("testchannel")
}

// --- PartChannel error paths ---

func TestPartChannel_ClosedConn(t *testing.T) {
	_, client := net.Pipe()
	_ = client.Close()
	bot := &DwarfBot{
		conn: client,
		Name: "testbot",
	}
	// Should log error and return without panic
	bot.PartChannel("testchannel")
}

// --- HandleChat PONG write error ---

func TestHandleChat_PongWriteError(t *testing.T) {
	server, client := net.Pipe()
	bot := &DwarfBot{
		conn:      client,
		Name:      "testbot",
		startTime: time.Now(),
		exitFunc:  func(int) {},
	}

	go func() {
		// Send PING then immediately close server so PONG write fails
		_, _ = server.Write([]byte("PING :tmi.twitch.tv\r\n"))
		// Give the bot time to process the PING
		time.Sleep(50 * time.Millisecond)
		_ = server.Close()
	}()

	err := bot.HandleChat()
	// Should get an error - either PONG write fails or read fails
	if err == nil {
		t.Error("expected error when PONG write fails or connection closes")
	}
}

// --- Shutdown with nil conn ---

func TestDwarfBot_Shutdown_NilConn(t *testing.T) {
	exitCalled := false
	bot := &DwarfBot{
		exitFunc: func(code int) {
			exitCalled = true
		},
	}
	// Should not panic with nil conn
	bot.Shutdown(0)
	if !exitCalled {
		t.Error("expected exitFunc to be called")
	}
}

// --- SendMessage via closed connection ---

func TestDwarfBot_SendMessage_ClosedConn(t *testing.T) {
	_, client := net.Pipe()
	_ = client.Close()
	bot := &DwarfBot{
		conn: client,
		Name: "testbot",
	}
	err := bot.SendMessage("ch", "hello")
	if err == nil {
		t.Error("expected error sending to closed connection")
	}
}

// --- Metrics integration tests ---

func TestDwarfBot_Disconnect_RecordsMetrics(t *testing.T) {
	rec := newMockMetricsRecorder()
	_, client := net.Pipe()
	bot := &DwarfBot{
		conn:      client,
		startTime: time.Now().Add(-5 * time.Second),
		Metrics:   rec,
	}

	bot.Disconnect()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.disconnected) != 1 {
		t.Fatalf("expected 1 disconnect recorded, got %d", len(rec.disconnected))
	}
	if rec.disconnected[0].platform != "twitch" {
		t.Errorf("expected platform 'twitch', got %q", rec.disconnected[0].platform)
	}
	if rec.disconnected[0].reason != "shutdown" {
		t.Errorf("expected reason 'shutdown', got %q", rec.disconnected[0].reason)
	}
	if len(rec.connectionDurations) != 1 {
		t.Fatalf("expected 1 duration recorded, got %d", len(rec.connectionDurations))
	}
}

func TestDwarfBot_Disconnect_ErrorReason(t *testing.T) {
	rec := newMockMetricsRecorder()
	_, client := net.Pipe()
	bot := &DwarfBot{
		conn:                 client,
		startTime:            time.Now(),
		Metrics:              rec,
		lastDisconnectReason: "read_error",
	}

	bot.Disconnect()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.disconnected[0].reason != "read_error" {
		t.Errorf("expected reason 'read_error', got %q", rec.disconnected[0].reason)
	}
}

func TestDwarfBot_SendMessage_RecordsSuccessMetric(t *testing.T) {
	rec := newMockMetricsRecorder()
	bot, server, cleanup := newTestBot(t)
	bot.Metrics = rec
	defer cleanup()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := bot.SendMessage("testchannel", "hello"); err != nil {
			t.Errorf("SendMessage error: %v", err)
		}
	}()

	_ = readFromConn(t, server)
	<-done

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.messagesSent) != 1 {
		t.Fatalf("expected 1 message sent metric, got %d", len(rec.messagesSent))
	}
	if rec.messagesSent[0].result != "success" {
		t.Errorf("expected result 'success', got %q", rec.messagesSent[0].result)
	}
}

func TestDwarfBot_SendMessage_RecordsFailureMetric(t *testing.T) {
	rec := newMockMetricsRecorder()
	_, client := net.Pipe()
	_ = client.Close()
	bot := &DwarfBot{
		conn:    client,
		Name:    "testbot",
		Metrics: rec,
	}

	_ = bot.SendMessage("ch", "hello")

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.messagesSent) != 1 {
		t.Fatalf("expected 1 message sent metric, got %d", len(rec.messagesSent))
	}
	if rec.messagesSent[0].result != "failure" {
		t.Errorf("expected result 'failure', got %q", rec.messagesSent[0].result)
	}
}

func TestDwarfBot_HandleChat_RecordsMessageReceived(t *testing.T) {
	rec := newMockMetricsRecorder()
	bot, server, cleanup := newTestBot(t)
	bot.Metrics = rec
	defer cleanup()

	go func() {
		line := ":someuser!someuser@someuser.tmi.twitch.tv PRIVMSG #testchannel :!dwarfbot ping\r\n"
		_, _ = server.Write([]byte(line))
		// Read the response
		buf := make([]byte, 4096)
		_ = server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, _ = server.Read(buf)
		_ = server.Close()
	}()

	_ = bot.HandleChat()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.messagesReceived) == 0 {
		t.Error("expected message received metric to be recorded")
	}
}

func TestParseCommand_RecordsCommandMetric(t *testing.T) {
	rec := newMockMetricsRecorder()
	mock := newMockPlatform("testbot", []string{"ch1"})

	_ = parseCommand(mock, "ch1", "user", "ping", []string{}, parseCommandOpts{
		metrics:      rec,
		platformName: "twitch",
	})

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.commandsProcessed) != 1 {
		t.Fatalf("expected 1 command metric, got %d", len(rec.commandsProcessed))
	}
	if rec.commandsProcessed[0].platform != "twitch" {
		t.Errorf("expected platform 'twitch', got %q", rec.commandsProcessed[0].platform)
	}
	if rec.commandsProcessed[0].command != "ping" {
		t.Errorf("expected command 'ping', got %q", rec.commandsProcessed[0].command)
	}
	if rec.commandsProcessed[0].admin != "false" {
		t.Errorf("expected admin 'false', got %q", rec.commandsProcessed[0].admin)
	}
}

func TestParseCommand_RecordsAdminMetric(t *testing.T) {
	rec := newMockMetricsRecorder()
	mock := newMockPlatformWithAdmin("testbot", []string{"ch1"}, func(ch, user string) bool {
		return user == "admin"
	})

	_ = parseCommand(mock, "ch1", "admin", "ping", []string{}, parseCommandOpts{
		metrics:      rec,
		platformName: "discord",
	})

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.commandsProcessed) != 1 {
		t.Fatalf("expected 1 command metric, got %d", len(rec.commandsProcessed))
	}
	if rec.commandsProcessed[0].admin != "true" {
		t.Errorf("expected admin 'true', got %q", rec.commandsProcessed[0].admin)
	}
}

func TestParseCommand_UnknownCommandNormalized(t *testing.T) {
	rec := newMockMetricsRecorder()
	mock := newMockPlatform("testbot", []string{"ch1"})

	_ = parseCommand(mock, "ch1", "user", "xyzgarbage", []string{}, parseCommandOpts{
		metrics:      rec,
		platformName: "twitch",
	})

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.commandsProcessed) != 1 {
		t.Fatalf("expected 1 command metric, got %d", len(rec.commandsProcessed))
	}
	if rec.commandsProcessed[0].command != "unknown" {
		t.Errorf("expected command label 'unknown' for unrecognized command, got %q", rec.commandsProcessed[0].command)
	}
}

func TestNormalizeCommandLabel(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"ping", "ping"},
		{"channels", "channels"},
		{"shutdown", "shutdown"},
		{"unknown_cmd", "unknown"},
		{"", "unknown"},
		{"PING", "unknown"}, // case sensitive
	}
	for _, tt := range tests {
		if got := normalizeCommandLabel(tt.input); got != tt.expected {
			t.Errorf("normalizeCommandLabel(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// Verify DwarfBot satisfies ChatPlatform at compile time
var _ ChatPlatform = (*DwarfBot)(nil)
