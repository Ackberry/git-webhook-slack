// Shared Slack message building + posting, used by both the webhook and the poller.

// Build a Slack message for a newly opened issue.
// `issue` is a GitHub issue object (same shape from webhook payloads and the REST API).
export function buildSlackMessage({ issue, repoName, repoFullName, repoHtmlUrl, author }) {
  const labels =
    issue.labels?.map((l) => (typeof l === "string" ? l : l.name)).join(", ") ||
    "none";
  const body = issue.body?.trim()
    ? issue.body.length > 500
      ? issue.body.slice(0, 500) + "…"
      : issue.body
    : "_No description provided._";

  return {
    text: `New issue in ${repoFullName}: #${issue.number} ${issue.title}`,
    blocks: [
      {
        type: "header",
        text: {
          type: "plain_text",
          text: `🐛 New issue: ${repoName} #${issue.number}`,
        },
      },
      {
        type: "section",
        text: {
          type: "mrkdwn",
          text: `*<${issue.html_url}|${issue.title}>*\n${body}`,
        },
      },
      {
        type: "section",
        fields: [
          { type: "mrkdwn", text: `*Repo:*\n<${repoHtmlUrl}|${repoFullName}>` },
          { type: "mrkdwn", text: `*Opened by:*\n<${author.html_url}|${author.login}>` },
          { type: "mrkdwn", text: `*Labels:*\n${labels}` },
          { type: "mrkdwn", text: `*Issue #:*\n${issue.number}` },
        ],
      },
    ],
  };
}

export async function postToSlack(message) {
  const res = await fetch(process.env.SLACK_WEBHOOK_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(message),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`Slack responded ${res.status}: ${text}`);
  }
}
