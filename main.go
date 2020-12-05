package main

import (
	"log"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	// Pushbullet token
	pbToken := os.Getenv("PUSHBULLET_TOKEN")
	if pbToken == "" {
		log.Printf("Missing Pushbullet token")
		return
	}

	// Discord webhook
	webhook := os.Getenv("DISCORD_WEBHOOK")
	if webhook == "" {
		log.Printf("Missing Discord webhook")
		return
	}

	cli := NewPushbulletClient(pbToken)
	cli.AddHandler(NewPushToDiscordHandler(webhook))

	cli.Open()
	<-make(chan struct{})
}
