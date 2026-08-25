package Untis

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var Location *time.Location

func init() {
	godotenv.Load("../.env")
	timezone := os.Getenv("LOCATION_ENV")
	if timezone == "" {
		timezone = "UTC"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("Failed to load timezone, set as UTC")
	}
	Location = loc
}

func GetTime() time.Time {
	return time.Now().In(Location)
}
