package dwarfbot

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/textproto"
	"time"
)

type OAuthCreds struct {
	// Client ID
	Name string `json:"name,omitempty"`

	// Client Secret
	ClientSecret string `json:"client_secret,omitempty"`
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
	db.conn.Write([]byte("PASS oauth: " + db.Credentials.ClientSecret + "\r\n"))
	db.conn.Write([]byte("NICK " + db.Name + "\r\n"))
	db.conn.Write([]byte("JOIN #" + db.Channel + "\r\n"))

	log.Printf("Joined channel #%s as @%s", db.Channel, db.Name)
}

// HandleChat is the main loop, listenting to incoming chat and responding
func (db *DwarfBot) HandleChat() error {
	tp := textproto.NewReader(bufio.NewReader(db.conn))

	for {
		line, err := tp.ReadLine()
		if err != nil {
			db.Disconnect()

			return err
		}

		if db.Verbose {
			fmt.Println(line)
		}
	}
}
