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
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"net/textproto"
	"os"
	"regexp"
	"strings"
	"time"
)

// Bot aliases
var aliases = []string{"hammerdwarfbot", "dwarfbot"}

// Regex for parsing PRIVMSG strings.
//
// First matched group is the user's name, the second matched is the message type (PRIVMSG),
// the third group is the incomming channel, and the fourth is the content of the user's message.
var msgRegex *regexp.Regexp = regexp.MustCompile(`^:(\w+)!\w+@\w+\.tmi\.twitch\.tv (PRIVMSG) #(\w+)(?: :(.*))?$`)

// Regex for parsing user commands, from already parsed PRIVMSG strings.
//
// First matched group is the command name and the second matched group is the argument for the
// command.
var cmdRegex *regexp.Regexp = regexp.MustCompile(`^!(\w+)\s?(\w+)\s?(.+)?`)

type OAuthCreds struct {
	// Client ID
	Name string `json:"name,omitempty"`

	// Client Secret
	Token string `json:"token,omitempty"`
}

type Bot interface {
	// Start begins the bot loop, and tries to reconnect on error
	Start()

	// Connect to the IRC server
	Connect()

	// Disconnect from the IRC server
	Disconnect()

	// Shutdown the script
	Die()
}

type DwarfBot struct {
	// Channel to join, must be lowercase
	Channel string

	// Reference to the bot's connection to the server
	conn net.Conn

	// OAuth credential for authentication
	Credentials *OAuthCreds

	// Rate-limit bot messages. 20/30 millisecond is OK
	MsgRate time.Duration

	// Name of the bot used in chat
	Name string

	// Port for the IRC Server
	Port string

	// Domain of the IRC Server
	Server string

	// Start time (useful?)
	startTime time.Time

	// Verbose output
	Verbose bool
}

func (db *DwarfBot) Start() {
	var err error

	defer db.Disconnect()

	log.Println("dwarfbot is starting...")

	for {
		if db.Verbose {
			log.Println("dwarfbot is waiting for a command...")
		}

		db.Connect()
		db.Authenticate()
		db.JoinChannel(db.Channel)
		defer db.PartChannel(db.Channel)

		db.JoinChannel("hammerdwarfbot")
		defer db.PartChannel("hammerdwarfbot")

		err = db.HandleChat()
		if err != nil {
			log.Println(err)
			time.Sleep(time.Second)
			log.Println("restarting bot...")
		}
	}

}

func (db *DwarfBot) Connect() {
	var err error

	// Return if no server is specified
	if db.Server == "" || db.Port == "" {
		log.Fatalf("IRC server and port must be specified")
		return
	}

	// Creates connection to Twitch IRC server
	db.conn, err = net.Dial("tcp", db.Server+":"+db.Port)
	if err != nil {
		log.Printf("Failed connecting to %s, retrying...\n", db.Server)
		db.Connect()
		return
	}

	// Record connection time
	db.startTime = time.Now()
	log.Printf("Connected to %s", db.Server)

}

func (db *DwarfBot) Disconnect() {
	db.conn.Close()
	log.Printf("Connection closed; elapsed time %g", (time.Since(db.startTime).Seconds()))
}

func (db *DwarfBot) Authenticate() {
	db.conn.Write([]byte("PASS oauth:" + db.Credentials.Token + "\r\n"))
	db.conn.Write([]byte("NICK " + db.Name + "\r\n"))
}

// JoinChannel joins a specific IRC Channel
func (db *DwarfBot) JoinChannel(channel string) {
	if channel == "" {
		return
	}

	// Channel login must be lowercase (https://dev.twitch.tv/docs/irc/guide#syntax-notes)
	db.conn.Write([]byte("JOIN #" + strings.ToLower(channel) + "\r\n"))

	log.Printf("Joined channel #%s as @%s", channel, db.Name)
}

func (db *DwarfBot) PartChannel(channel string) {
	if channel == "" {
		return
	}

	db.conn.Write([]byte("PART #" + strings.ToLower(channel) + "\r\n"))
	log.Printf("Parted from channel #%s", channel)
}

// Handle shutdown for good commands
func (db *DwarfBot) Die(exitCode int) {
	os.Exit(exitCode)
}

// HandleChat is the main loop, listenting to incoming chat and responding
func (db *DwarfBot) HandleChat() error {
	tp := textproto.NewReader(bufio.NewReader(db.conn))

	for {
		line, err := tp.ReadLine()
		if err != nil {
			db.Disconnect()

			var errMsg string
			if db.Verbose {
				errMsg = fmt.Sprintf("(%s)", err)
			}

			return errors.New(fmt.Sprintf("Failed to read line from channel, disconnecting %s", errMsg))
		}

		if db.Verbose {
			log.Println(line)
		}

		if "PING :tmi.twitch.tv" == line {

			// Must reply to PING messages with PONG message to stay connected
			pong := "PONG :tmi.twitch.tv\r\n"
			db.conn.Write([]byte(pong))
			log.Print(pong)
			continue

		} else {

			// handle a PRIVMSG message
			matches := msgRegex.FindStringSubmatch(line)
			if matches != nil {
				userName := matches[1]
				msgType := matches[2]
				channelName := matches[3]

				if db.Verbose {
					log.Printf("User: %s, Channel: %s, Message Type: %s", userName, channelName, msgType)
				}

				switch msgType {
				case "PRIVMSG":
					msg := matches[4]
					log.Printf("%s #%s: %s", userName, channelName, msg)

					cmdMatches := cmdRegex.FindStringSubmatch(msg)
					if cmdMatches != nil {
						botId, cmd := strings.ToLower(cmdMatches[1]), strings.ToLower(cmdMatches[2])
						// Split the third match into a slice on whitespace for arguments
						arguments := strings.Fields(cmdMatches[3])

						// Ignore the command if it's not directed at this bot
						if !contains(aliases, botId) {
							break
						}

						parseCommand(db, channelName, userName, cmd, arguments)
						if err != nil {
							return err
						}
					}
				default:
					// do nothing
				}
			}
		}
	}
}

// Makes the bot send a message to the chat channel.
func (db *DwarfBot) Say(channelName, msg string) error {
	if msg == "" {
		return errors.New("msg was empty")
	}

	_, err := db.conn.Write([]byte(fmt.Sprintf("PRIVMSG #%s :%s\r\n", channelName, msg)))
	log.Printf("%s #%s: %s", db.Name, channelName, msg)

	if err != nil {
		return err
	}

	return nil
}
