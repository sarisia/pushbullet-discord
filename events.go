package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type DiscordWebhookPayload struct {
	Embeds []Embed `json:"embeds"`
}

type Embed struct {
	Color       int    `json:"color"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

var httpCli = &http.Client{
	Timeout: 30 * time.Second,
}

func NewPushToDiscordHandler(webhook string) func(context.Context, *PushPayload) {
	return func(_ context.Context, p *PushPayload) {
		if p.Push.Type == "dismissal" {
			return
		}

		wp := DiscordWebhookPayload{
			[]Embed{
				{
					Color:       0x00acee,
					Title:       p.Push.Title,
					Description: p.Push.Body,
				},
			},
		}

		b, err := json.Marshal(&wp)
		if err != nil {
			log.Printf("failed to marshal discord webhook payload: %v", err)
			return
		}

		resp, err := httpCli.Post(webhook, "application/json", bytes.NewBuffer(b))
		if err != nil {
			log.Printf("failed to execute webhook: %v", err)
			return
		}
		defer resp.Body.Close()
		respBuf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("failed to read discord response: %v", err)
			return
		}

		if resp.StatusCode/100 != 2 {
			log.Printf("webhook failed (%s %s)", resp.Status, respBuf)
		}
	}
}
