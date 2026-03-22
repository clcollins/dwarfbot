package cmd

import (
	"testing"
)

func TestRootCommandExists(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd should not be nil")
	}
	if rootCmd.Use != "dwarfbot" {
		t.Errorf("expected Use 'dwarfbot', got %q", rootCmd.Use)
	}
}

func TestRootCommandHasFlags(t *testing.T) {
	flags := []struct {
		name     string
		shorthand string
	}{
		{"config", ""},
		{"server", "s"},
		{"port", "p"},
		{"channels", "c"},
		{"verbose", "v"},
		{"name", "n"},
		{"discord-token", ""},
		{"discord-channels", ""},
		{"discord-admin-role", ""},
	}

	for _, f := range flags {
		t.Run(f.name, func(t *testing.T) {
			flag := rootCmd.PersistentFlags().Lookup(f.name)
			if flag == nil {
				t.Fatalf("expected flag %q to exist", f.name)
			}
			if f.shorthand != "" && flag.Shorthand != f.shorthand {
				t.Errorf("expected shorthand %q, got %q", f.shorthand, flag.Shorthand)
			}
		})
	}
}

func TestDefaultServerAndPort(t *testing.T) {
	if twitchChatServer != "irc.chat.twitch.tv" {
		t.Errorf("expected default server 'irc.chat.twitch.tv', got %q", twitchChatServer)
	}
	if twitchChatPort != "6667" {
		t.Errorf("expected default port '6667', got %q", twitchChatPort)
	}
}

func TestServerFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("server")
	if flag == nil {
		t.Fatal("server flag not found")
	}
	if flag.DefValue != twitchChatServer {
		t.Errorf("expected server default %q, got %q", twitchChatServer, flag.DefValue)
	}
}

func TestPortFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("port")
	if flag == nil {
		t.Fatal("port flag not found")
	}
	if flag.DefValue != twitchChatPort {
		t.Errorf("expected port default %q, got %q", twitchChatPort, flag.DefValue)
	}
}

func TestVerboseFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("verbose")
	if flag == nil {
		t.Fatal("verbose flag not found")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected verbose default 'false', got %q", flag.DefValue)
	}
}

func TestDiscordAdminRoleDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("discord-admin-role")
	if flag == nil {
		t.Fatal("discord-admin-role flag not found")
	}
	if flag.DefValue != "dwarfbot-admin" {
		t.Errorf("expected discord-admin-role default 'dwarfbot-admin', got %q", flag.DefValue)
	}
}
