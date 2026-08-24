package Untis

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Params struct {
	Users    string `json:"user"`
	Password string `json:"password"`
	Client   string `json:"client"`
}

type Login struct {
	Id     string `json:"id"`
	Method string `json:"method"`
	Params Params `json:"params"`

	Jsonrpc string `json:"jsonrpc"`
}
type LoginResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Result  Loginresult `json:"result"`
}

var Url = os.Getenv("URL")

var location *time.Location

func init() {
	locEnv := os.Getenv("LOCATION_ENV")
	if locEnv == "" {
		locEnv = "UTC"
	}
	location, _ = time.LoadLocation(locEnv)
}

// var Password = os.Getenv("UNTIS_PASSWORD")
// var USERS = os.Getenv("UNTIS_USER")
func Main(user, password, userID string) {
	godotenv.Load("../.env")
	cookies, err := Auth(user, password, userID)
	if err != nil {
		log.Printf("Authentication failed for user %s: %v", user, err)
		return
	}
	Rooms(cookies)
	Classes(cookies)
	Subjects(cookies)
	Timetable(cookies, userID)
	Teachers(cookies)
}

func Auth(user, password, userID string) ([]*http.Cookie, error) {
	l := Login{"2023-05-06 15:44:22.215292", "authenticate", Params{user, password, "WebUntis Test"}, "2.0"}
	loginJSON, err := json.Marshal(l)
	if err != nil {
		return nil, err
	}
	login := bytes.NewReader(loginJSON)
	if Url == "" {
		Url = os.Getenv("URL")
	}
	LoginOut, err := http.Post(Url, "application/json", login)
	if err != nil {
		return nil, err
	}
	defer LoginOut.Body.Close()

	cookies := LoginOut.Cookies()
	log.Printf("Login successful for user: %s", user)

	response, err := io.ReadAll(LoginOut.Body)
	if err != nil {
		return nil, err
	}

	var Response LoginResponse
	if err := json.Unmarshal(response, &Response); err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(Response.Result, "", "  ")
	if err != nil {
		return nil, err
	}

	loginFile := "login.json"
	if userID != "" {
		loginFile = "login_" + userID + ".json"
	}
	if err := os.WriteFile(loginFile, data, 0o644); err != nil {
		return nil, err
	}

	return cookies, nil
}
