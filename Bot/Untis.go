package Untis

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type Params struct {
	Users    string `json:"users"`
	Password string `json:"key"`
	Client   string `json:"client"`
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

func Auth() {
	l := Login{"2023-05-06 15:44:22.215292", "authenticate", Params{os.Getenv("UNTIS_USER"), os.Getenv("UNTIS_PASSWORD"), "WebUntis Test"}, "2.0"}
	loginJSON, err := json.Marshal(l)
	login := bytes.NewReader(loginJSON)
	if err != nil {
		log.Fatalf("Error marshaling login data: %v", err)
		return
	}

	log.Println("Run")
	res, err := http.Post(Url, "application/json", login)

	if err != nil {
		log.Printf("Error during authentication: %v", err)
		return
	}
	log.Println(res)
	log.Println("")

}
