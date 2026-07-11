package main

import (
	"encoding/json"
	"testing"

	"github.com/google/go-github/v78/github"
)

// A fake issue to exercise buildSlackMessage without hitting GitHub.
// go-github fields are pointers, so github.Ptr(...) wraps each literal.
func fixtureIssue(association string) *github.Issue {
	return &github.Issue{
		Number:            github.Ptr(42),
		Title:             github.Ptr("Poller drops issues when Slack is down"),
		Body:              github.Ptr("Steps to reproduce:\n1. Stop Slack\n2. Open an issue"),
		HTMLURL:           github.Ptr("https://github.com/Ackberry/git-webhook-slack/issues/42"),
		AuthorAssociation: github.Ptr(association),
		User: &github.User{
			Login:   github.Ptr("somebody"),
			HTMLURL: github.Ptr("https://github.com/somebody"),
		},
		Labels: []*github.Label{
			{Name: github.Ptr("bug")},
			{Name: github.Ptr("help wanted")},
		},
	}
}

// Prints the JSON your buildSlackMessage produces, for one team-tier author
// and one community author. Run with:
//
//	go test -run Preview -v
//
// Paste the output into https://app.slack.com/block-kit-builder to see the
// rendered message without posting anything to your real Slack channel.
func TestPreviewSlackMessage(t *testing.T) {
	cfg := config{slackMentionUserID: "U012ABCDEF"}
	for _, association := range []string{"OWNER", "NONE"} {
		msg := buildSlackMessage(fixtureIssue(association), "git-webhook-slack",
			"Ackberry/git-webhook-slack", "https://github.com/Ackberry/git-webhook-slack", cfg)
		out, err := json.MarshalIndent(msg, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		t.Logf("author_association=%s:\n%s", association, out)
	}
}
