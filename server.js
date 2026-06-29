import express from "express";
import crypto from "node:crypto";
import { buildSlackMessage, postToSlack } from "./slack.js";
import { startPolling } from "./poller.js";

const PORT = process.env.PORT || 3000;
const SLACK_WEBHOOK_URL = process.env.SLACK_WEBHOOK_URL;
const GITHUB_WEBHOOK_SECRET = process.env.GITHUB_WEBHOOK_SECRET;
const POLL_REPOS = (process.env.POLL_REPOS || "")
  .split(",")
  .map((s) => s.trim())
  .filter(Boolean);
const POLL_INTERVAL_SECONDS = Number(process.env.POLL_INTERVAL_SECONDS) || 300;
const GITHUB_TOKEN = process.env.GITHUB_TOKEN;

const webhookEnabled = Boolean(GITHUB_WEBHOOK_SECRET);
const pollEnabled = POLL_REPOS.length > 0;

if (!SLACK_WEBHOOK_URL) {
  console.error("Missing SLACK_WEBHOOK_URL (see .env.example).");
  process.exit(1);
}
if (!webhookEnabled && !pollEnabled) {
  console.error(
    "Nothing to do: set GITHUB_WEBHOOK_SECRET (to receive webhooks) and/or " +
      "POLL_REPOS (to poll repos you don't own). See .env.example."
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

if (webhookEnabled) {
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
        await postToSlack(
          buildSlackMessage({
            issue: payload.issue,
            repoName: payload.repository.name,
            repoFullName: payload.repository.full_name,
            repoHtmlUrl: payload.repository.html_url,
            author: payload.sender,
          })
        );
        console.log(
          `Notified Slack (webhook): ${payload.repository.full_name}#${payload.issue.number}`
        );
      } catch (err) {
        console.error("Failed to post to Slack:", err.message);
      }
    }
  });
}

app.get("/healthz", (_req, res) => res.send("ok"));

app.listen(PORT, () => {
  console.log(`Listening on http://localhost:${PORT}`);
  if (webhookEnabled) console.log("Webhook enabled at POST /webhook");
  if (pollEnabled) {
    startPolling({
      repos: POLL_REPOS,
      intervalSeconds: POLL_INTERVAL_SECONDS,
      token: GITHUB_TOKEN,
    });
  }
});
