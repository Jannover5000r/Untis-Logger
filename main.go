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
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

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

var location *time.Location

// lessonKey creates a stable key for a lesson (date + time + subject)
func lessonKey(e NamedTimetableEntry) string {
	subject := "unknown"
	if len(e.Su) > 0 {
		subject = e.Su[0]
	}
	// Include date for multi-day accuracy (e.g., Monday vs Tuesday)
	return e.Date + "|" + e.StartTime + "|" + subject
}

// getSubject returns the first subject or "Free period"
func getSubject(e NamedTimetableEntry) string {
	if len(e.Su) == 0 {
		return "Free period"
	}
	return e.Su[0]
}

// formatRoom returns a readable room string
func formatRoom(rooms []string) string {
	if len(rooms) == 0 {
		return "no room"
	}
	return strings.Join(rooms, ", ")
}

// formatStatus returns a human-readable status
func formatStatus(code string) string {
	if code == "" {
		return "regular"
	}
	return code
}

// slicesEqual checks if two string slices are equal (order-sensitive)
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
	locEnv := os.Getenv("LOCATION_ENV")

	var locErr error

	location, locErr = time.LoadLocation(locEnv)

	if locErr != nil {
		log.Fatalf("Failed to load timezone: %v", locErr)
	}
	log.Printf("Timezone set to: %s", location)
}

func getTime() time.Time {
	return time.Now().In(location)
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
	scheduled := []string{"07:45", "08:35", "09:35", "10:25", "11:25", "12:15", "13:40", "14:25", "15:20", "16:00"}
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
			// Parse current and previous timetable data
			var currentEntries, prevEntries []NamedTimetableEntry

			if err := json.Unmarshal(data, &currentEntries); err != nil {
				log.Printf("Failed to parse new timetable for user %s: %v", acc.Username, err)
				prevData[acc.UserID] = data // still update to avoid repeated errors
				continue
			}

			if prev, ok := prevData[acc.UserID]; ok {
				if err := json.Unmarshal(prev, &prevEntries); err != nil {
					log.Printf("Failed to parse previous timetable for user %s: %v", acc.Username, err)
					prevData[acc.UserID] = data
					continue
				}

				// Build maps keyed by a stable lesson identifier
				currMap := make(map[string]NamedTimetableEntry)
				prevMap := make(map[string]NamedTimetableEntry)
				// Also build timeslot-based maps for better exchange detection
				currTimeslotMap := make(map[string]NamedTimetableEntry)
				prevTimeslotMap := make(map[string]NamedTimetableEntry)

				// Get current date to filter out new day changes
				today := time.Now().Format("02-01-2006")
				// Get current time to filter any changes to Timetable before 3am so the new day changes stop
				now := time.Now()

				for _, e := range currentEntries {
					key := lessonKey(e)
					currMap[key] = e
				}
				for _, e := range prevEntries {
					key := lessonKey(e)
					prevMap[key] = e
				}

				var changes []string

				// Check for modified or new lessons
				for key, curr := range currMap {
					// Skip lessons that are not for today to avoid new day spam
					if curr.Date != today {
						continue
					}
					if now.Hour() < 3 {
						continue
					}

					if prev, exists := prevMap[key]; exists {
						// Existing lesson — check for changes
						subject := getSubject(curr)

						// Room change?
						if !slicesEqual(prev.Ro, curr.Ro) {
							oldRoom := formatRoom(prev.Ro)
							newRoom := formatRoom(curr.Ro)
							changes = append(changes, fmt.Sprintf("Room changed for %s: %s → %s", subject, oldRoom, newRoom))
						}

						for _, e := range currentEntries {
							key := lessonKey(e)
							currMap[key] = e
							// Create timeslot key (date + startTime + endTime)
							timeslotKey := e.Date + "|" + e.StartTime + "|" + e.EndTime
							currTimeslotMap[timeslotKey] = e
						}
						for _, e := range prevEntries {
							key := lessonKey(e)
							prevMap[key] = e
							// Create timeslot key (date + startTime + endTime)
							timeslotKey := e.Date + "|" + e.StartTime + "|" + e.EndTime
							prevTimeslotMap[timeslotKey] = e
						}

						// Code/status change?
						if prev.Code != curr.Code {
							oldStatus := formatStatus(prev.Code)
							newStatus := formatStatus(curr.Code)
							changes = append(changes, fmt.Sprintf("Status changed for %s: %s → %s", subject, oldStatus, newStatus))
						}
					} else {
						// New lesson - but check if it's actually an exchange in the same timeslot
						timeslotKey := curr.Date + "|" + curr.StartTime + "|" + curr.EndTime
						if prevInTimeslot, exists := prevTimeslotMap[timeslotKey]; exists {
							// This is an exchange: old lesson was removed, new one added in same timeslot
							oldSubject := getSubject(prevInTimeslot)
							newSubject := getSubject(curr)
							room := formatRoom(curr.Ro)
							changes = append(changes, fmt.Sprintf("Lesson exchanged: %s → %s in %s", oldSubject, newSubject, room))
						} else {
							// Truly new lesson
							subject := getSubject(curr)
							room := formatRoom(curr.Ro)
							changes = append(changes, fmt.Sprintf("New lesson: %s in %s", subject, room))
						}
					}
				}

				// Check for removed lessons
				for key, prev := range prevMap {
					// Skip lessons that were not for today
					if prev.Date != today {
						continue
					}

					if _, exists := currMap[key]; !exists {
						subject := getSubject(prev)
						// Check if this removal is part of an exchange (already handled above)
						timeslotKey := prev.Date + "|" + prev.StartTime + "|" + prev.EndTime
						if _, newExists := currTimeslotMap[timeslotKey]; !newExists {
							// Only report as removed if no new lesson took its place
							changes = append(changes, fmt.Sprintf("Lesson removed: %s", subject))
						}
					}
				}

				// Send notification if changes detected
				if len(changes) > 0 {
					log.Printf("Timetable changed for user %s: %d changes", acc.Username, len(changes))
					message := "Your timetable has changed!\n\n" + strings.Join(changes, "\n")
					BotStart.SendDM(acc.UserID, message)

					// Optional: send to Discord webhook if configured and user is "default"
					if acc.UserID == "default" && discordWebhookURL != "" {
						sendUpdateDiscordWebhookWithDetails(changes)
					}
				}
			} else {
				// First run — no previous data, so don't notify
				log.Printf("First timetable fetch for user %s", acc.Username)
			}

			// Always update prevData
			prevData[acc.UserID] = data
		}
	}

	// Initial update
	updateAllUsers()

	// Ticker for periodic updates
	hourTicker := time.NewTicker(30 * time.Minute)
	go func() {
		for range hourTicker.C {
			updateAllUsers()
		}
	}()

	// Ticker for scheduled lesson notifications
	startMinuteTicker(func() {
		now := getTime()
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
	now := getTime()
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

	now := getTime().Format("15:04")

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

	// Calculate 30 minutes from now
	thirtyMinutesLater := now.Add(30 * time.Minute)

	var times []time.Time
	timeToStr := make(map[time.Time]string)
	for t := range roomByStartTime {
		parsed, err := time.Parse(layout, t)
		if err != nil {
			continue
		}
		// Only include times within the next 30 minutes
		if parsed.After(now) && parsed.Before(thirtyMinutesLater) {
			times = append(times, parsed)
			timeToStr[parsed] = t
		}
	}

	if len(times) == 0 {
		return "", "", false
	}

	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	// Return the first (earliest) room within the 30-minute window
	return timeToStr[times[0]], roomByStartTime[timeToStr[times[0]]], true
}

func NextSubjectForTime(subjectByStartTime map[string]string, current string) (string, bool) {
	layout := "15:04"
	now, err := time.Parse(layout, current)
	if err != nil {
		return "", false
	}

	// Calculate 30 minutes from now
	thirtyMinutesLater := now.Add(30 * time.Minute)

	var times []time.Time
	timeToStr := make(map[time.Time]string)
	for t := range subjectByStartTime {
		parsed, err := time.Parse(layout, t)
		if err != nil {
			continue
		}
		// Only include times within the next 30 minutes
		if parsed.After(now) && parsed.Before(thirtyMinutesLater) {
			times = append(times, parsed)
			timeToStr[parsed] = t
		}
	}

	if len(times) == 0 {
		return "", false
	}

	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	// Return the first (earliest) subject within the 30-minute window
	return subjectByStartTime[timeToStr[times[0]]], true
}

func NextCodeForTime(codeByStartTime map[string]string, current string) (string, bool) {
	layout := "15:04"
	now, err := time.Parse(layout, current)
	if err != nil {
		return "", false
	}

	// Calculate 30 minutes from now
	thirtyMinutesLater := now.Add(30 * time.Minute)

	var times []time.Time
	timeToStr := make(map[time.Time]string)
	for t := range codeByStartTime {
		parsed, err := time.Parse(layout, t)
		if err != nil {
			continue
		}
		// Only include times within the next 30 minutes
		if parsed.After(now) && parsed.Before(thirtyMinutesLater) {
			times = append(times, parsed)
			timeToStr[parsed] = t
		}
	}

	if len(times) == 0 {
		return "", false
	}

	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	// Return the first (earliest) code within the 30-minute window
	return codeByStartTime[timeToStr[times[0]]], true
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
			codeByStartTime[entry.StartTime] = entry.Code
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

func sendUpdateDiscordWebhookWithDetails(changes []string) {
	if BotStart.WebHook {
		log.Println("Sending detailed Discord webhook notification...")

		message := "**Timetable Update**\n\n" + strings.Join(changes, "\n")

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
			log.Println("Detailed Discord webhook sent successfully")
		} else {
			body, _ := ioutil.ReadAll(resp.Body)
			log.Printf("Discord webhook failed with status %d: %s", resp.StatusCode, string(body))
		}
	}
}
