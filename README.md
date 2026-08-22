# Discord bot for logging Untis timetable to a Discord Webhook and Multiple accounts which can be added using !addaccount in a server whit the Bot

## create .env with Credentials

- UNTIS_PASSWORD
- UNTIS_USER
- DISCORD_WEBHOOK_URL
- DISCORD_BOT_TOKEN
- ENC_KEY (generated via head -c 32 /dev/urandom | base64)
- URL

### URL can be found by following this structure: "<https://[SchoolName].webuntis.com/WebUntis/jsonrpc.do?school=[SchholName]" while SchoolName can be found by visiting the official website, going to the login for your school and it will be: "<https://[SchoolName].webuntis.com/WebUntis/?school=[SchoolName]#/basic/login>"

# This is a project for me, issues will be resolved as i find the motivation to do so, improvements may follow in the future

## I will try to maintain this project as best as possible. Maybe add a better security to it than to trust the host but for now it is working and that was my goal. Please report any errors you find while using this bot

# Installation

# This is a discord application intended for self hosting. You will need to add an application in the [disord developer website](https://discord.com/developers/applications) for this bot mainly with the read messages and write messages privelege as well as send privat messages for the account handling. If you give it administrator permissions you dont have to worry further but this could present you with a security risk if my github account gets compromised and a malicious update to the script gets pushed which you than use since the bot got access to your whole server

# I also recommend to use a tool like [coolify](https://coolify.io/) to host the bot. Make sure to add the Env's and add the **accounts/** folder to persistent storage

### This bot is self hosted and will run correctly when doing "go run . " in the root folder of the project. **Before usage add the .env file with the Credentials as mentioned above.** When you run the Program for the first time, all important files will be created automatically and the bot is ready to go. The user added via the fields UNTIS_USER and UNTIS_PASSWORD in the .env will be the one where the Webhook is sourced from and the other users will be send a DM after adding their account with the command. Add the Discord Webhook via the DISCORD_WEBHOOK_URL point in the .env and will send the messages for the timetable in there

### For multi user support: using the command **!addaccount** in the discord server the bot is used in, it will than dm you with the sign up process prompting you to enter your untis Username and password (no email neededd just untis username). The username is stored as plain text and the password is hashed on the device hosting the bot, so currently it is **not secure** since the host will have access to the hashed password and the hash used! Deleting an account is done by using **!removeaccount** which will delete the account and the password, created with your discord account

## For your timezone setup an ENV called "LOCATION_ENV" with your timezone as explained in the example env file. In case your school is using a different timing for the Lessons than mine, you can change the times where you will be notified with the next room and Lesson for the day in **Line 71 in the main.go** file. The bot may not send you a message if there is no lesson after the specified time so that you can leave empty lessons as is for the days

# I am neither a representative of Untis, Untis Baden-Württemberg GmbH nor a Developer in their team. This project is based on their API and my code and is not affiliated with them
