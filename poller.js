// Polls the GitHub REST API for newly created issues on repos you don't own
// (you can't add a webhook to a repo without admin rights), and posts them to Slack.

import { buildSlackMessage, postToSlack } from "./slack.js";

export function startPolling({ repos, intervalSeconds, token }) {
  const intervalMs = intervalSeconds * 1000;

  // Only notify about issues created AFTER the service starts, so a restart
  // doesn't replay every existing issue. ISO strings compare lexicographically.
  const startIso = new Date().toISOString();
  const lastCreatedAt = Object.fromEntries(repos.map((r) => [r, startIso]));
  const seen = new Set(); // issue ids already posted, guards against dupes

  const headers = {
    Accept: "application/vnd.github+json",
    "User-Agent": "git-webhook-slack",
    "X-GitHub-Api-Version": "2022-11-28",
  };
  if (token) headers.Authorization = `Bearer ${token}`;

  async function poll(repo) {
    const [owner, name] = repo.split("/");
    if (!owner || !name) {
      console.error(`Skipping invalid repo "${repo}" (expected owner/name)`);
      return;
    }

    const url = `https://api.github.com/repos/${owner}/${name}/issues?state=open&sort=created&direction=desc&per_page=30`;
    let res;
    try {
      res = await fetch(url, { headers });
    } catch (err) {
      console.error(`Poll ${repo}: network error: ${err.message}`);
      return;
    }

    if (!res.ok) {
      const remaining = res.headers.get("x-ratelimit-remaining");
      console.error(
        `Poll ${repo}: GitHub responded ${res.status}` +
          (remaining !== null ? ` (rate limit remaining: ${remaining})` : "")
      );
      return;
    }

    const issues = await res.json();
    const cutoff = lastCreatedAt[repo];

    // The /issues endpoint also returns pull requests — exclude them.
    const realIssues = issues.filter((i) => !i.pull_request);

    const fresh = realIssues
      .filter((i) => i.created_at > cutoff && !seen.has(i.id))
      .sort((a, b) => a.created_at.localeCompare(b.created_at)); // oldest first

    for (const issue of fresh) {
      try {
        await postToSlack(
          buildSlackMessage({
            issue,
            repoName: name,
            repoFullName: repo,
            repoHtmlUrl: `https://github.com/${repo}`,
            author: issue.user,
          })
        );
        seen.add(issue.id);
        console.log(`Notified Slack (poll): ${repo}#${issue.number}`);
      } catch (err) {
        console.error(`Poll ${repo}: failed to post to Slack: ${err.message}`);
      }
    }

    // Advance the cutoff to the newest issue we've seen.
    const newest = realIssues
      .map((i) => i.created_at)
      .sort((a, b) => a.localeCompare(b))
      .pop();
    if (newest && newest > cutoff) lastCreatedAt[repo] = newest;
  }

  async function pollAll() {
    for (const repo of repos) await poll(repo);
  }

  console.log(
    `Polling ${repos.length} repo(s) every ${intervalSeconds}s: ${repos.join(", ")}` +
      (token ? " (authenticated)" : " (unauthenticated — see rate-limit note)")
  );
  setInterval(pollAll, intervalMs);
}
