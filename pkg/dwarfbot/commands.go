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
	var err error

	switch cmd {
	case "shutdown":
		platform.SendMessage(channelName, "Yah, boss! Shuttin' 'er doon!")
		platform.Shutdown(0)
		return nil
	}
	return err
}

func parseCommand(platform ChatPlatform, channelName string, userName string, cmd string, arguments []string) error {
	var err error

	if platform.IsAdmin(channelName, userName) {
		log.Printf("Received orders from the boss...")
		parseAdminCommand(platform, channelName, cmd, arguments)
	}

	switch cmd {
	case "ping":
		ping(platform, channelName, arguments)
	case "channels":
		channels(platform, channelName, arguments)
	}

	return err
}

func ping(platform ChatPlatform, channelName string, arguments []string) error {
	re := regexp.MustCompile(`(?i)heyo.+`)

	switch {
	case contains(arguments, strings.ToLower("heyo")):
		platform.SendMessage(channelName, "Heyo, yourself boy-o!")
	case reContains(arguments, re):
		platform.SendMessage(channelName, "Heyo, yourself boy-o!")
	default:
		platform.SendMessage(channelName, "Ach! I dunnae own 'n Atari, but nevertheless: \"Pong\"")
	}

	return nil
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

func channels(platform ChatPlatform, channelName string, arguments []string) error {
	msg := fmt.Sprintf("Aye, I like ta hang about here: %s", platform.BotName())
	for _, channel := range platform.BotChannels() {
		msg = msg + fmt.Sprintf(" %s", channel)
	}
	platform.SendMessage(channelName, msg)
	return nil
}
