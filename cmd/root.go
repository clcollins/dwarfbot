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
	"context"
	"dwarfbot/pkg/dwarfbot"
	"dwarfbot/pkg/metrics"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Default Twitch values
var (
	twitchChatServer string = "irc.chat.twitch.tv"
	twitchChatPort   string = "6667"
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
		metricsPort := viper.GetString("metrics_port")

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

		// Initialize metrics
		version := "dev"
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
			version = info.Main.Version
		}
		m := metrics.New()
		m.Init(version, time.Now())
		m.SetConfigMetrics(twitchToken, discordToken, twitchChannels, discordChannels)
		recorder := metrics.NewRecorder(m)

		// Start metrics HTTP server
		metricsSrv := metrics.NewServer(":"+metricsPort, m.Registry)
		go func() {
			log.Printf("Metrics server listening on :%s", metricsPort)
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Metrics server error: %v", err)
			}
		}()

		// Start Discord bot if configured (non-fatal on failure)
		var discordBot *dwarfbot.DiscordBot
		discordRunning := false
		if discordEnabled {
			discordBot = &dwarfbot.DiscordBot{
				Token:      discordToken,
				ChannelIDs: discordChannels,
				AdminRole:  discordAdminRole,
				Name:       name,
				Metrics:    recorder,
			}

			if err := discordBot.Start(); err != nil {
				log.Printf("WARNING: Failed to start Discord bot: %v", err)
				discordBot = nil
			} else {
				discordRunning = true
				defer func() {
					if err := discordBot.Stop(); err != nil {
						log.Printf("Failed to stop Discord bot: %v", err)
					}
				}()
				log.Println("Discord bot is running")
			}
		}

		// Handle graceful shutdown via signal
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)

		// Start Twitch bot if configured (non-fatal on failure)
		twitchErrCh := make(chan error, 1)
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
				Metrics:  recorder,
			}

			go func() {
				twitchErrCh <- bot.Start()
			}()
		}

		// Verify at least one platform started
		if !discordRunning && !twitchEnabled {
			log.Fatal("No platforms started successfully")
		}

		// Wait for signal or Twitch failure
		select {
		case <-sc:
			log.Println("Shutting down...")
		case err := <-twitchErrCh:
			if err != nil {
				log.Printf("Twitch bot exited with error: %v", err)
				if !discordRunning {
					log.Fatal("All platforms have failed, exiting")
				}
				log.Println("Continuing with Discord only")
				<-sc
				log.Println("Shutting down...")
			}
		}

		// Gracefully shut down metrics server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsSrv.Shutdown(ctx); err != nil {
			log.Printf("Metrics server shutdown error: %v", err)
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
	cobra.CheckErr(viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server")))

	rootCmd.PersistentFlags().StringP("port", "p", twitchChatPort, fmt.Sprintf("Twitch IRC port (default: %s)", twitchChatPort))
	cobra.CheckErr(viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port")))

	rootCmd.PersistentFlags().StringSliceP("channels", "c", []string{}, "Twitch channels to participate in")
	cobra.CheckErr(viper.BindPFlag("channels", rootCmd.PersistentFlags().Lookup("channels")))

	// General configuration
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose logging")
	cobra.CheckErr(viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose")))

	rootCmd.PersistentFlags().StringP("name", "n", "", "bot display name")
	cobra.CheckErr(viper.BindPFlag("name", rootCmd.PersistentFlags().Lookup("name")))

	// Discord configuration
	rootCmd.PersistentFlags().String("discord-token", "", "Discord bot token")
	cobra.CheckErr(viper.BindPFlag("discord_token", rootCmd.PersistentFlags().Lookup("discord-token")))

	rootCmd.PersistentFlags().StringSlice("discord-channels", []string{}, "Discord channel IDs to listen in")
	cobra.CheckErr(viper.BindPFlag("discord_channels", rootCmd.PersistentFlags().Lookup("discord-channels")))

	rootCmd.PersistentFlags().String("discord-admin-role", "dwarfbot-admin", "Discord role name for admin commands")
	cobra.CheckErr(viper.BindPFlag("discord_admin_role", rootCmd.PersistentFlags().Lookup("discord-admin-role")))

	// Metrics configuration
	rootCmd.PersistentFlags().String("metrics-port", "8080", "Port for Prometheus metrics HTTP server")
	cobra.CheckErr(viper.BindPFlag("metrics_port", rootCmd.PersistentFlags().Lookup("metrics-port")))
}

// initConfig reads in config file and ENV variables if set.
// If no config file is found and none was explicitly requested via --config,
// the bot falls back to environment variables (DWARFBOT_*) and CLI flags.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
			viper.SetConfigType("yaml")
			viper.SetConfigName(".dwarfbot")
		}
	}

	viper.SetEnvPrefix("DWARFBOT")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if cfgFile != "" {
			// User explicitly requested a config file that failed to load
			log.Fatalf("Error reading config file: %s", err.Error())
		}
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			log.Println("No config file found; using environment variables and/or CLI flags")
		} else {
			log.Fatalf("Error reading config file: %s", err.Error())
		}
	} else {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
