package dwarfbot

import (
	"log"
	"regexp"
	"strings"
)

func parseAdminCommand(db *DwarfBot, cmd string, arguments []string) error {
	var err error

	switch cmd {
	case "shutdown":
		db.Say("Yah, boss! Shuttin' 'er doon!")
		db.Disconnect()
		db.Die(0)
		return nil
	}
	return err
}

func parseCommand(db *DwarfBot, userName string, cmd string, arguments []string) error {
	var err error

	if userName == db.Channel {
		log.Printf("Received orders from the boss...")
		parseAdminCommand(db, cmd, arguments)
	}

	switch cmd {
	case "ping":
		ping(db, arguments)
	}

	return err
}

func ping(db *DwarfBot, arguments []string) error {
	re := regexp.MustCompile(`(?i)heyo.+`)

	switch {
	case contains(arguments, strings.ToLower("heyo")):
		db.Say("Heyo, yourself boy-o!")
	case reContains(arguments, re):
		db.Say("Heyo, yourself boy-o!")
	default:
		db.Say("Ach! I dunnae own 'n Atari, but nevertheless: \"Pong\"")
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
