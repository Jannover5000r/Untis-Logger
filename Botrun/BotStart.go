package bot

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"untislogger/Untis"

	"github.com/joho/godotenv"

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
	accountsFile = "accounts/accounts.json"
)

var encryptionKey []byte

var WebHook = true

func init() {
	godotenv.Load(".env")
	keyStr := os.Getenv("ENC_KEY")
	if keyStr == "" {
		panic("ENC_KEY environment variable not set")
	}
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		panic("ENC_KEY is not valid base64")
	}
	if len(key) != 32 {
		panic("ENC_KEY must be exactly 32 bytes after base64 decoding")
	}
	encryptionKey = key
}

func encrypt(text string) (string, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(text), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(encrypted string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Save account info to JSON file (appends or updates)
func saveAccount(userID, username, password string) error {
	var accounts []Account

	// Ensure the accounts directory exists
	accountsDir := filepath.Dir(accountsFile)
	if err := os.MkdirAll(accountsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create accounts directory: %v", err)
	}

	// Load existing accounts if file exists
	if data, err := os.ReadFile(accountsFile); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &accounts)
	}

	// Remove any account with the same userID
	newAccounts := accounts[:0]
	for _, acc := range accounts {
		if acc.UserID != userID {
			newAccounts = append(newAccounts, acc)
		}
	}
	accounts = newAccounts

	// Encrypt the password before saving
	encPwd, err := encrypt(password)
	if err != nil {
		return err
	}
	accounts = append(accounts, Account{
		UserID:   userID,
		Username: username,
		Password: encPwd,
	})

	// Save back to file
	newData, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(accountsFile, newData, 0o644); err != nil {
		return err
	}

	// Ensure timetable files exist for this user
	timetableFile := getTimetableFile(userID)
	timetableFilledFile := getTimetableFilledFile(userID)
	if _, err := os.Stat(timetableFile); os.IsNotExist(err) {
		os.WriteFile(timetableFile, []byte("[]"), 0o644)
	}
	if _, err := os.Stat(timetableFilledFile); os.IsNotExist(err) {
		os.WriteFile(timetableFilledFile, []byte("[]"), 0o644)
	}

	return nil
}

// Helper functions for per-user timetable files
func getTimetableFile(userID string) string {
	return fmt.Sprintf("timetable_%s.json", userID)
}

func getTimetableFilledFile(userID string) string {
	return fmt.Sprintf("timetableFilled_%s.json", userID)
}

// Send a Discord message mentioning the user
func sendLessonNotification(s *discordgo.Session, userID, username, message string) {
	// Send a DM to the user
	channel, err := s.UserChannelCreate(userID)
	if err != nil {
		fmt.Println("Error creating DM channel:", err)
		return
	}
	s.ChannelMessageSend(channel.ID, fmt.Sprintf("**%s**: %s", username, message))
}

// Check for timetable changes for a user and notify if changed
func checkTimetableChangesForUser(user Account, decPwd string, s *discordgo.Session) {
	timetableFilledFile := getTimetableFilledFile(user.UserID)

	// Load previous timetable
	prevData, _ := os.ReadFile(timetableFilledFile)

	// Fetch new timetable for this user
	cookies, err := Untis.Auth(user.Username, decPwd, user.UserID)
	if err != nil {
		fmt.Printf("Error authenticating user %s: %v\n", user.Username, err)
		return
	}
	Untis.Timetable(cookies, user.UserID)

	// Load the newly fetched timetable data
	newData, err := os.ReadFile(timetableFilledFile)
	if err != nil {
		fmt.Printf("Error reading new timetable for user %s: %v\n", user.Username, err)
		return
	}

	// Compare and notify if changed
	if !bytes.Equal(newData, prevData) {
		// The main.go now handles detailed change notifications
		// This function is kept for compatibility but no longer sends generic messages
	}
}

// Load all accounts from accounts.json
func loadAllAccounts() []Account {
	var accounts []Account
	if data, err := os.ReadFile(accountsFile); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &accounts)
	}
	return accounts
}

// Scheduled check for all users
func checkAllUsersTimetables(s *discordgo.Session) {
	accounts := loadAllAccounts()
	for _, user := range accounts {
		decPwd, _ := decrypt(user.Password)
		checkTimetableChangesForUser(user, decPwd, s)
	}
}

var DiscordSession *discordgo.Session

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
	DiscordSession = dg // Save session for use elsewhere

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

// DecryptedAccount holds user information with a decrypted password.
type DecryptedAccount struct {
	UserID            string
	Username          string
	DecryptedPassword string
}

// LoadAndDecryptAccounts loads all accounts from the JSON file and decrypts their passwords.
func LoadAndDecryptAccounts() ([]DecryptedAccount, error) {
	accounts := loadAllAccounts()
	decryptedAccounts := make([]DecryptedAccount, 0, len(accounts))

	for _, acc := range accounts {
		decPwd, err := decrypt(acc.Password)
		if err != nil {
			// Log the error but continue with other accounts
			fmt.Printf("Error decrypting password for user %s: %v\n", acc.UserID, err)
			continue
		}
		decryptedAccounts = append(decryptedAccounts, DecryptedAccount{
			UserID:            acc.UserID,
			Username:          acc.Username,
			DecryptedPassword: decPwd,
		})
	}

	return decryptedAccounts, nil
}

// Expose this for main.go to trigger notifications
func NotifyAllUsers() {
	if DiscordSession != nil {
		checkAllUsersTimetables(DiscordSession)
	}
}

// SendDM sends a direct message to a user.
func SendDM(userID, message string) {
	if DiscordSession == nil {
		fmt.Println("Discord session not initialized, cannot send DM.")
		return
	}
	// Avoid sending messages to the "default" user placeholder
	if userID == "default" {
		return
	}
	channel, err := DiscordSession.UserChannelCreate(userID)
	if err != nil {
		fmt.Printf("Error creating DM channel for %s: %v\n", userID, err)
		return
	}
	_, err = DiscordSession.ChannelMessageSend(channel.ID, message)
	if err != nil {
		fmt.Printf("Error sending DM to %s: %v\n", userID, err)
	}
}

// Delete account and associated files for a given userID
func deleteAccount(userID string) error {
	var accounts []Account

	// Load existing accounts
	data, err := os.ReadFile(accountsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No accounts file means nothing to delete
		}
		return err
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, &accounts); err != nil {
			return err
		}
	}

	// Filter out the account with matching userID
	newAccounts := accounts[:0]
	found := false
	for _, acc := range accounts {
		if acc.UserID != userID {
			newAccounts = append(newAccounts, acc)
		} else {
			found = true
		}
	}

	if !found {
		return errors.New("account not found")
	}

	// Save remaining accounts back to file
	newData, err := json.MarshalIndent(newAccounts, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(accountsFile, newData, 0o644); err != nil {
		return err
	}

	// Delete associated timetable files
	os.Remove(getTimetableFile(userID))
	os.Remove(getTimetableFilledFile(userID))

	return nil
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}
	if m.GuildID != "" && m.Content == "!WebHook" {
		_ = s.ChannelMessageDelete(m.ChannelID, m.ID)
		WebHook = !WebHook
		if WebHook {
			s.ChannelMessageSend(m.ChannelID, "Sending Webhook set to True")
		} else {
			s.ChannelMessageSend(m.ChannelID, "Sending Webhook set to False")
		}
		return
	}
	// Handle "!addaccount" only in guilds (not in DMs)
	if m.Content == "!addaccount" {
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

	if m.Content == "!update" {
		_ = s.ChannelMessageDelete(m.ChannelID, m.ID)
		checkAllUsersTimetables(DiscordSession)
		s.ChannelMessageSend(m.ChannelID, "Updated all Timetables manually")
	}

	// Handle "!deleteaccount" and "!removeaccount" commands
	if m.GuildID != "" && (m.Content == "!deleteaccount" || m.Content == "!removeaccount") {
		// Delete the command message for privacy
		_ = s.ChannelMessageDelete(m.ChannelID, m.ID)

		err := deleteAccount(m.Author.ID)
		if err != nil {
			// Create DM channel to notify of error
			channel, err := s.UserChannelCreate(m.Author.ID)
			if err == nil {
				s.ChannelMessageSend(channel.ID, "Failed to delete your account: "+err.Error())
			}
			return
		}

		// Notify success via DM
		channel, err := s.UserChannelCreate(m.Author.ID)
		if err == nil {
			s.ChannelMessageSend(channel.ID, "Your account has been successfully deleted.")
		}
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
