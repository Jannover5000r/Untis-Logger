package Untis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type TimetableResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Result  []timetable `json:"result"`
}
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
type timetable struct {
	ID           int     `json:"id"`
	Date         int     `json:"date"`
	StartTime    int     `json:"startTime"`
	EndTime      int     `json:"endTime"`
	Code         string  `json:"code,omitempty"`
	Statflags    string  `json:"statflags,omitempty"`
	Kl           []IDObj `json:"kl"`
	Su           []IDObj `json:"su"`
	Ro           []IDObj `json:"ro"`
	ActivityType string  `json:"activityType"`
}
type IDObj struct {
	ID int `json:"id"`
}
type TimetableEntry struct {
	ID           int     `json:"id"`
	Date         int     `json:"date"`
	StartTime    int     `json:"startTime"`
	EndTime      int     `json:"endTime"`
	Code         string  `json:"code,omitempty"`
	Statflags    string  `json:"statflags,omitempty"`
	Kl           []IDObj `json:"kl"`
	Su           []IDObj `json:"su"`
	Ro           []IDObj `json:"ro"`
	ActivityType string  `json:"activityType"`
}
type NamedObj struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type getTimetable struct {
	Id      string `json:"id"`
	Method  string `json:"method"`
	Params  params `json:"params"`
	Jsonrpc string `json:"jsonrpc"`
}
type params struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Id        int    `json:"id"`
	Type      int    `json:"type"`
}

func ReadLoginResultFromFile(path string) (Loginresult, error) {
	var result Loginresult
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}

func Timetable(cookies []*http.Cookie) {
	loginResult, err := ReadLoginResultFromFile("login.json")
	if err != nil {
		log.Fatal(err)
	}
	today := time.Now().Format("20060102")
	g := getTimetable{"2023-05-06 15:44:22.215292", "getTimetable", params{today, today, loginResult.PersonID, loginResult.PersonType}, "2.0"}
	TimetablesJson, err := json.Marshal(g)
	if err != nil {
		log.Fatalf("Error marshaling login data: %v", err)
		return
	}
	timetable := bytes.NewReader(TimetablesJson)

	prompt, err := http.NewRequest("GET", Url, timetable)
	if err != nil {
		log.Fatalf("Error creatingrequest: %v", err)
		return
	}
	//log.Println("prompt without extra header or cookie ", prompt)
	//log.Println("Cookie: ", cookies)

	prompt.Header.Set("Content-Type", "application/json")
	prompt.Header.Set("User-Agent", "Webuntis Test")

	for _, cookie := range cookies {
		//if cookie.Name == "JSESSIONID" {
		prompt.AddCookie(cookie)
		//log.Printf("Added JSESSIONID cookie: %s=%s", cookie.Name, cookie.Value)
		//}
	}
	//log.Println("Request JSON:", string(TeachersJson))
	out, err := http.DefaultClient.Do(prompt)
	if err != nil {
		log.Printf("Error during request: %v", err)
		return
	}
	defer out.Body.Close()
	//log.Println(out.Status)
	response, err := io.ReadAll(out.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return
	}
	//responseString := string(response)
	//log.Println("Repsonse ", responseString)
	var Response TimetableResponse
	err = json.Unmarshal(response, &Response)
	if err != nil {
		log.Fatalf("Error unmarshaling response: %v", err)
	}
	data, err := json.MarshalIndent(Response.Result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile("timetable.json", data, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Updated Timetable")
	setTimetable()
}

func LoadIDMap(path string) (map[int]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var objs []NamedObj
	if err := json.Unmarshal(data, &objs); err != nil {
		return nil, err
	}
	m := make(map[int]string)
	for _, obj := range objs {
		m[obj.ID] = obj.Name
	}
	return m, nil
}
func LoadTimetable(path string) ([]TimetableEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []TimetableEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
func formatTime(t int) string {
	h := t / 100
	m := t % 100
	return fmt.Sprintf("%02d:%02d", h, m)
}
func formatDate(date int) string {
	s := fmt.Sprintf("%08d", date) // ensures leading zeros
	year := s[:4]
	month := s[4:6]
	day := s[6:8]
	return fmt.Sprintf("%s-%s-%s", day, month, year)
}
func setTimetable() {
	subjects, _ := LoadIDMap("subjects.json")
	rooms, _ := LoadIDMap("rooms.json")
	classes, _ := LoadIDMap("classes.json")
	timetable, _ := LoadTimetable("timetable.json")

	var namedTimetable []NamedTimetableEntry

	for _, lesson := range timetable {
		var klNames, suNames, roNames []string
		for _, kl := range lesson.Kl {
			klNames = append(klNames, classes[kl.ID])
		}
		for _, su := range lesson.Su {
			suNames = append(suNames, subjects[su.ID])
		}
		for _, ro := range lesson.Ro {
			roNames = append(roNames, rooms[ro.ID])
		}
		namedTimetable = append(namedTimetable, NamedTimetableEntry{
			ID:           lesson.ID,
			Date:         formatDate(lesson.Date),
			StartTime:    formatTime(lesson.StartTime),
			EndTime:      formatTime(lesson.EndTime),
			Code:         lesson.Code,
			Statflags:    lesson.Statflags,
			Kl:           klNames,
			Su:           suNames,
			Ro:           roNames,
			ActivityType: lesson.ActivityType,
		})
	}

	data, err := json.MarshalIndent(namedTimetable, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile("timetableFilled.json", data, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Filled Timetable")
}
