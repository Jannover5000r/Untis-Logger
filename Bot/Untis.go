package Untis

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

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

var Url = "https://thalia.webuntis.com/WebUntis/jsonrpc.do?school=Mons_Tabor"
var Password = os.Getenv("UNTIS_PASSWORD")
var USERS = os.Getenv("UNTIS_USER")

func Main() {
	godotenv.Load("../.env")
	cookies, err := Auth()
	if err != nil {
		log.Fatal(err)
		return
	}
	Rooms(cookies)

	Classes(cookies)

	Subjects(cookies)

	Timetable(cookies)

	//getTeachers sends empty response
	Teachers(cookies)
}

func Auth() ([]*http.Cookie, error) {
	l := Login{"2023-05-06 15:44:22.215292", "authenticate", Params{USERS, Password, "WebUntis Test"}, "2.0"}
	loginJSON, err := json.Marshal(l)
	if err != nil {
		log.Fatalf("Error marshaling login data: %v", err)
		return nil, err
	}
	login := bytes.NewReader(loginJSON)

	//log.Println("Run")
	LoginOut, err := http.Post(Url, "application/json", login)
	if err != nil {
		log.Printf("Error during authentication: %v", err)
		return nil, err
	}
	defer LoginOut.Body.Close()

	// Parse cookies from response
	cookies := LoginOut.Cookies()
	//log.Println("Received cookies:", cookies)
	//log.Println("Set-Cookie headers:", LoginOut.Header["Set-Cookie"])
	//loginRespBody, _ := io.ReadAll(LoginOut.Body)
	//log.Println("Login response body:", string(loginRespBody))

	log.Println("Login successful")

	//log.Println("Login successful")
	response, err := io.ReadAll(LoginOut.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}
	var Response LoginResponse
	err = json.Unmarshal(response, &Response)
	if err != nil {
		log.Fatalf("Error unmarshaling response: %v", err)
	}
	data, err := json.MarshalIndent(Response.Result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile("login.json", data, 0644)
	if err != nil {
		log.Fatal(err)
	}

	return cookies, nil

}
