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
type getRooms struct {
	Id      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Jsonrpc string      `json:"jsonrpc"`
}
type Login struct {
	Id     string `json:"id"`
	Method string `json:"method"`
	Params Params `json:"params"`

	Jsonrpc string `json:"jsonrpc"`
}

type LoginResponse struct {
	result struct {
		ID         string `json:"sessionId"`
		PersonType string `json:"personType"`
		PersonId   string `json:"personId"`
		KlasseId   string `json:"klasseId"`
	}
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
}

func Auth() ([]*http.Cookie, error) {
	l := Login{"2023-05-06 15:44:22.215292", "authenticate", Params{os.Getenv("UNTIS_USER"), os.Getenv("UNTIS_PASSWORD"), "WebUntis Test"}, "2.0"}
	loginJSON, err := json.Marshal(l)
	if err != nil {
		log.Fatalf("Error marshaling login data: %v", err)
		return nil, err
	}
	login := bytes.NewReader(loginJSON)

	log.Println("Run")
	LoginOut, err := http.Post(Url, "application/json", login)
	if err != nil {
		log.Printf("Error during authentication: %v", err)
		return nil, err
	}
	defer LoginOut.Body.Close()

	// Parse cookies from response
	cookies := LoginOut.Cookies()
	log.Println("Received cookies:", cookies)
	log.Println("Set-Cookie headers:", LoginOut.Header["Set-Cookie"])
	loginRespBody, _ := io.ReadAll(LoginOut.Body)
	log.Println("Login response body:", string(loginRespBody))
	return cookies, nil

}

func Rooms(cookies []*http.Cookie) {
	//log.Println("Abrufen der Stunden")
	g := getRooms{"2023-05-06 15:44:22.215292", "getRooms", map[string]interface{}{}, "2.0"}
	roomsJson, err := json.Marshal(g)
	if err != nil {
		log.Fatalf("Error marshaling login data: %v", err)
		return
	}
	rooms := bytes.NewReader(roomsJson)

	prompt, err := http.NewRequest("POST", Url, rooms)
	if err != nil {
		log.Fatalf("Error creatingrequest: %v", err)
		return
	}
	//log.Println("prompt without extra header or cookie ", prompt)
	log.Println("Cookie: ", cookies)

	prompt.Header.Set("Content-Type", "application/json")
	prompt.Header.Set("User-Agent", "Webuntis Test")

	for _, cookie := range cookies {
		//if cookie.Name == "JSESSIONID" {
		prompt.AddCookie(cookie)
		log.Printf("Added JSESSIONID cookie: %s=%s", cookie.Name, cookie.Value)
		//}
	}
	log.Println("Request JSON:", string(roomsJson))
	out, err := http.DefaultClient.Do(prompt)
	if err != nil {
		log.Printf("Error during request: %v", err)
		return
	}
	defer out.Body.Close()
	log.Println(out.Status)
	response, err := io.ReadAll(out.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return
	}
	responseString := string(response)
	log.Println("Repsonse ", responseString)

}
