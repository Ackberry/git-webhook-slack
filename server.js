import express from "express";
import crypto from "node:crypto";

const PORT = process.env.PORT || 3000;
const GITHUB_WEBHOOK_SECRET = process.env.GITHUB_WEBHOOK_SECRET;
const SLACK_WEBHOOK_URL = process.env.SLACK_WEBHOOK_URL;

if (!GITHUB_WEBHOOK_SECRET || !SLACK_WEBHOOK_URL) {
  console.error(
    "Missing config. Set GITHUB_WEBHOOK_SECRET and SLACK_WEBHOOK_URL (see .env.example)."
  );
  process.exit(1);
}

const app = express();

// Capture the raw body so we can verify GitHub's signature against the exact bytes sent.
app.use(
  express.json({
    verify: (req, _res, buf) => {
      req.rawBody = buf;
    },
  })
);

// Verify GitHub's HMAC-SHA256 signature using a timing-safe comparison.
function isValidSignature(req) {
  const signature = req.get("X-Hub-Signature-256");
  if (!signature) return false;

  const expected =
    "sha256=" +
    crypto
      .createHmac("sha256", GITHUB_WEBHOOK_SECRET)
      .update(req.rawBody)
      .digest("hex");

  const a = Buffer.from(signature);
  const b = Buffer.from(expected);
  return a.length === b.length && crypto.timingSafeEqual(a, b);
}

// Build a Slack message for a newly opened issue.
function buildSlackMessage(payload) {
  const { issue, repository, sender } = payload;
  const labels = issue.labels?.map((l) => l.name).join(", ") || "none";
  const body = issue.body?.trim()
    ? issue.body.length > 500
      ? issue.body.slice(0, 500) + "…"
      : issue.body
    : "_No description provided._";

  return {
    text: `New issue in ${repository.full_name}: #${issue.number} ${issue.title}`,
    blocks: [
      {
        type: "header",
        text: {
          type: "plain_text",
          text: `🐛 New issue: ${repository.name} #${issue.number}`,
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
          { type: "mrkdwn", text: `*Repo:*\n<${repository.html_url}|${repository.full_name}>` },
          { type: "mrkdwn", text: `*Opened by:*\n<${sender.html_url}|${sender.login}>` },
          { type: "mrkdwn", text: `*Labels:*\n${labels}` },
          { type: "mrkdwn", text: `*Issue #:*\n${issue.number}` },
        ],
      },
    ],
  };
}

async function postToSlack(message) {
  const res = await fetch(SLACK_WEBHOOK_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(message),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`Slack responded ${res.status}: ${text}`);
  }
}

app.post("/webhook", async (req, res) => {
  if (!isValidSignature(req)) {
    console.warn("Rejected webhook: invalid signature");
    return res.status(401).send("Invalid signature");
  }

  const event = req.get("X-GitHub-Event");
  const payload = req.body;

  // Respond fast so GitHub doesn't time out; do the Slack post after.
  res.status(204).end();

  if (event === "ping") {
    console.log("Received ping from GitHub — webhook is connected.");
    return;
  }

  if (event === "issues" && payload.action === "opened") {
    try {
      await postToSlack(buildSlackMessage(payload));
      console.log(
        `Notified Slack: ${payload.repository.full_name}#${payload.issue.number}`
      );
    } catch (err) {
      console.error("Failed to post to Slack:", err.message);
    }
  }
});

app.get("/healthz", (_req, res) => res.send("ok"));

app.listen(PORT, () => {
  console.log(`Listening on http://localhost:${PORT} — webhook at POST /webhook`);
});
