package Untis

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var (
	locerr   error
	Location *time.Location
)

func GetTime() time.Time {
	godotenv.Load("../.env")
	timezone := os.Getenv("LOCATION_ENV")
	Location, locerr = time.LoadLocation(timezone)
	if locerr != nil {
		log.Fatalf("Failed to load Timezone: %v", locerr)
	}
	return time.Now().In(Location)
}
