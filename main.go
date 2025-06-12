package main

import (
	"os"
	Untis "untislogger/Bot"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	Untis.BotToken = os.Getenv("DISCORD_TOKEN")
	Untis.Run() // call the run function of Bot/Untis.go
}
