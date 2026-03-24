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
		client.Close()
		server.Close()
	}
	return bot, server, cleanup
}

// readFromConn reads everything available from conn until it blocks, with a short deadline.
func readFromConn(t *testing.T, conn net.Conn) string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	return string(buf[:n])
}

// readAllFromConn reads multiple chunks until timeout, concatenating them.
func readAllFromConn(t *testing.T, conn net.Conn, reads int) string {
	t.Helper()
	var all string
	for i := 0; i < reads; i++ {
		conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
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
		bot.Say("testchannel", "Hello! @user #channel :colon")
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
	client.Close()
	server.Close()

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

	server.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
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
		server.Write([]byte("PING :tmi.twitch.tv\r\n"))
		buf := make([]byte, 4096)
		server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _ := server.Read(buf)
		response := string(buf[:n])
		if !strings.Contains(response, "PONG :tmi.twitch.tv") {
			t.Errorf("expected PONG response, got %q", response)
		}
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

	go func() {
		line := ":someuser!someuser@someuser.tmi.twitch.tv PRIVMSG #testchannel :!dwarfbot ping\r\n"
		server.Write([]byte(line))

		reader := bufio.NewReader(server)
		server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		response, _ := reader.ReadString('\n')
		if !strings.Contains(response, "PRIVMSG #testchannel") {
			t.Errorf("expected PRIVMSG response, got %q", response)
		}
		if !strings.Contains(response, "Atari") {
			t.Errorf("expected ping response containing 'Atari', got %q", response)
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
		server.Write([]byte("some random IRC line\r\n"))
		time.Sleep(50 * time.Millisecond)
		server.Close()
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
		server.Write([]byte(line))
		time.Sleep(50 * time.Millisecond)
		server.Close()
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
		server.Write([]byte(line))
		server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		io.ReadAll(server)
		server.Close()
	}()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
}

func TestHandleChat_VerboseErrorMessage(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	bot.Verbose = true
	defer cleanup()

	// Close immediately to trigger error with verbose message
	server.Close()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
	// Verbose mode includes the underlying error in parentheses
	if !strings.Contains(err.Error(), "(") {
		t.Errorf("expected verbose error message with parentheses, got %q", err.Error())
	}
}

func TestHandleChat_NonVerboseErrorMessage(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	bot.Verbose = false
	defer cleanup()

	server.Close()

	err := bot.HandleChat()
	if err == nil {
		t.Error("expected error after connection close")
	}
	// Non-verbose should not include underlying error details
	if strings.Contains(err.Error(), "(") {
		t.Errorf("expected non-verbose error without parentheses, got %q", err.Error())
	}
}

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
		server.Write([]byte(line))

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

func TestHandleChat_MultiplePings(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		// Send two pings
		server.Write([]byte("PING :tmi.twitch.tv\r\n"))
		buf := make([]byte, 4096)
		server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		server.Read(buf) // read first PONG

		server.Write([]byte("PING :tmi.twitch.tv\r\n"))
		server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _ := server.Read(buf)
		response := string(buf[:n])
		if !strings.Contains(response, "PONG") {
			t.Errorf("expected second PONG, got %q", response)
		}
		server.Close()
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
		server.Write([]byte(line))

		reader := bufio.NewReader(server)
		server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		response, _ := reader.ReadString('\n')
		if !strings.Contains(response, "PRIVMSG #testchannel") {
			t.Errorf("expected PRIVMSG response for hammerdwarfbot alias, got %q", response)
		}
		server.Close()
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

// Verify DwarfBot satisfies ChatPlatform at compile time
var _ ChatPlatform = (*DwarfBot)(nil)
