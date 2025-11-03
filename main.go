package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	Untis "untislogger/Untis"

	BotStart "untislogger/Botrun"

	"github.com/joho/godotenv"
)

type NamedTimetableEntry struct {
	ID           int      `json:"id"`
	Date         string   `json:"date"`
	StartTime    string   `json:"startTime"`
	EndTime      string   `json:"endTime"`
	Code         string   `json:"code,omitempty"`
	Statflags    string   `json:"statflags,omitempty"`
	Kl           []string `json:"kl"`
	Su           []string `json:"su"`
	Ro           []string `json:"ro"`
	ActivityType string   `json:"activityType"`
}

// init and main//
func init() {
	godotenv.Load(".env")
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}
	discordWebhookURL = os.Getenv("DISCORD_WEBHOOK_URL")
	log.Println("Initializing application...")
	log.Printf("DISCORD_WEBHOOK_URL: %q", discordWebhookURL)

	// Check if Discord webhook is configured
	if discordWebhookURL != "" {
		log.Println("Discord webhook configured")
	} else {
		log.Println("No Discord webhook provided, Discord notifications will be disabled")
	}
}

func main() {
	godotenv.Load(".env")
	// Untis.Main() //starting API calls function| happens in schedule func
	// Run()
	// Starts logging the timetable for each new Lesson and logs changes
	go BotStart.Start()
	scheduleTimetableUpdate()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Program is running. Press Ctrl+C to stop.")
	// Start logging
	// Run()
	// Block until we receive a signal
	<-sigChan
	log.Println("Shutting down...")
}

func isScheduledTime(now time.Time) bool {
	scheduled := []string{"07:45", "08:35", "09:35", "10:25", "11:25", "12:15", "13:45", "14:25"}
	current := now.Format("15:04")
	for _, t := range scheduled {
		if t == current {
			return true
		}
	}
	return false
}

func scheduleTimetableUpdate() {
	prevData := make(map[string][]byte)

	// Function to update all users
	updateAllUsers := func() {
		// Load all registered users
		accounts, err := BotStart.LoadAndDecryptAccounts()
		if err != nil {
			log.Printf("Error loading accounts: %v", err)
		}

		// Add the default user from .env
		defaultUser := os.Getenv("UNTIS_USER")
		defaultPassword := os.Getenv("UNTIS_PASSWORD")
		if defaultUser != "" && defaultPassword != "" {
			accounts = append(accounts, BotStart.DecryptedAccount{
				UserID:            "default",
				Username:          defaultUser,
				DecryptedPassword: defaultPassword,
			})
		}

		// Process each user
		for _, acc := range accounts {
			// Fetch the latest timetable
			Untis.Main(acc.Username, acc.DecryptedPassword, acc.UserID)

			// Check for changes
			timetableFile := fmt.Sprintf("timetableFilled_%s.json", acc.UserID)
			if acc.UserID == "default" {
				timetableFile = "timetableFilled_default.json"
			}
			data, err := os.ReadFile(timetableFile)
			if err != nil {
				log.Printf("Error reading timetable for user %s: %v", acc.Username, err)
				continue
			}

			if prev, ok := prevData[acc.UserID]; ok && !bytes.Equal(data, prev) {
				log.Printf("Timetable has changed for user %s.", acc.Username)
				BotStart.SendDM(acc.UserID, "Your timetable has changed!")
			}
			prevData[acc.UserID] = data
		}
	}

	// Initial update
	updateAllUsers()

	// Ticker for periodic updates
	hourTicker := time.NewTicker(1 * time.Minute) // For testing, 1 minute. Change to 1 * time.Hour for production.
	go func() {
		for range hourTicker.C {
			updateAllUsers()
		}
	}()

	// Ticker for scheduled lesson notifications
	startMinuteTicker(func() {
		now := time.Now()
		now = now.Add(1 * time.Hour)
		if isScheduledTime(now) {
			log.Println("Scheduled time reached, running notifications...")
			accounts, _ := BotStart.LoadAndDecryptAccounts()
			// Also notify the default user
			accounts = append(accounts, BotStart.DecryptedAccount{UserID: "default"})
			for _, acc := range accounts {
				Run(acc.UserID)
			}
			log.Println("Finished running notifications.")
		}
	})
}

func startMinuteTicker(f func()) {
	now := time.Now()
	next := now.Truncate(time.Minute).Add(time.Minute)
	time.Sleep(time.Until(next))
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			f()
			<-ticker.C
		}
	}()
}

func Run(userID string) {
	timetableFile := "timetableFilled_default.json"
	if userID != "default" {
		timetableFile = fmt.Sprintf("timetableFilled_%s.json", userID)
	}
	log.Printf("Sending next lesson for user %s", userID)

	// Check if the timetable file exists
	if _, err := os.Stat(timetableFile); os.IsNotExist(err) {
		log.Printf("Timetable file %s not found for user %s.", timetableFile, userID)
		return
	}

	codeByStartTime, _ := MapTimeToCode(timetableFile)
	roomByStartTime, _ := MapTimeToRoom(timetableFile)
	subjectByStartTime, _ := MapTimeToSubject(timetableFile)

	nowOld := time.Now()
	nowOld = nowOld.Add(1 * time.Hour)
	now := nowOld.Format("15:04")
	nextTime, room, foundRoom := NextRoomForTime(roomByStartTime, now)
	if !foundRoom {
		return // No upcoming lessons
	}

	subject, foundSubject := NextSubjectForTime(subjectByStartTime, now)
	if !foundSubject {
		return // Should not happen if room was found
	}

	status, _ := NextCodeForTime(codeByStartTime, now)

	var message string
	if status == "cancelled" {
		message = fmt.Sprintf("Next lesson: **%s** at **%s**. is **%s**", subject, nextTime, status)
	} else if status != "" && status != "cancelled" {
		message = fmt.Sprintf("Next lesson: **%s** in room **%s** at **%s**. Status: **%s**", subject, room, nextTime, status)
	} else {
		message = fmt.Sprintf("Next lesson: **%s** in room **%s** at **%s**.", subject, room, nextTime)
	}

	if userID == "default" {
		if BotStart.WebHook {
			log.Println("WebHook status ", BotStart.WebHook)
			sendDiscordWebhook(subject, room, nextTime, status)
		}
	}
	BotStart.SendDM(userID, message)
}

/*
 */
func NextRoomForTime(roomByStartTime map[string]string, current string) (string, string, bool) {
	layout := "15:04"
	now, err := time.Parse(layout, current)
	if err != nil {
		return "", "", false
	}
	var times []time.Time
	timeToStr := make(map[time.Time]string)
	for t := range roomByStartTime {
		parsed, err := time.Parse(layout, t)
		if err != nil {
			continue
		}
		times = append(times, parsed)
		timeToStr[parsed] = t
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	for _, t := range times {
		if t.After(now) {
			return timeToStr[t], roomByStartTime[timeToStr[t]], true
		}
	}
	return "", "", false
}

func NextSubjectForTime(subjectByStartTime map[string]string, current string) (string, bool) {
	layout := "15:04"
	now, err := time.Parse(layout, current)
	if err != nil {
		return "", false
	}
	var times []time.Time
	timeToStr := make(map[time.Time]string)
	for t := range subjectByStartTime {
		parsed, err := time.Parse(layout, t)
		if err != nil {
			continue
		}
		times = append(times, parsed)
		timeToStr[parsed] = t
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	for _, t := range times {
		if t.After(now) {
			return subjectByStartTime[timeToStr[t]], true
		}
	}
	return "", false
}

func NextCodeForTime(codeByStartTime map[string]string, current string) (string, bool) {
	layout := "15:04"
	now, err := time.Parse(layout, current)
	if err != nil {
		return "", false
	}
	var times []time.Time
	timeToStr := make(map[time.Time]string)
	for t := range codeByStartTime {
		parsed, err := time.Parse(layout, t)
		if err != nil {
			continue
		}
		times = append(times, parsed)
		timeToStr[parsed] = t
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	for _, t := range times {
		if t.After(now) {
			return codeByStartTime[timeToStr[t]], true
		}
	}
	return "", false
}

func MapTimeToRoom(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var table []NamedTimetableEntry
	err = json.Unmarshal(data, &table)
	if err != nil {
		return nil, err
	}
	roomByStartTime := make(map[string]string)
	for _, entry := range table {
		if len(entry.Ro) > 0 {
			roomByStartTime[entry.StartTime] = entry.Ro[0]
		}
	}
	return roomByStartTime, nil
}

func MapTimeToCode(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var table []NamedTimetableEntry
	err = json.Unmarshal(data, &table)
	if err != nil {
		return nil, err
	}
	codeByStartTime := make(map[string]string)
	for _, entry := range table {
		if len(entry.Code) > 0 {
			if _, exists := codeByStartTime[entry.StartTime]; !exists {
				codeByStartTime[entry.StartTime] = entry.Code
			}
		}
	}
	return codeByStartTime, nil
}

func MapTimeToSubject(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var table []NamedTimetableEntry
	err = json.Unmarshal(data, &table)
	if err != nil {
		return nil, err
	}
	subjectByStartTime := make(map[string]string)
	for _, entry := range table {
		if len(entry.Su) > 0 {
			subjectByStartTime[entry.StartTime] = entry.Su[0]
		}
	}
	return subjectByStartTime, nil
}

// Discord webhook configuration

var discordWebhookURL string // Webhook URL from environment variable

// DiscordWebhookPayload represents the structure for Discord webhook messages
type DiscordWebhookPayload struct {
	Content string  `json:"content"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

type Embed struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Color       int     `json:"color"`
	Timestamp   string  `json:"timestamp"`
	Fields      []Field `json:"fields,omitempty"`
}

type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

func sendDiscordWebhook(subject string, room string, nextTime string, Status string) {
	log.Println("Sending Discord webhook notification...")
	// Create a rich embed message
	var message string
	if Status != "" {
		message = fmt.Sprintf(
			"Subject: %s\nRoom: %s\nStart-Time: %s\nStatus: %s",
			subject, room, nextTime, Status,
		)
	} else {
		message = fmt.Sprintf(
			"Subject: %s\nRoom: %s\nStart-Time: %s",
			subject, room, nextTime,
		)
	}
	payload := DiscordWebhookPayload{
		Content: message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling webhook payload: %v", err)
		return
	}

	resp, err := http.Post(discordWebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error sending Discord webhook: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Println("Discord webhook notification sent successfully")
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Discord webhook failed with status %d: %s", resp.StatusCode, string(body))
	}
}

func sendUpdateDiscordWebhook() {
	log.Println("Sending Discord webhook notification...")
	// Create a rich embed message
	message := "A lesson on your timetable has changed"
	/*embed := Embed{
		Title:       "Next Lesson ",
		Description: "The next lesson is starting soon:  ",
		Color:       3066993, // Green color
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []Field{
			{
				Name: "New Lesson",
				//	Value:  fmt.Sprintf("`%s`", ip),
				Value:  fmt.Sprintf("Room: %s", room),
				Inline: true,
			},
			{
				Name:   "Start-Time",
				Value:  fmt.Sprintf("Start time: %s", nextTime),
				Inline: true,
			},
			{
				Name:   "Status",
				Value:  fmt.Sprintf("Status: %s", Status),
				Inline: true,
			},
		},
	}
	*/
	payload := DiscordWebhookPayload{
		Content: message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling webhook payload: %v", err)
		return
	}

	resp, err := http.Post(discordWebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error sending Discord webhook: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Println("Discord webhook notification sent successfully")
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Discord webhook failed with status %d: %s", resp.StatusCode, string(body))
	}
}
