/*
MIT License

# Copyright (c) 2021 Chris Collins

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
package cmd

import (
	"dwarfbot/pkg/dwarfbot"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// Default Twitch values
var (
	twitchChatServer      string        = "irc.chat.twitch.tv"
	twitchChatPort        string        = "6667"
	twitchChatMessageRate time.Duration = 20 * time.Millisecond / 30
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "dwarfbot",
	Short: "A bot to assist with https://twitch.tv/hammerdwarf and Co.",
	Long: `Dwarfbot is a bot to assist with the Twitch channel for
	https://twitch.tv/hammerdwarf and related media, social and not.
	Supports both Twitch IRC and Discord channels.`,

	Run: func(cmd *cobra.Command, args []string) {
		name := viper.GetString("name")
		verbose := viper.GetBool("verbose")

		// Twitch config
		twitchToken := viper.GetString("token")
		twitchChannels := viper.GetStringSlice("channels")
		server := viper.GetString("server")
		port := viper.GetString("port")

		// Discord config
		discordToken := viper.GetString("discord_token")
		discordChannels := viper.GetStringSlice("discord_channels")
		discordAdminRole := viper.GetString("discord_admin_role")

		twitchEnabled := twitchToken != "" && len(twitchChannels) > 0
		discordEnabled := discordToken != "" && len(discordChannels) > 0

		if !twitchEnabled && !discordEnabled {
			log.Fatal("At least one platform must be configured (Twitch: token + channels, Discord: discord_token + discord_channels)")
		}

		// Start Discord bot if configured
		var discordBot *dwarfbot.DiscordBot
		if discordEnabled {
			discordBot = &dwarfbot.DiscordBot{
				Token:      discordToken,
				ChannelIDs: discordChannels,
				AdminRole:  discordAdminRole,
				Name:       name,
			}

			if err := discordBot.Start(); err != nil {
				log.Fatalf("Failed to start Discord bot: %v", err)
			}
			defer func() {
				_ = discordBot.Stop()
			}()
			log.Println("Discord bot is running")
		}

		// Start Twitch bot if configured (blocking)
		if twitchEnabled {
			bot := dwarfbot.DwarfBot{
				Credentials: &dwarfbot.OAuthCreds{
					Name:  name,
					Token: twitchToken,
				},
				Verbose:  verbose,
				Server:   server,
				Port:     port,
				Channels: twitchChannels,
				Name:     name,
			}

			bot.Start()
		} else {
			// Discord-only mode: wait for interrupt signal
			log.Println("Running in Discord-only mode. Press Ctrl+C to stop.")
			sc := make(chan os.Signal, 1)
			signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
			<-sc
			log.Println("Shutting down...")
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dwarfbot.yaml)")

	// Twitch configuration
	rootCmd.PersistentFlags().StringP("server", "s", twitchChatServer, fmt.Sprintf("Twitch IRC server (default: %s)", twitchChatServer))
	_ = viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server"))

	rootCmd.PersistentFlags().StringP("port", "p", twitchChatPort, fmt.Sprintf("Twitch IRC port (default: %s)", twitchChatPort))
	_ = viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))

	rootCmd.PersistentFlags().StringP("channels", "c", "", "Twitch channels to participate in ([]string)")
	_ = viper.BindPFlag("channels", rootCmd.PersistentFlags().Lookup("channels"))

	// General configuration
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose logging")
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	rootCmd.PersistentFlags().StringP("name", "n", "", "bot display name")
	_ = viper.BindPFlag("name", rootCmd.PersistentFlags().Lookup("name"))

	// Discord configuration
	rootCmd.PersistentFlags().String("discord-token", "", "Discord bot token")
	_ = viper.BindPFlag("discord_token", rootCmd.PersistentFlags().Lookup("discord-token"))

	rootCmd.PersistentFlags().StringSlice("discord-channels", []string{}, "Discord channel IDs to listen in")
	_ = viper.BindPFlag("discord_channels", rootCmd.PersistentFlags().Lookup("discord-channels"))

	rootCmd.PersistentFlags().String("discord-admin-role", "dwarfbot-admin", "Discord role name for admin commands")
	_ = viper.BindPFlag("discord_admin_role", rootCmd.PersistentFlags().Lookup("discord-admin-role"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".dwarfbot.yaml" (with extension).
		// Note, Viper appends the .yaml when searching
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".dwarfbot")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else {
		log.Fatalf("%s", err.Error())
	}
}
