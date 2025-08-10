# Discord bot for logging Untis timetable to a Discord Webhook and Multiple accounts which can be added using !addaccount in a server whit the Bot
## create .env with Credentials:
- UNTIS_PASSWORD
- UNTIS_USER
- DISCORD_WEBHOOK_URL
- DISCORD_BOT_TOKEN
- ENC_KEY (generated via head -c 32 /dev/urandom | base64)

# This is a project for me, issues will be resolved as i find the motivation to do so, improvements may follow in the future
### I will try to maintain this project as best as possible. Maybe add a better security to it than to trust the host but for now it is working and that was my goal. Please report any errors you find while using this bot.

# Installation

### This bot is self hosted and will run correctly when doing "go run . " in the root folder of the project. Before usage add the .env file with the Credentials as mentioned above. When you run the Programm for the first time, all important files will be created automatically and the bot is ready to go. The user added via the fields UNTIS_USER and UNTIS_PASSWORD in the .env will be the one where the Webhook is sourced from and the other users will be send a DM after adding their account with the command.

## In case your school is using a different timing for the Lessons than mine, you can change the times where you will be notified with the next room and Lesson for the day in **Line 71 in the main.go** file.

# I am neither a representative of Untis Untis Baden-Württemberg GmbH nor a Developer in their team. This project is based on their API and is not affiliated with them.