package dwarfbot

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"net/textproto"
	"regexp"
	"strings"
	"time"
)

// Bot aliases
var aliases = []string{"hammerdwarfbot", "dwarfbot"}

// Regex for parsing PRIVMSG strings.
//
// First matched group is the user's name and the second matched group is the content of the
// user's message.
var msgRegex *regexp.Regexp = regexp.MustCompile(`^:(\w+)!\w+@\w+\.tmi\.twitch\.tv (PRIVMSG) #\w+(?: :(.*))?$`)

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
		db.JoinChannel()
		defer db.PartChannel()

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

// JoinChannel joins a specific IRC Channel
func (db *DwarfBot) JoinChannel() {
	db.conn.Write([]byte("PASS oauth:" + db.Credentials.Token + "\r\n"))
	db.conn.Write([]byte("NICK " + db.Name + "\r\n"))

	// Channel login must be lowercase (https://dev.twitch.tv/docs/irc/guide#syntax-notes)
	db.conn.Write([]byte("JOIN #" + strings.ToLower(db.Channel) + "\r\n"))

	log.Printf("Joined channel #%s as @%s", db.Channel, db.Name)
}

func (db *DwarfBot) PartChannel() {
	db.conn.Write([]byte("PART #" + strings.ToLower(db.Channel) + "\r\n"))
	log.Printf("Parted from channel #%s", db.Channel)
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

				if db.Verbose {
					log.Printf("User: %s, Message Type: %s", userName, msgType)
				}

				switch msgType {
				case "PRIVMSG":
					msg := matches[3]
					log.Printf("%s: %s", userName, msg)

					cmdMatches := cmdRegex.FindStringSubmatch(msg)
					if cmdMatches != nil {
						botId, cmd := strings.ToLower(cmdMatches[1]), strings.ToLower(cmdMatches[2])
						// Split the third match into a slice on whitespace for arguments
						arguments := strings.Fields(cmdMatches[3])

						// Ignore the command if it's not directed at this bot
						if !contains(aliases, botId) {
							break
						}

						parseCommand(db, userName, cmd, arguments)
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
func (db *DwarfBot) Say(msg string) error {
	if msg == "" {
		return errors.New("msg was empty")
	}

	_, err := db.conn.Write([]byte(fmt.Sprintf("PRIVMSG #%s :%s\r\n", db.Channel, msg)))
	log.Printf("%s: %s", db.Name, msg)

	if err != nil {
		return err
	}

	return nil
}

func parseAdminCommand(db *DwarfBot, cmd string, arguments []string) error {
	var err error

	switch cmd {
	case "shutdown":
		db.Say("Yah, boss! Shuttin' 'er doon!")
		db.Disconnect()
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
