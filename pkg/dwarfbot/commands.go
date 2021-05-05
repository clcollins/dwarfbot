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

func parseAdminCommand(db *DwarfBot, channelName string, cmd string, arguments []string) error {
	var err error

	switch cmd {
	case "shutdown":
		db.Say(channelName, "Yah, boss! Shuttin' 'er doon!")
		db.Disconnect()
		db.Die(0)
		return nil
	}
	return err
}

func parseCommand(db *DwarfBot, channelName string, userName string, cmd string, arguments []string) error {
	var err error

	if userName == channelName {
		log.Printf("Received orders from the boss...")
		parseAdminCommand(db, channelName, cmd, arguments)
	}

	switch cmd {
	case "ping":
		ping(db, channelName, arguments)
	case "channels":
		channels(db, channelName, arguments)
	}

	return err
}

func ping(db *DwarfBot, channelName string, arguments []string) error {
	re := regexp.MustCompile(`(?i)heyo.+`)

	switch {
	case contains(arguments, strings.ToLower("heyo")):
		db.Say(channelName, "Heyo, yourself boy-o!")
	case reContains(arguments, re):
		db.Say(channelName, "Heyo, yourself boy-o!")
	default:
		db.Say(channelName, "Ach! I dunnae own 'n Atari, but nevertheless: \"Pong\"")
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

func channels(db *DwarfBot, channelName string, arguments []string) error {
	msg := fmt.Sprintf("Aye, I like ta hang about here: %s", db.Name)
	for _, channel := range db.Channels {
		msg = msg + fmt.Sprintf(" %s", channel)
	}
	db.Say(channelName, msg)
	return nil
}
