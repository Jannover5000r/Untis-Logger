package Untis

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

type TeachersResponse struct {
	Jsonrpc string     `json:"jsonrpc"`
	ID      string     `json:"id"`
	Result  []teachers `json:"result"`
}
type teachers struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LongName      string `json:"longName"`
	Active        bool   `json:"active"`
	AlternateName string `json:"alternateName"`
}
type getTeachers struct {
	Id      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Jsonrpc string      `json:"jsonrpc"`
}

func Teachers(cookies []*http.Cookie) {
	g := getTeachers{"2023-05-06 15:44:22.215292", "getTeachers", map[string]interface{}{}, "2.0"}
	TeachersJson, err := json.Marshal(g)
	if err != nil {
		log.Fatalf("Error marshaling login data: %v", err)
		return
	}
	teachers := bytes.NewReader(TeachersJson)

	prompt, err := http.NewRequest("POST", Url, teachers)
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
	var Response TeachersResponse
	err = json.Unmarshal(response, &Response)
	if err != nil {
		log.Fatalf("Error unmarshaling response: %v", err)
	}
	data, err := json.MarshalIndent(Response.Result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile("teachers.json", data, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Updated Teachers")
}
