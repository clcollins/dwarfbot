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
	"log"
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
		name := viper.GetString("name")
		channel := viper.GetString("channel")
		server := viper.GetString("server")
		port := viper.GetString("port")
		verbose := viper.GetBool("verbose")

		clientSecret := viper.GetString("clientSecret")

		bot := dwarfbot.DwarfBot{
			Credentials: &dwarfbot.OAuthCreds{
				Name:         name,
				ClientSecret: clientSecret,
			},
			Verbose: verbose,
			Server:  server,
			Port:    port,
			Channel: channel,
			Name:    name,
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
	rootCmd.PersistentFlags().StringP("server", "s", twitchChatServer, fmt.Sprintf("server to connect to (default: %s)", twitchChatServer))
	viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server"))

	// Configure the port to use for the connection
	rootCmd.PersistentFlags().StringP("port", "p", twitchChatPort, fmt.Sprintf("port to connect to (default: %s)", twitchChatPort))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))

	// Configure the port to use for the connection
	rootCmd.PersistentFlags().StringP("channel", "c", "", "channel to participate in (required)")
	viper.BindPFlag("channel", rootCmd.PersistentFlags().Lookup("channel"))

	// Enable verbose logging to stdout
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose logging")
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// IRC Nick to connect with
	rootCmd.PersistentFlags().StringP("name", "n", "", "IRC Nick to connect as")
	viper.BindPFlag("name", rootCmd.PersistentFlags().Lookup("name"))
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
		log.Fatalf(err.Error())
	}
}
