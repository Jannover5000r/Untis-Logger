package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

// Account structure to save in JSON
type Account struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// State management for conversation steps
type UserState struct {
	Step     string // "awaiting_username", "awaiting_password"
	Username string // Temporary storage for username until password is received
}

var (
	userStates   = make(map[string]*UserState) // userID -> state
	stateMutex   sync.Mutex                    // protect userStates
	accountsFile = "accounts.json"
)

// Save account info to JSON file (appends or updates)
func saveAccount(userID, username, password string) error {
	var accounts []Account

	// Load existing accounts if file exists
	if data, err := os.ReadFile(accountsFile); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &accounts)
	}

	// Append new account
	accounts = append(accounts, Account{
		UserID:   userID,
		Username: username,
		Password: password,
	})

	// Save back to file
	newData, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(accountsFile, newData, 0644)
}

func Start() {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		fmt.Println("DISCORD_BOT_TOKEN environment variable is not set.")
		return
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL+C to exit.")
	// Wait for CTRL+C or other term signal to exit.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Handle "!addaccount" only in guilds (not in DMs)
	if m.GuildID != "" && m.Content == "!addaccount" {
		// Delete the command for privacy
		_ = s.ChannelMessageDelete(m.ChannelID, m.ID)
		// Create DM channel
		channel, err := s.UserChannelCreate(m.Author.ID)
		if err != nil {
			fmt.Println("Error creating DM channel:", err)
			return
		}
		s.ChannelMessageSend(channel.ID, "Let's add your account. Please provide your username:")

		stateMutex.Lock()
		userStates[m.Author.ID] = &UserState{Step: "awaiting_username"}
		stateMutex.Unlock()
		return
	}

	// Handle DMs for the two-step process
	if m.GuildID == "" {
		stateMutex.Lock()
		state, ok := userStates[m.Author.ID]
		stateMutex.Unlock()
		if !ok {
			return // Not in the process
		}

		switch state.Step {
		case "awaiting_username":
			// Store the username and prompt for password
			stateMutex.Lock()
			state.Username = m.Content
			state.Step = "awaiting_password"
			userStates[m.Author.ID] = state
			stateMutex.Unlock()
			s.ChannelMessageSend(m.ChannelID, "Now, please provide your password:")
		case "awaiting_password":
			username := state.Username
			password := m.Content
			// Save to JSON
			if err := saveAccount(m.Author.ID, username, password); err != nil {
				s.ChannelMessageSend(m.ChannelID, "There was an error saving your account. Please try again later.")
				fmt.Println("Error saving account:", err)
			} else {
				s.ChannelMessageSend(m.ChannelID, "Your account has been saved!")
			}
			// Cleanup state
			stateMutex.Lock()
			delete(userStates, m.Author.ID)
			stateMutex.Unlock()
		}
	}
}
