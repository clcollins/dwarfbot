package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestRootCommandExists(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd should not be nil")
	}
}

func TestRootCommandUse(t *testing.T) {
	if rootCmd.Use != "dwarfbot" {
		t.Errorf("expected Use 'dwarfbot', got %q", rootCmd.Use)
	}
}

func TestRootCommandShort(t *testing.T) {
	if rootCmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

func TestRootCommandLong(t *testing.T) {
	if rootCmd.Long == "" {
		t.Error("expected non-empty Long description")
	}
}

func TestRootCommandHasRunFunc(t *testing.T) {
	if rootCmd.Run == nil {
		t.Error("expected Run function to be set")
	}
}

func TestRootCommandHasFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
	}{
		{"config", ""},
		{"twitch-token", ""},
		{"twitch-server", ""},
		{"twitch-port", ""},
		{"twitch-channels", ""},
		{"verbose", "v"},
		{"name", "n"},
		{"discord-token", ""},
		{"discord-channels", ""},
		{"discord-admin-role", ""},
		{"metrics-port", ""},
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

func TestTwitchTokenFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("twitch-token")
	if flag == nil {
		t.Fatal("twitch-token flag not found")
	}
	if flag.DefValue != "" {
		t.Errorf("expected empty twitch-token default, got %q", flag.DefValue)
	}
}

func TestTwitchServerFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("twitch-server")
	if flag == nil {
		t.Fatal("twitch-server flag not found")
	}
	if flag.DefValue != twitchChatServer {
		t.Errorf("expected server default %q, got %q", twitchChatServer, flag.DefValue)
	}
}

func TestTwitchPortFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("twitch-port")
	if flag == nil {
		t.Fatal("twitch-port flag not found")
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

func TestNameFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("name")
	if flag == nil {
		t.Fatal("name flag not found")
	}
	if flag.DefValue != "" {
		t.Errorf("expected empty name default, got %q", flag.DefValue)
	}
}

func TestTwitchChannelsFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("twitch-channels")
	if flag == nil {
		t.Fatal("twitch-channels flag not found")
	}
	if flag.DefValue != "[]" {
		t.Errorf("expected '[]' twitch-channels default, got %q", flag.DefValue)
	}
}

func TestConfigFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("config")
	if flag == nil {
		t.Fatal("config flag not found")
	}
	if flag.DefValue != "" {
		t.Errorf("expected empty config default, got %q", flag.DefValue)
	}
}

// --- Discord flag tests ---

func TestDiscordTokenFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("discord-token")
	if flag == nil {
		t.Fatal("discord-token flag not found")
	}
	if flag.DefValue != "" {
		t.Errorf("expected empty discord-token default, got %q", flag.DefValue)
	}
}

func TestDiscordChannelsFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("discord-channels")
	if flag == nil {
		t.Fatal("discord-channels flag not found")
	}
	if flag.DefValue != "[]" {
		t.Errorf("expected '[]' discord-channels default, got %q", flag.DefValue)
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

func TestMetricsPortFlagDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("metrics-port")
	if flag == nil {
		t.Fatal("metrics-port flag not found")
	}
	if flag.DefValue != "8080" {
		t.Errorf("expected metrics-port default '8080', got %q", flag.DefValue)
	}
}

// --- initConfig tests ---

func TestInitConfig_NoConfigFile(t *testing.T) {
	// Point HOME at a directory with no .dwarfbot.yaml.
	// If initConfig() still fatals on missing config, this test will crash.
	t.Setenv("HOME", t.TempDir())
	savedCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = savedCfgFile }()

	viper.Reset()
	initConfig()
	// If we get here, initConfig gracefully handled missing config file
}

func TestInitConfig_EnvVarPrefix(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DWARFBOT_NAME", "envbot")
	t.Setenv("DWARFBOT_TWITCH_TOKEN", "env-test-token")
	savedCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = savedCfgFile }()

	viper.Reset()
	initConfig()

	if got := viper.GetString("name"); got != "envbot" {
		t.Errorf("expected DWARFBOT_NAME to set name='envbot', got %q", got)
	}
	if got := viper.GetString("twitch_token"); got != "env-test-token" {
		t.Errorf("expected DWARFBOT_TWITCH_TOKEN to set twitch_token='env-test-token', got %q", got)
	}
}

func TestInitConfig_DiscordEnvVars(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DWARFBOT_DISCORD_TOKEN", "discord-env-token")
	t.Setenv("DWARFBOT_DISCORD_ADMIN_ROLE", "custom-role")
	savedCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = savedCfgFile }()

	viper.Reset()
	initConfig()

	if got := viper.GetString("discord_token"); got != "discord-env-token" {
		t.Errorf("expected DWARFBOT_DISCORD_TOKEN to set discord_token, got %q", got)
	}
	if got := viper.GetString("discord_admin_role"); got != "custom-role" {
		t.Errorf("expected DWARFBOT_DISCORD_ADMIN_ROLE to set discord_admin_role, got %q", got)
	}
}

// --- Flag usage/help text tests ---

func TestFlagsHaveUsageText(t *testing.T) {
	flagNames := []string{
		"twitch-token", "twitch-server", "twitch-port", "twitch-channels",
		"verbose", "name",
		"discord-token", "discord-channels", "discord-admin-role",
	}
	for _, name := range flagNames {
		t.Run(name, func(t *testing.T) {
			flag := rootCmd.PersistentFlags().Lookup(name)
			if flag == nil {
				t.Fatalf("flag %q not found", name)
			}
			if flag.Usage == "" {
				t.Errorf("expected non-empty usage for flag %q", name)
			}
		})
	}
}

// TestFlagNamingConvention validates that all non-general CLI flags follow
// the <provider>-<item> naming convention (provider-prefixed).
func TestFlagNamingConvention(t *testing.T) {
	generalFlags := map[string]bool{
		"config":       true,
		"verbose":      true,
		"name":         true,
		"metrics-port": true,
	}
	providerPrefixes := []string{"twitch-", "discord-"}

	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if generalFlags[f.Name] {
			return
		}
		hasPrefix := false
		for _, p := range providerPrefixes {
			if strings.HasPrefix(f.Name, p) {
				hasPrefix = true
				break
			}
		}
		if !hasPrefix {
			t.Errorf("CLI flag %q does not follow <provider>-<item> naming convention (expected prefix: %v)", f.Name, providerPrefixes)
		}
	})
}
