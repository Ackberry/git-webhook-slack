package main

import (
	"log"
	"net/http"

	"github.com/google/go-github/v78/github"
)

// webhookHandler returns the POST /webhook handler.
//
// go-github does the two fiddly parts server.js hand-rolled:
//   - ValidatePayload checks the X-Hub-Signature-256 HMAC against the raw
//     body bytes, with a constant-time comparison.
//   - ParseWebHook turns the raw JSON into a typed event struct we can
//     switch on, instead of string-matching event names and actions.
func webhookHandler(cfg config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := github.ValidatePayload(r, []byte(cfg.githubWebhookSecret))
		if err != nil {
			log.Printf("Rejected webhook: %v", err)
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		event, err := github.ParseWebHook(github.WebHookType(r), payload)
		if err != nil {
			http.Error(w, "unparseable payload", http.StatusBadRequest)
			return
		}

		switch e := event.(type) {
		case *github.PingEvent:
			log.Println("Received ping from GitHub — webhook is connected.")
		case *github.IssuesEvent:
			if e.GetAction() != "opened" {
				break
			}
			repo := e.GetRepo()
			issue := e.GetIssue()
			// Post to Slack in a goroutine so GitHub gets its 204 immediately
			// (same "respond fast, then work" idea as the Node version).
			go func() {
				msg := buildSlackMessage(issue, repo.GetName(), repo.GetFullName(), repo.GetHTMLURL(), cfg)
				if err := postToSlack(cfg.slackWebhookURL, msg); err != nil {
					log.Printf("Failed to post to Slack: %v", err)
					return
				}
				log.Printf("Notified Slack (webhook): %s#%d", repo.GetFullName(), issue.GetNumber())
			}()
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
