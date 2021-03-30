/*
Copyright © 2021 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"dwarfbot/pkg/dwarfbot"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// Default Twitch values
var (
	twitchChatServer      string        = "irc.chat.twitch.tv"
	twitchChatPort        string        = "6667"
	twitchChatMessageRate time.Duration = time.Duration(20/30) * time.Millisecond
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "dwarfbot",
	Short: "A bot to assist with https://twitch.tv/hammerdwarf and Co.",
	Long: `Dwarfbot is a bot to assist with the Twitch channel for
	https://twitch.tv/hammerdwarf and related media, social and not.`,

	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		server, _ := cmd.Flags().GetString("server")
		port, _ := cmd.Flags().GetString("port")
		verbose, _ := cmd.Flags().GetBool("verbose")

		bot := dwarfbot.DwarfBot{
			Verbose: verbose,
			Server:  server,
			Port:    port,
		}
		bot.Start()
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

	// Configure the server to connect to
	rootCmd.Flags().StringP("server", "s", twitchChatServer, fmt.Sprintf("server to connect to (default: %s)", twitchChatServer))

	// Configure the port to use for the connection
	rootCmd.Flags().StringP("port", "p", twitchChatPort, fmt.Sprintf("port to connect to (default: %s)", twitchChatPort))

	// Configure the port to use for the connection
	rootCmd.Flags().StringP("channel", "c", "", "channel to participate in (required)")
	rootCmd.MarkFlagRequired("channel")

	// Enable verbose logging to stdout
	rootCmd.Flags().BoolP("verbose", "v", false, "enable verbose logging")
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

		// Search config in home directory with name ".dwarfbot" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".dwarfbot")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
