package dwarfbot

import (
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

func TestContains_FirstElement(t *testing.T) {
	list := []string{"apple", "banana", "cherry"}
	if !contains(list, "apple") {
		t.Error("expected contains to find first element 'apple'")
	}
}

func TestContains_LastElement(t *testing.T) {
	list := []string{"apple", "banana", "cherry"}
	if !contains(list, "cherry") {
		t.Error("expected contains to find last element 'cherry'")
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

func TestContains_EmptyString(t *testing.T) {
	list := []string{"", "notempty"}
	if !contains(list, "") {
		t.Error("expected to find empty string in list")
	}
}

func TestContains_SingleElement(t *testing.T) {
	if !contains([]string{"only"}, "only") {
		t.Error("expected to find 'only' in single-element list")
	}
	if contains([]string{"only"}, "other") {
		t.Error("expected not to find 'other' in single-element list")
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

func TestReContains_PartialMatch(t *testing.T) {
	re := regexp.MustCompile(`heyo`)
	list := []string{"xheyox"}
	if !reContains(list, re) {
		t.Error("expected partial match to succeed")
	}
}

func TestReContains_CaseInsensitive(t *testing.T) {
	re := regexp.MustCompile(`(?i)hello`)
	list := []string{"HELLO", "world"}
	if !reContains(list, re) {
		t.Error("expected case-insensitive match")
	}
}

func TestReContains_MultipleMatches(t *testing.T) {
	re := regexp.MustCompile(`test`)
	list := []string{"test1", "test2", "test3"}
	if !reContains(list, re) {
		t.Error("expected match with multiple matching elements")
	}
}

// --- ping Tests ---

func TestPing_Default(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		_ = ping(bot, "testchannel", []string{})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "Atari") {
		t.Errorf("expected default pong response with 'Atari', got %q", got)
	}
	if !strings.Contains(got, "Pong") {
		t.Errorf("expected 'Pong' in default response, got %q", got)
	}
}

func TestPing_Heyo(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		_ = ping(bot, "testchannel", []string{"heyo"})
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
		_ = ping(bot, "testchannel", []string{"heyooo"})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "Heyo") {
		t.Errorf("expected heyo response for extended heyo, got %q", got)
	}
}

func TestPing_HeyoAmongOtherArgs(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		_ = ping(bot, "testchannel", []string{"something", "heyo"})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "Heyo") {
		t.Errorf("expected heyo response when heyo is among args, got %q", got)
	}
}

func TestPing_NonHeyoArgs(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		_ = ping(bot, "testchannel", []string{"something", "else"})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "Pong") {
		t.Errorf("expected default Pong for non-heyo args, got %q", got)
	}
}

func TestPing_ReturnsNil(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ping(bot, "testchannel", []string{})
	}()

	readFromConn(t, server)
	if err := <-errCh; err != nil {
		t.Errorf("expected nil error from ping, got %v", err)
	}
}

// --- channels Tests ---

func TestChannels(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		_ = channels(bot, "testchannel", []string{})
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
		_ = client.Close()
		_ = server.Close()
	}()

	go func() {
		_ = channels(bot, "testchannel", []string{})
	}()

	_ = server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 4096)
	n, _ := server.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "solobot") {
		t.Errorf("expected bot name, got %q", got)
	}
}

func TestChannels_ReturnsNil(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	errCh := make(chan error, 1)
	go func() {
		errCh <- channels(bot, "testchannel", []string{})
	}()

	readFromConn(t, server)
	if err := <-errCh; err != nil {
		t.Errorf("expected nil error from channels, got %v", err)
	}
}

func TestChannels_MessageFormat(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		_ = channels(bot, "testchannel", []string{})
	}()

	got := readFromConn(t, server)
	if !strings.Contains(got, "Aye, I like ta hang about here:") {
		t.Errorf("expected dwarf speech prefix, got %q", got)
	}
}

// --- parseCommand Tests ---

func TestParseCommand_Ping(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		_ = parseCommand(bot, "testchannel", "someuser", "ping", []string{})
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
		_ = parseCommand(bot, "testchannel", "someuser", "channels", []string{})
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
		_ = parseCommand(bot, "testchannel", "someuser", "unknowncmd", []string{})
	}()

	_ = server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 256)
	n, _ := server.Read(buf)
	if n > 0 {
		t.Errorf("expected no output for unknown command, got %q", string(buf[:n]))
	}
}

func TestParseCommand_ReturnsNoError(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	errCh := make(chan error, 1)
	go func() {
		errCh <- parseCommand(bot, "testchannel", "someuser", "ping", []string{})
	}()

	readFromConn(t, server)
	if err := <-errCh; err != nil {
		t.Errorf("expected nil error from parseCommand, got %v", err)
	}
}

// --- parseAdminCommand Tests ---

func TestParseCommand_AdminFromOwner(t *testing.T) {
	shutdownCh := make(chan int, 1)
	bot, server, cleanup := newTestBot(t)
	bot.exitFunc = func(code int) { shutdownCh <- code }
	defer cleanup()

	go func() {
		_ = parseCommand(bot, "owneruser", "owneruser", "shutdown", []string{})
	}()

	_ = server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 4096)
	n, _ := server.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "Shuttin") {
		t.Errorf("expected shutdown message, got %q", got)
	}
	select {
	case <-shutdownCh:
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Error("expected Die/exitFunc to be called for admin shutdown")
	}
}

func TestParseCommand_AdminFromNonOwner(t *testing.T) {
	shutdownCalled := false
	bot, server, cleanup := newTestBot(t)
	bot.exitFunc = func(code int) { shutdownCalled = true }
	defer cleanup()

	go func() {
		_ = parseCommand(bot, "owneruser", "regularuser", "shutdown", []string{})
	}()

	_ = server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 256)
	n, _ := server.Read(buf)
	if n > 0 {
		t.Errorf("expected no output for non-admin shutdown, got %q", string(buf[:n]))
	}
	if shutdownCalled {
		t.Error("expected Die/exitFunc NOT to be called for non-admin user")
	}
}

func TestParseAdminCommand_Shutdown(t *testing.T) {
	shutdownCh := make(chan int, 1)
	bot, server, cleanup := newTestBot(t)
	bot.exitFunc = func(code int) { shutdownCh <- code }
	defer cleanup()

	go func() {
		_ = parseAdminCommand(bot, "testchannel", "shutdown", []string{})
	}()

	_ = server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 4096)
	n, _ := server.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "Shuttin") {
		t.Errorf("expected shutdown message, got %q", got)
	}
	select {
	case <-shutdownCh:
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Error("expected Shutdown to be called")
	}
}

func TestParseAdminCommand_ShutdownReturnsNil(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	errCh := make(chan error, 1)
	go func() {
		errCh <- parseAdminCommand(bot, "testchannel", "shutdown", []string{})
	}()

	readFromConn(t, server)
	if err := <-errCh; err != nil {
		t.Errorf("expected nil error from shutdown, got %v", err)
	}
}

func TestParseAdminCommand_UnknownCommand(t *testing.T) {
	bot, server, cleanup := newTestBot(t)
	defer cleanup()

	go func() {
		_ = parseAdminCommand(bot, "testchannel", "unknownadmin", []string{})
	}()

	_ = server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 256)
	n, _ := server.Read(buf)
	if n > 0 {
		t.Errorf("expected no output for unknown admin command, got %q", string(buf[:n]))
	}
}

func TestParseAdminCommand_UnknownReturnsNilError(t *testing.T) {
	bot, _, cleanup := newTestBot(t)
	defer cleanup()

	err := parseAdminCommand(bot, "testchannel", "nonexistent", []string{})
	if err != nil {
		t.Errorf("expected nil error for unknown admin command, got %v", err)
	}
}
