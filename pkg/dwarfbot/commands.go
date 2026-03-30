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

package dwarfbot

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

func parseAdminCommand(platform ChatPlatform, channelName string, cmd string, arguments []string) error {
	switch cmd {
	case "shutdown":
		if err := platform.SendMessage(channelName, "Yah, boss! Shuttin' 'er doon!"); err != nil {
			log.Printf("failed to send shutdown message to channel %s: %v", channelName, err)
		}
		platform.Shutdown(0)
		return nil
	}
	return nil
}

// parseCommandOpts holds optional parameters for parseCommand.
type parseCommandOpts struct {
	metrics      PlatformMetrics
	platformName string
}

func parseCommand(platform ChatPlatform, channelName string, userName string, cmd string, arguments []string, opts ...parseCommandOpts) error {
	isAdmin := platform.IsAdmin(channelName, userName)
	if isAdmin {
		log.Printf("Received orders from the boss...")
		if err := parseAdminCommand(platform, channelName, cmd, arguments); err != nil {
			return err
		}
	}

	if len(opts) > 0 && opts[0].metrics != nil {
		adminStr := "false"
		if isAdmin {
			adminStr = "true"
		}
		opts[0].metrics.RecordCommandProcessed(opts[0].platformName, normalizeCommandLabel(cmd), adminStr)
	}

	switch cmd {
	case "ping":
		return ping(platform, channelName, arguments)
	case "channels":
		return channels(platform, channelName, arguments)
	}

	return nil
}

func ping(platform ChatPlatform, channelName string, arguments []string) error {
	re := regexp.MustCompile(`(?i)heyo.+`)

	lowerArgs := make([]string, len(arguments))
	for i, a := range arguments {
		lowerArgs[i] = strings.ToLower(a)
	}

	switch {
	case contains(lowerArgs, "heyo"):
		return platform.SendMessage(channelName, "Heyo, yourself boy-o!")
	case reContains(lowerArgs, re):
		return platform.SendMessage(channelName, "Heyo, yourself boy-o!")
	default:
		return platform.SendMessage(channelName, "Ach! I dunnae own 'n Atari, but nevertheless: \"Pong\"")
	}
}

func reContains(list []string, re *regexp.Regexp) bool {
	for _, x := range list {
		if re.MatchString(x) {
			return true
		}
	}
	return false
}

func contains(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}

// knownCommands is the set of valid command names for metrics labels.
// Unknown commands are recorded as "unknown" to prevent unbounded cardinality.
var knownCommands = map[string]bool{
	"ping":     true,
	"channels": true,
	"shutdown": true,
}

func normalizeCommandLabel(cmd string) string {
	if knownCommands[cmd] {
		return cmd
	}
	return "unknown"
}

func channels(platform ChatPlatform, channelName string, arguments []string) error {
	msg := fmt.Sprintf("Aye, I like ta hang about here: %s", platform.BotName())
	for _, channel := range platform.BotChannels() {
		msg = msg + fmt.Sprintf(" %s", channel)
	}
	return platform.SendMessage(channelName, msg)
}
