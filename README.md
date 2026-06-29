# git-webhook-slack

Sends a Slack notification whenever an issue is **opened** in a GitHub repo.

## How it works

GitHub → POST `/webhook` → verify HMAC signature → if `issues`/`opened`, post to Slack Incoming Webhook.

## Setup

1. **Install**
   ```bash
   npm install
   ```

2. **Slack Incoming Webhook** — create one at
   https://api.slack.com/messaging/webhooks, pick the channel, copy the URL.

3. **Config** — copy `.env.example` to `.env` and fill in:
   ```bash
   cp .env.example .env
   # GITHUB_WEBHOOK_SECRET=<openssl rand -hex 32>
   # SLACK_WEBHOOK_URL=<your Slack webhook URL>
   ```

4. **Run**
   ```bash
   npm start
   ```

5. **Expose it** (for local dev) — GitHub needs a public URL:
   ```bash
   ngrok http 3000
   ```
   Use the `https://...ngrok...` URL below. For production, deploy anywhere
   (Render, Railway, Fly, a VPS) and use that host instead.

6. **Add the GitHub webhook** — in your repo: **Settings → Webhooks → Add webhook**
   - **Payload URL:** `https://<your-host>/webhook`
   - **Content type:** `application/json`
   - **Secret:** the same value as `GITHUB_WEBHOOK_SECRET`
   - **Events:** "Let me select individual events" → check **Issues** only
   - Save. GitHub sends a `ping`; you'll see it logged.

7. **Test** — open an issue in the repo. A message appears in Slack.

## Tracking repos you DON'T own (polling)

You can't add a webhook to a repo without admin rights. To track those, the app
can poll the GitHub REST API for new issues instead. Set:

```
POLL_REPOS=GoogleCloudPlatform/kubectl-ai          # comma-separated for multiple
GITHUB_TOKEN=<token>                                 # strongly recommended
POLL_INTERVAL_SECONDS=60                              # optional, default 60
```

- Only notifies issues created **after** the service starts (a restart won't replay old ones).
- Pull requests are filtered out (the `/issues` endpoint returns both).
- `GITHUB_WEBHOOK_SECRET` is optional in poll-only mode; set it only if you also
  receive webhooks from repos you own.

### Rate limits

The poller makes 1 request per repo per poll cycle:
`requests/hour = (3600 / POLL_INTERVAL_SECONDS) × number_of_repos`.

| Mode | Limit |
|---|---|
| Unauthenticated | 60 req/hr per IP |
| With `GITHUB_TOKEN` | 5,000 req/hr |

One repo at 60s = 60 req/hr, which is exactly the unauthenticated cap — so add a
`GITHUB_TOKEN` (fine-grained, public read is enough) for real headroom.

## Deploy to Railway

The repo is Railway-ready (`railway.json` sets the start command and a
`/healthz` healthcheck; the server binds to Railway's `$PORT` automatically).

1. **Push to GitHub** (Railway deploys from a repo). If this isn't a git repo yet:
   ```bash
   git init && git add . && git commit -m "Initial commit"
   # create a repo on GitHub, then:
   git remote add origin https://github.com/<you>/git-webhook-slack.git
   git push -u origin main
   ```
   `.env` is gitignored, so your secrets won't be pushed.

2. **Create the Railway project** — https://railway.com → **New Project → Deploy
   from GitHub repo** → pick this repo. Railway builds it with Nixpacks and runs
   `npm start`.

3. **Set environment variables** — in the service's **Variables** tab, add:
   - `GITHUB_WEBHOOK_SECRET` — same value you'll put in the GitHub webhook
   - `SLACK_WEBHOOK_URL` — your Slack Incoming Webhook URL

   Don't set `PORT` — Railway provides it.

4. **Generate a public domain** — **Settings → Networking → Generate Domain**.
   You'll get something like `https://git-webhook-slack-production.up.railway.app`.

5. **Add the GitHub webhook** (see step 6 below) using
   `https://<your-railway-domain>/webhook` as the Payload URL.

That's it — always-on, no laptop or ngrok needed.

## Notes

- Only `issues` with action `opened` notify. To cover more (closed, reopened,
  comments…), extend the handler in `server.js`.
- The signature check rejects any request not signed with your secret.
