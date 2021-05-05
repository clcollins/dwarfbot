/*
MIT License

Copyright (c) 2021 Chris Collins

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
		channels := viper.GetStringSlice("channels")
		server := viper.GetString("server")
		port := viper.GetString("port")
		verbose := viper.GetBool("verbose")

		token := viper.GetString("token")

		bot := dwarfbot.DwarfBot{
			Credentials: &dwarfbot.OAuthCreds{
				Name:  name,
				Token: token,
			},
			Verbose:  verbose,
			Server:   server,
			Port:     port,
			Channels: channels,
			Name:     name,
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
	rootCmd.PersistentFlags().StringP("channels", "c", "", "channels to participate in (required, []string)")
	viper.BindPFlag("channels", rootCmd.PersistentFlags().Lookup("channels"))

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
