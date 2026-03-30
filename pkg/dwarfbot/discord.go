package dwarfbot

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// DiscordBot implements ChatPlatform for Discord.
type DiscordBot struct {
	// Discord bot token
	Token string

	// Discord channel IDs to listen in
	ChannelIDs []string

	// Discord role name required for admin commands
	AdminRole string

	// Name of the bot used in responses
	Name string

	// discordgo session
	session *discordgo.Session

	// exitFunc is called by Shutdown to exit the process.
	// Defaults to os.Exit if nil. Used for testing.
	exitFunc func(int)

	// adminRoleCache maps guild ID to resolved admin role ID to avoid
	// repeated REST lookups on every admin check.
	adminRoleCache map[string]string
	adminRoleMu    sync.RWMutex

	// Metrics records platform-level metrics. Nil means no metrics.
	Metrics PlatformMetrics
}

// Start creates the Discord session, registers handlers, and opens the connection.
func (d *DiscordBot) Start() error {
	var err error
	d.session, err = discordgo.New("Bot " + d.Token)
	if err != nil {
		if d.Metrics != nil {
			d.Metrics.RecordConnectionAttempt("discord", "failure")
		}
		return fmt.Errorf("error creating Discord session: %w", err)
	}

	d.session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	d.adminRoleCache = make(map[string]string)

	d.session.AddHandler(d.messageHandler)

	err = d.session.Open()
	if err != nil {
		if d.Metrics != nil {
			d.Metrics.RecordConnectionAttempt("discord", "failure")
		}
		return fmt.Errorf("error opening Discord connection: %w", err)
	}

	if d.Metrics != nil {
		d.Metrics.RecordConnectionAttempt("discord", "success")
		d.Metrics.RecordConnected("discord")
	}

	log.Printf("Discord bot connected as %s", d.Name)
	for _, ch := range d.ChannelIDs {
		log.Printf("Discord: listening in channel %s", ch)
	}

	return nil
}

// Stop cleanly closes the Discord session.
func (d *DiscordBot) Stop() error {
	if d.session != nil {
		if d.Metrics != nil {
			d.Metrics.RecordDisconnected("discord", "shutdown")
		}
		return d.session.Close()
	}
	return nil
}

// messageHandler processes incoming Discord messages.
func (d *DiscordBot) messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Only listen in configured channels
	if !contains(d.ChannelIDs, m.ChannelID) {
		return
	}

	// Parse command using the same regex as Twitch
	cmdMatches := cmdRegex.FindStringSubmatch(m.Content)
	if cmdMatches == nil {
		return
	}

	botId, cmd := strings.ToLower(cmdMatches[1]), strings.ToLower(cmdMatches[2])
	arguments := strings.Fields(cmdMatches[3])

	// Ignore if not directed at this bot
	if !contains(aliases, botId) {
		return
	}

	log.Printf("Discord: %s #%s: %s", m.Author.Username, m.ChannelID, m.Content)
	if d.Metrics != nil {
		d.Metrics.RecordMessageReceived("discord")
	}

	if err := parseCommand(d, m.ChannelID, m.Author.ID, cmd, arguments, parseCommandOpts{metrics: d.Metrics, platformName: "discord"}); err != nil {
		log.Printf("Discord: error handling command %q from user %s in channel %s: %v", cmd, m.Author.ID, m.ChannelID, err)
	}
}

// ChatPlatform interface implementation for DiscordBot.

func (d *DiscordBot) SendMessage(channel, msg string) error {
	if d.session == nil {
		return fmt.Errorf("discord session not initialized")
	}
	_, err := d.session.ChannelMessageSend(channel, msg)
	if err != nil {
		if d.Metrics != nil {
			d.Metrics.RecordMessageSent("discord", "failure")
		}
		return fmt.Errorf("error sending Discord message: %w", err)
	}
	if d.Metrics != nil {
		d.Metrics.RecordMessageSent("discord", "success")
	}
	log.Printf("%s #%s: %s", d.Name, channel, msg)
	return nil
}

func (d *DiscordBot) IsAdmin(channel, userID string) bool {
	if d.session == nil || d.AdminRole == "" {
		return false
	}

	// Get the channel to find the guild ID
	ch, err := d.session.Channel(channel)
	if err != nil {
		log.Printf("Discord: error getting channel info: %v", err)
		return false
	}

	// Resolve and cache the admin role ID per guild to avoid repeated
	// GuildRoles REST calls on every admin check.
	d.adminRoleMu.RLock()
	adminRoleID, cached := d.adminRoleCache[ch.GuildID]
	d.adminRoleMu.RUnlock()
	if !cached {
		roles, err := d.session.GuildRoles(ch.GuildID)
		if err != nil {
			log.Printf("Discord: error getting guild roles: %v", err)
			return false
		}
		for _, role := range roles {
			if strings.EqualFold(role.Name, d.AdminRole) {
				adminRoleID = role.ID
				break
			}
		}
		if adminRoleID != "" {
			d.adminRoleMu.Lock()
			d.adminRoleCache[ch.GuildID] = adminRoleID
			d.adminRoleMu.Unlock()
		}
	}

	if adminRoleID == "" {
		return false
	}

	// Get the member's roles in this guild
	member, err := d.session.GuildMember(ch.GuildID, userID)
	if err != nil {
		log.Printf("Discord: error getting member info: %v", err)
		return false
	}

	// Check if the member has the admin role
	for _, roleID := range member.Roles {
		if roleID == adminRoleID {
			return true
		}
	}

	return false
}

func (d *DiscordBot) BotName() string {
	return d.Name
}

func (d *DiscordBot) BotChannels() []string {
	return d.ChannelIDs
}

func (d *DiscordBot) Shutdown(exitCode int) {
	if err := d.Stop(); err != nil {
		log.Printf("Error stopping Discord session: %v", err)
	}
	if d.exitFunc != nil {
		d.exitFunc(exitCode)
		return
	}
	os.Exit(exitCode)
}
