package main

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v78/github"
)

// startPolling checks each repo for newly created issues on an interval.
// Used for repos we can't add webhooks to (no admin rights).
func startPolling(client *github.Client, cfg config) {
	// Only notify about issues created after startup, so a restart doesn't
	// replay every existing issue.
	start := time.Now()
	cutoffs := make(map[string]time.Time, len(cfg.pollRepos))
	for _, repo := range cfg.pollRepos {
		cutoffs[repo] = start
	}
	seen := make(map[int64]bool) // issue IDs already posted, guards against dupes

	auth := "unauthenticated — see rate-limit note"
	if cfg.githubToken != "" {
		auth = "authenticated"
	}
	log.Printf("Polling %d repo(s) every %s: %s (%s)",
		len(cfg.pollRepos), cfg.pollInterval, strings.Join(cfg.pollRepos, ", "), auth)

	for range time.Tick(cfg.pollInterval) {
		for _, repo := range cfg.pollRepos {
			pollRepo(client, repo, cutoffs, seen, cfg)
		}
	}
}

func pollRepo(client *github.Client, repo string, cutoffs map[string]time.Time, seen map[int64]bool, cfg config) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		log.Printf("Skipping invalid repo %q (expected owner/name)", repo)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	issues, _, err := client.Issues.ListByRepo(ctx, owner, name, &github.IssueListByRepoOptions{
		State:       "open",
		Sort:        "created",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 30},
	})
	if err != nil {
		// go-github surfaces rate limiting as a typed error, so we can say
		// exactly when it lifts instead of parsing headers by hand.
		var rateErr *github.RateLimitError
		if errors.As(err, &rateErr) {
			log.Printf("Poll %s: rate limited until %s", repo, rateErr.Rate.Reset.Time)
			return
		}
		log.Printf("Poll %s: %v", repo, err)
		return
	}

	cutoff := cutoffs[repo]
	newest := cutoff
	var fresh []*github.Issue
	for _, issue := range issues {
		if issue.IsPullRequest() { // the issues endpoint also returns PRs
			continue
		}
		created := issue.GetCreatedAt().Time
		if created.After(newest) {
			newest = created
		}
		if created.After(cutoff) && !seen[issue.GetID()] {
			fresh = append(fresh, issue)
		}
	}
	// Oldest first, so Slack messages land in the order issues were opened.
	sort.Slice(fresh, func(i, j int) bool {
		return fresh[i].GetCreatedAt().Time.Before(fresh[j].GetCreatedAt().Time)
	})

	for _, issue := range fresh {
		msg := buildSlackMessage(issue, name, repo, "https://github.com/"+repo, cfg)
		if err := postToSlack(cfg.slackWebhookURL, msg); err != nil {
			log.Printf("Poll %s: failed to post to Slack: %v", repo, err)
			continue
		}
		seen[issue.GetID()] = true
		log.Printf("Notified Slack (poll): %s#%d", repo, issue.GetNumber())
	}

	cutoffs[repo] = newest
}
