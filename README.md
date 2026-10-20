<p align="right">
  <b>English</b> · <a href="README.zh.md">简体中文</a> · <a href="README.zh-TW.md">繁體中文</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="README.assets/logo-dark.svg" />
    <img src="README.assets/logo.svg" alt="Spearhook" width="96" height="96" title="Spearhook" />
  </picture>
</p>

<h1 align="center">Spearhook</h1>

<p align="center">
  <strong>Self-hosted phishing simulation console for authorized red teams</strong>
</p>

<p align="center">
  Go + Gin · Next.js · SQLite · single binary · optional remote agents
</p>

<p align="center">
  <a href="#legal-notice">Legal</a> ·
  <a href="#overview">Overview</a> ·
  <a href="#features">Features</a> ·
  <a href="https://p1temax.github.io/Spearhook/">Live demo</a> ·
  <a href="#architecture">Architecture</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="#agents">Agents</a> ·
  <a href="#console">Console</a> ·
  <a href="#disclaimer">Disclaimer</a>
</p>

---

## Legal notice

Spearhook is intended exclusively for authorized security testing, red-team exercises, and phishing awareness training.

Unauthorized use against systems, accounts, or individuals is unlawful. Users are solely responsible for obtaining authorization and for their operation of this software. The authors and contributors accept no liability for misuse.

Do not use Spearhook without written authorization or other clearly verifiable authorization.

---

## Overview

**Spearhook** is a self-hosted phishing simulation console covering landing-page hosting, email delivery, open and click tracking, credential capture, and real-time submission notifications. Pages can run on the control plane or be deployed to remote agents.

For readers familiar with [Gophish](https://github.com/gophish/gophish): that toolkit focuses on campaigns and templates, whereas Spearhook is an end-to-end operations console for the engagement. It combines an AI-assisted page library, an AI operations assistant, multi-agent deployment, live QR relays, AI-powered OSINT collection, webhooks, IP blacklisting, audit logging, and role management.

One Go binary embeds the UI (`frontend/dist`) and serves API + console on the same port. Stack: Go 1.24 / Gin / GORM+SQLite / Next.js static export / optional Docker SDK.

---

## Languages

English is the source language for the console and the primary documentation
language, and it is the fallback when a translation is missing. The web console
also ships with Simplified Chinese, Traditional Chinese, Japanese, and Korean.
Use the language picker in the header or on the login screen; the choice is
stored per browser. This README is maintained in all five languages; English
remains the source of truth when they differ.

## Features

- **AI Assistant** — natural-language ops assistant (`/chat`). Token-level SSE streaming via a whitelisted tool registry (`list_agents`, `clone_page`, `create_project`, `start_info_gather`, `create_mail_draft`, `send_mail_campaign`, QR/credentials/blacklist, …). Read-only tools execute automatically, destructive actions require confirmation, and every write is recorded in the audit log. The assistant does not generate HTML directly; URL mirroring is handled by the page-library pipeline.
- **Page library / AI mirror** — upload HTML or clone a public login/landing URL into the library via AI rewrite (OpenAI-compatible `chat/completions` or Anthropic `/messages`). Relative CSS/assets are absolutized against the source origin so previews keep styles.
- **Projects** — bind a library page, run with **Docker** or **local Flask**, optional TLS, one or many agents, QR relay placeholders. Credential submits, logs, and deployments roll up per project.
- **Mail** — SMTP profiles, templates, scheduled / bulk send, per-campaign open & click tracking (`/api/<slug>`). Optional HTML noise. Tracking public URL and HMAC live under System → Mail Tracking.
- **Push channels** — Feishu, WeCom, Telegram, Slack, DingTalk, and Discord. Bind a webhook to a project to receive notifications on credential submission; opens and clicks do not trigger this path.
- **QR phishing** — stable public image at `/q/{slug}.png`. A local capture script uploads screen frames; pages use `{{QR_RELAY_URL}}` / `{{QR_RELAY_IMG}}`.
- **Information gathering** — An AI endpoint plus a configurable web-search provider (native `web_search`, DeepSeek Search, Brave, Tavily, Serper, or Bocha). Collect publicly available email addresses and phone numbers with sources, export them as CSV, and use them for mail delivery.
- **Agents** — `spearhook-agent` runs on additional hosts. A single agent can host multiple projects, and a single project can be deployed to multiple agents. Agents provide heartbeats and buffer access logs in `pending-logs.json` for retransmission after recovery. Credential submissions are proxied to the control plane in real time; if the control plane is unreachable, visitors receive HTTP 502 (no submission queue is provided).
- **Auth & roles** — JWT login; optional HTTP Basic perimeter; **admin** / **operator** with ownership scoping; append-only audit log (no product delete).
- **Security and operations** — Dashboard and host CPU/memory monitoring, encryption of secrets and passwords at rest, QQWry geolocation, CSV export, and an **IP blacklist** for blocking abusive submission sources. The UI is available in English, Simplified Chinese, Traditional Chinese, Japanese, and Korean. AI Settings supports OpenAI-compatible and Anthropic Messages profiles for page mirroring, the AI Assistant, and information gathering.

**Standard workflow:** Configure SMTP and webhooks → mirror or upload a page → create a project (bind agents and push channels) → send mail → await submission notifications → review records. If submission volume is abusive, add the source IP to the blacklist.

---

## Architecture

```text
  Browser (i18n UI · JWT · optional Basic Auth)
           │
           ▼
  Control plane  (Go / Gin · SQLite · embedded Next UI · Assistant SSE)
           │
     ┌─────┴──────────────┐
     ▼                    ▼
  Docker / local Flask   Remote spearhook-agent(s)
     │                    │
     └─────────┬──────────┘
               ▼
  submits · access logs · mail open/click · live QR · webhooks
```

| Piece | Role |
|-------|------|
| Control plane (`spearhook`) | Console, API, SQLite, mail tracking endpoints, QR public URL, AI assistant orchestrator |
| Agent (`spearhook-agent`) | Edge landing-page runtime; initiates an **outbound** connection to the control plane |
| Frontend | Next.js App Router → static `frontend/dist`, embedded via `go:embed` |
| AI profile | One enabled OpenAI-compatible or Anthropic Messages endpoint for page mirror, assistant tools, and info gather |
| Search provider | Web search for info gather: native `web_search` or DeepSeek / Brave / Tavily / Serper / Bocha |

---

## Quick start

Linux, macOS, or Windows.

| Need | Notes |
|------|--------|
| Run a release | Binary + `config.yaml` |
| Build from source | Go **1.24+**, Node.js (frontend) |
| Docker run mode | Docker Engine reachable; skip if you only use local Flask / agents |
| Local Flask run mode | `python3` + `flask`, `flask_cors`, `requests` |
| QR capture | Python 3 + zip from **Workbench → QR Phishing** (or [`scripts/qr-relay`](scripts/qr-relay/README.md)); macOS decode wants `zbar` |
| Agents | Must reach the control plane over HTTP(S) |

| What | Default |
|------|---------|
| Console / API | `:8000` |
| Frontend dev | `:8081` → proxies `/api` to `:8000` |
| Agent landing ports | `20000–30000/TCP` on the agent host |

### 1. Config

```bash
cp config.yaml.example config.yaml
```

Replace placeholders before a real engagement:

```yaml
server:
  port: 8000
  gin_mode: release

database:
  path: ./data/fishing.db

security:
  encryption_key: replace-with-a-generated-key
  jwt_secret: replace-with-a-long-random-secret
  container_secret: replace-with-another-long-random-secret
  agent_registration_token: replace-with-agent-registration-token
  platform_basic_auth_enabled: true

container:
  ssl_cert_path: /app/certificates/cert.pem
  ssl_key_path: /app/certificates/key.pem
```

`cmd/generate-encryption-key` can mint `encryption_key`. Mail tracking (public URL, HMAC, redirect allow-list) is configured under **System → Mail Tracking** after boot — YAML is seed/fallback. Click/open paths are generated per campaign.

Do not commit `config.yaml`, `data/`, or TLS keys (gitignored).

### 2. Run

```bash
./spearhook
# or a build from ./release/ after ./build-all.sh
```

Empty/default secrets trigger a startup confirm — fine on a lab box. Rotate them before a real job.

### 3. Build

```bash
./build-all.sh
```

Builds the frontend, then control plane + agent for linux / windows / darwin (amd64 & arm64), and copies `config.yaml.example` + `agent.yaml.example` into `release/`.

UI only (API already up):

```bash
cd frontend && npm install && npm run dev   # http://127.0.0.1:8081
```

### 4. Login

Open `http://localhost:8000` (or your `server.port`). First boot writes admin credentials to:

```bash
cat data/admin_password.txt
```

Save the password, then delete the file. Later resets: `cmd/reset-password`.

---

## Agents

Deploy **spearhook-agent** to additional hosts when not every landing page should run on the control plane. A single project can be deployed to multiple nodes, and a single node can host multiple projects. Each binding has its own port, revision, and logs. Credential submissions are proxied to the control plane in real time; access logs are buffered in `pending-logs.json` and retransmitted when the control plane becomes reachable again.

Project ports default to **`20000–30000/TCP`**. The agent initiates an **outbound** connection. Visitors access the agent directly; when it is behind a reverse proxy, configure `advertise_host` as described in the full guide.

**Token** — set `security.agent_registration_token` on the plane (not the JWT secret) and restart. First register uses that shared token once; then `{work_dir}/state.json` holds the per-agent id/token (`0600` on Linux/macOS).

**Start** — after `./build-all.sh`:

```bash
cp agent.yaml.example agent.yaml
```

```yaml
server_url: http://control-plane.example.com:8000   # no trailing slash
registration_token: replace-with-a-long-random-agent-token
```

```bash
chmod +x ./spearhook-agent
./spearhook-agent -config ./agent.yaml
```

```powershell
.\spearhook-agent-windows-amd64.exe -config .\agent.yaml
```

You want a log line like `agent … connected to http://…`.

**In the UI** — **System → Agents** until online (~90s without heartbeat = offline). **Projects → New**, pick agents, attach a page. Each agent is its own instance (`pending` → `deploying` → `running`). Edit = new revision; add agent = deploy; uncheck = delete task.

systemd, Windows service, reverse proxy: **[docs/agent-deployment.md](docs/agent-deployment.md)**.

---

## Console

| Area | What you do there |
|------|-------------------|
| **Dashboard** | Counts and host CPU / memory |
| **AI Assistant** | Natural-language ops via platform tools (clone, create project, list agents, …) |
| **Projects** | Overview, deployments, logs, credential records |
| **Workbench → Page Builder** | Upload HTML or AI-mirror a URL into the page library |
| **Workbench → Send Email** | Templates, campaigns, open/click |
| **Workbench → Info Gathering** | AI web search → contacts CSV |
| **Workbench → QR Phishing** | Live QR relays + capture script zip |
| **Push Channels** | Webhooks on credential submission |
| **Agents** | Online status / last seen |
| **IP Blacklist** | Block abusive source IPs (added from credential records) |
| **Mail Services** | SMTP profiles |
| **Users / AI Settings / Mail Tracking** | Admin |
| **Audit Logs** | Admin; immutable in-product |

Pages POST to `/api/submit` by default. HTML placeholders: `{{SUBMIT_URL}}`, `{{REDIRECT_URL}}`, `{{QR_RELAY_URL}}`, `{{QR_RELAY_IMG}}`, …. Mail: `{{email}}`, `{{click_url}}`, `{{open_pixel}}`, `{{landing_url}}`, ….

Guides: [EN](docs/placeholders.en.md) · [ZH](docs/placeholders.zh.md)

Access log: `data/access-YYYY-MM-DD.log`. High-frequency dashboard polls stay out of the console stream; they still land in the file.

---

## Disclaimer

Spearhook is intended solely for authorized red-team phishing exercises and security-awareness training. Any use against people or systems for which you lack authorization is the sole responsibility of the user; the project does not support such use.

Code is under the [MIT License](LICENSE).

Issues and PRs welcome. Open a GitHub Issue if something is broken.
