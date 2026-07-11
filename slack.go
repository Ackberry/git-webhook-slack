// Shared Slack message building + posting, used by both the webhook and the
// poller. This file is the pairing half owned by YOU — look for TODO(you).

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/go-github/v78/github"
)

// Slack Block Kit payloads (https://api.slack.com/block-kit). Only the fields
// we use. The `json:"..."` tags map Go field names to the JSON keys Slack
// expects; `omitempty` drops a field from the output when it's zero-valued,
// so a header block doesn't carry an empty "fields": [].
type textObject struct {
	Type string `json:"type"` // "plain_text" or "mrkdwn"
	Text string `json:"text"`
}

type block struct {
	Type   string       `json:"type"`             // "header" or "section"
	Text   *textObject  `json:"text,omitempty"`   // headers + plain sections
	Fields []textObject `json:"fields,omitempty"` // two-column section layout
}

type slackMessage struct {
	Text   string  `json:"text"` // fallback shown in notifications
	Blocks []block `json:"blocks,omitempty"`
}

// ─── Exercise 1 · TODO(you) ─────────────────────────────────────────────────
// Map GitHub's author_association onto the tiers we show in Slack:
//
//	OWNER, MEMBER, COLLABORATOR, CONTRIBUTOR → "Contributor"
//	FIRST_TIME_CONTRIBUTOR, FIRST_TIMER      → "First-time contributor"
//	anything else (NONE, MANNEQUIN, …)       → "Community"
//
// Hint: a Go switch can take several values in one case —
//
//	switch association {
//	case "OWNER", "MEMBER":
//		return ...
//	}
//
// — and there is no fall-through between cases (unlike C/JS). `default:`
// handles the catch-all.



func roleLabel(association string) string {
	switch association {
	case "OWNER", "MEMBER", "CONTRIBUTOR", "COLLABORATOR":
		return "high"
	default:
		return "normal"

	}
	// TODO(you): replace with the real mapping
}

// ─── Exercise 2 · TODO(you) ─────────────────────────────────────────────────
// Shorten s to at most max characters, appending "…" if anything was cut.
// (slack.js did: body.length > 500 ? body.slice(0, 500) + "…" : body)
//
// Gotcha to look up first: s[:max] slices BYTES, not characters. An emoji or
// accented letter is several bytes in UTF-8, so byte-slicing can cut one in
// half. Convert to a rune slice first — runes := []rune(s) — slice that, and
// convert back with string(...).
func truncate(s string, max int) string {
	runedString := []rune(s)
	sliced_runedString := runedString[:max]
	s = string(sliced_runedString)
	return s + "..." // some runes chars not splitting stuff. 
}

// ─── Exercise 3 · TODO(you) ─────────────────────────────────────────────────
// Skeleton in place: the slice mechanics (literal + conditional append) are
// done; the TODO(you) comments inside mark the content left to build.
// Compare with slack.js for the original message shape.
//
// Key rule your first attempt tripped on: a composite literal ({...}) may
// only contain field: value pairs. Statements — if, append, loops — cannot
// live inside it; they go before or after, as their own lines.
//
// go-github tips: every struct field is a pointer, so use the nil-safe
// getters — issue.GetTitle(), issue.GetBody(), issue.GetHTMLURL(),
// issue.GetNumber(), issue.GetAuthorAssociation(), issue.GetUser().GetLogin(),
// issue.GetUser().GetHTMLURL(). fmt.Sprintf works like template literals.
// Iterate with: go test -run Preview -v  → paste the JSON into the Block Kit
// Builder (https://app.slack.com/block-kit-builder) to see it rendered.
func buildSlackMessage(issue *github.Issue, repoName, repoFullName, repoHTMLURL string, cfg config) slackMessage {
	blocks := []block{
		{
			Type: "header",
			Text: &textObject{
				Type: "plain_text",
				// TODO(you): vary this by statusLabel(issue.GetAuthorAssociation())
				// — different emoji/wording for "high" vs "normal".
				Text: fmt.Sprintf("🐛 New issue: %s #%d", repoName, issue.GetNumber()),
			},
		},
	}

	if cfg.slackMentionUserID != "" {
		blocks = append(blocks, block{
			Type: "section",
			Text: &textObject{Type: "mrkdwn", Text: "<@" + cfg.slackMentionUserID + ">"},
		})
	}
	body := issue.GetBody()
	if body == "" {
		body = "_.No description provided._"
	}
	blocks = append(blocks, block{
		Type: "section",
		Text: &textObject{Type: "mrkdwn", Text: fmt.Sprintf(" *<%s|%s>*\n%s"),issueURL, title, body},
	})
	// TODO(you): append the issue-link section here. mrkdwn text in the shape
	//   *<issueURL|title>*\n<body>
	// where body is truncate(issue.GetBody(), 500), or
	// "_No description provided._" when GetBody() is empty.

	blocks = append(blocks, block{
		Type: "section",
		Fields: []textObject{
			{Type: "mrkdwn", Text: fmt.Sprintf("*Repo:*\n<%s|%s>", repoHTMLURL, repoFullName)},
			{Type: "mrkdwn", Text: fmt.Sprintf("Login: %s ",)}
			{Type: "mrkdwn", Text: fmt.Sprintf("status: %s", roleLabel())}
			var labelList string[]
			for label := range(issue.Labels) {
				labelList = append(labelList, label)
			}
			if len(labelList) != 0 {
				label := strings.Join(labelList, ", ")
			}else {
				label = "none"
			}
			// TODO(you): four more fields — Opened by (login, linked to
			// profile), Status (your exercise-1 function!), Labels, Issue #.
			// Labels needs a for loop: range issue.Labels, collect
			// l.GetName() into a []string, then strings.Join(names, ", ")
			// — "none" if empty. You'll need to import "strings".
		},
	})

	return slackMessage{
		Text:   fmt.Sprintf("New issue in %s: #%d %s", repoFullName, issue.GetNumber(), issue.GetTitle()),
		Blocks: blocks,
	}
}

func postToSlack(webhookURL string, msg slackMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		text, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack responded %d: %s", resp.StatusCode, text)
	}
	return nil
}
