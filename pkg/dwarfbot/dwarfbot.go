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

type DwarfBot struct {
	// Channel to join, must be lowercase
	Channels []string

	// Reference to the bot's connection to the server
	conn net.Conn

	// OAuth credential for authentication
	Credentials *OAuthCreds

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

	// exitFunc is called by Die() to exit the process.
	// Defaults to os.Exit if nil.
	exitFunc func(int)

	// Metrics records platform-level metrics. Nil means no metrics.
	Metrics PlatformMetrics
}

func (db *DwarfBot) Start() error {
	defer db.Disconnect()

	log.Println("dwarfbot is starting...")

	for {
		if db.Verbose {
			log.Println("dwarfbot is waiting for a command...")
		}

		if err := db.Connect(); err != nil {
			return fmt.Errorf("twitch connection failed: %w", err)
		}
		db.Authenticate()

		// Join the bot's channel
		db.JoinChannel(db.Name)

		// Join secondary channels
		for _, channel := range db.Channels {
			db.JoinChannel(channel)
		}

		err := db.HandleChat()
		if err != nil {
			log.Println(err)
			time.Sleep(time.Second)
			log.Println("restarting bot...")
		}
	}
}

func (db *DwarfBot) Connect() error {
	if db.Server == "" || db.Port == "" {
		return fmt.Errorf("IRC server and port must be specified")
	}

	maxRetries := 10
	for attempt := range maxRetries {
		if db.Metrics != nil {
			db.Metrics.RecordConnectionAttempt("twitch", "attempting")
		}
		var err error
		db.conn, err = net.Dial("tcp", db.Server+":"+db.Port)
		if err == nil {
			db.startTime = time.Now()
			log.Printf("Connected to %s", db.Server)
			if db.Metrics != nil {
				db.Metrics.RecordConnectionAttempt("twitch", "success")
				db.Metrics.RecordConnected("twitch")
			}
			return nil
		}
		if db.Metrics != nil {
			db.Metrics.RecordConnectionAttempt("twitch", "failure")
		}
		backoff := time.Duration(attempt+1) * time.Second
		log.Printf("Failed connecting to %s, retrying in %v...\n", db.Server, backoff)
		time.Sleep(backoff)
	}

	return fmt.Errorf("failed to connect to %s after %d attempts", db.Server, maxRetries)
}

func (db *DwarfBot) Disconnect() {
	if db.conn == nil {
		return
	}
	if err := db.conn.Close(); err != nil {
		log.Printf("Error closing connection: %v", err)
	}
	duration := time.Since(db.startTime)
	log.Printf("Connection closed; elapsed time %g", duration.Seconds())
	if db.Metrics != nil {
		db.Metrics.RecordDisconnected("twitch", "closed")
		db.Metrics.RecordConnectionDuration("twitch", duration)
	}
}

func (db *DwarfBot) Authenticate() {
	if _, err := db.conn.Write([]byte("PASS oauth:" + db.Credentials.Token + "\r\n")); err != nil {
		log.Printf("Failed to send PASS during authentication: %v", err)
	}
	if _, err := db.conn.Write([]byte("NICK " + db.Name + "\r\n")); err != nil {
		log.Printf("Failed to send NICK during authentication: %v", err)
	}
}

// JoinChannel joins a specific IRC Channel
func (db *DwarfBot) JoinChannel(channel string) {
	if channel == "" {
		return
	}

	// Channel login must be lowercase (https://dev.twitch.tv/docs/irc/guide#syntax-notes)
	if _, err := db.conn.Write([]byte("JOIN #" + strings.ToLower(channel) + "\r\n")); err != nil {
		log.Printf("Failed to join channel #%s: %v", channel, err)
		return
	}

	log.Printf("Joined channel #%s as @%s", channel, db.Name)
}

func (db *DwarfBot) PartChannel(channel string) {
	if channel == "" {
		return
	}

	if _, err := db.conn.Write([]byte("PART #" + strings.ToLower(channel) + "\r\n")); err != nil {
		log.Printf("Failed to part from channel #%s: %v", channel, err)
		return
	}
	log.Printf("Parted from channel #%s", channel)
}

// Handle shutdown for good commands
func (db *DwarfBot) Die(exitCode int) {
	if db.exitFunc != nil {
		db.exitFunc(exitCode)
		return
	}
	os.Exit(exitCode)
}

// HandleChat is the main loop, listenting to incoming chat and responding
func (db *DwarfBot) HandleChat() error {
	tp := textproto.NewReader(bufio.NewReader(db.conn))

	for {
		line, err := tp.ReadLine()
		if err != nil {
			db.Disconnect()

			return fmt.Errorf("failed to read line from channel, disconnecting: %w", err)
		}

		if db.Verbose {
			log.Println(line)
		}

		if line == "PING :tmi.twitch.tv" {

			// Must reply to PING messages with PONG message to stay connected
			pong := "PONG :tmi.twitch.tv\r\n"
			if _, err := db.conn.Write([]byte(pong)); err != nil {
				db.Disconnect()
				return fmt.Errorf("failed to write PONG to server, disconnecting: %w", err)
			}
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
					if db.Metrics != nil {
						db.Metrics.RecordMessageReceived("twitch")
					}

					cmdMatches := cmdRegex.FindStringSubmatch(msg)
					if cmdMatches != nil {
						botId, cmd := strings.ToLower(cmdMatches[1]), strings.ToLower(cmdMatches[2])
						// Split the third match into a slice on whitespace for arguments
						arguments := strings.Fields(cmdMatches[3])

						// Ignore the command if it's not directed at this bot
						if !contains(aliases, botId) {
							break
						}

						if cmdErr := parseCommand(db, channelName, userName, cmd, arguments, db.Metrics); cmdErr != nil {
							return cmdErr
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

	_, err := fmt.Fprintf(db.conn, "PRIVMSG #%s :%s\r\n", channelName, msg)
	if err != nil {
		return err
	}
	log.Printf("%s #%s: %s", db.Name, channelName, msg)

	return nil
}

// ChatPlatform interface implementation for DwarfBot (Twitch).

func (db *DwarfBot) SendMessage(channel, msg string) error {
	err := db.Say(channel, msg)
	if db.Metrics != nil {
		if err != nil {
			db.Metrics.RecordMessageSent("twitch", "failure")
		} else {
			db.Metrics.RecordMessageSent("twitch", "success")
		}
	}
	return err
}

func (db *DwarfBot) IsAdmin(channel, user string) bool {
	return user == channel
}

func (db *DwarfBot) BotName() string {
	return db.Name
}

func (db *DwarfBot) BotChannels() []string {
	return db.Channels
}

func (db *DwarfBot) Shutdown(exitCode int) {
	db.Disconnect()
	db.Die(exitCode)
}
