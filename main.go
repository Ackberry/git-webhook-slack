package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v78/github"
)

// config mirrors the env vars the Node version used (see .env.example).
type config struct {
	port                string
	slackWebhookURL     string
	githubWebhookSecret string
	pollRepos           []string
	pollInterval        time.Duration
	githubToken         string
	slackMentionUserID  string
}

func loadConfig() config {
	cfg := config{
		port:                os.Getenv("PORT"),
		slackWebhookURL:     os.Getenv("SLACK_WEBHOOK_URL"),
		githubWebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		githubToken:         os.Getenv("GITHUB_TOKEN"),
		slackMentionUserID:  os.Getenv("SLACK_MENTION_USER_ID"),
	}
	if cfg.port == "" {
		cfg.port = "3000"
	}
	for _, repo := range strings.Split(os.Getenv("POLL_REPOS"), ",") {
		if repo = strings.TrimSpace(repo); repo != "" {
			cfg.pollRepos = append(cfg.pollRepos, repo)
		}
	}
	seconds, err := strconv.Atoi(os.Getenv("POLL_INTERVAL_SECONDS"))
	if err != nil || seconds <= 0 {
		seconds = 300
	}
	cfg.pollInterval = time.Duration(seconds) * time.Second
	return cfg
}

func main() {
	cfg := loadConfig()

	webhookEnabled := cfg.githubWebhookSecret != ""
	pollEnabled := len(cfg.pollRepos) > 0

	if cfg.slackWebhookURL == "" {
		log.Fatal("Missing SLACK_WEBHOOK_URL (see .env.example).")
	}
	if !webhookEnabled && !pollEnabled {
		log.Fatal("Nothing to do: set GITHUB_WEBHOOK_SECRET (to receive webhooks) and/or " +
			"POLL_REPOS (to poll repos you don't own). See .env.example.")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	if webhookEnabled {
		mux.HandleFunc("POST /webhook", webhookHandler(cfg))
		log.Println("Webhook enabled at POST /webhook")
	}
	if pollEnabled {
		client := github.NewClient(nil)
		if cfg.githubToken != "" {
			client = client.WithAuthToken(cfg.githubToken)
		}
		go startPolling(client, cfg)
	}

	log.Printf("Listening on http://localhost:%s", cfg.port)
	log.Fatal(http.ListenAndServe(":"+cfg.port, mux))
}
