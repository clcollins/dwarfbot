package dwarfbot

import (
	"log"
	"net"
	"time"
)

type OAuthCreds struct {
	// Client ID
	ClientID string `json:"client_id,omitempty"`

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
	Credentails *OAuthCreds

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
	log.Printf("Connection closed; elapsed time %g", (time.Now().Sub(db.startTime).Seconds()))
}

// JoinChannel joins a specific IRC Channel
func (db *DwarfBot) JoinChannel() {

}

// HandleChat is the main loop, listenting to incoming chat and responding
func (db *DwarfBot) HandleChat() error {
	var err error

	log.Printf("Awaiting chat messages, zzz")
	for {
		time.Sleep(5 * time.Second)
		log.Printf("Still awaiting chat messages, zzz")
	}

	return err
}
