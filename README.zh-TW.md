<p align="right">
  <a href="README.md">English</a> · <a href="README.zh.md">简体中文</a> · <b>繁體中文</b> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="README.assets/logo-dark.svg" />
    <img src="README.assets/logo.svg" alt="Spearhook" width="96" height="96" title="Spearhook" />
  </picture>
</p>

<h1 align="center">Spearhook</h1>

<p align="center">
  <strong>專為授權紅隊設計的自架釣魚演練控制台</strong>
</p>

<p align="center">
  Go + Gin · Next.js · SQLite · 單一執行檔 · 可選遠端 Agent
</p>

<p align="center">
  <a href="https://spearhook.pages.dev/">線上示範</a> ·
  <a href="#法律聲明">法律聲明</a> ·
  <a href="#專案簡介">專案簡介</a> ·
  <a href="#核心能力">核心能力</a> ·
  <a href="#架構">架構</a> ·
  <a href="#快速開始">快速開始</a> ·
  <a href="#agents">遠端 Agent</a> ·
  <a href="#控制台">控制台</a> ·
  <a href="#免責聲明">免責聲明</a>
</p>

---

## 法律聲明

Spearhook 僅限用於獲得明確授權的安全測試、紅隊演練與釣魚意識培訓。

未經明確授權，對系統、賬號或人員使用本軟體屬於違法行為。授權的取得及軟體的使用方式由使用者自行負責，作者與貢獻者不對任何濫用行為承擔責任。

如無書面授權或其他可核驗的授權憑證，請勿使用 Spearhook。

---

## 專案簡介

**Spearhook** 是一個自架的釣魚演練控制台，涵蓋著陸頁搭建、郵件投遞、開啟與點選追蹤、憑證採集以及提交即時通知。頁面既可執行於控制平面，也可部署至遠端 Agent。

若熟悉 [Gophish](https://github.com/gophish/gophish)：其側重於活動與模板管理；Spearhook 則面向演練全流程，提供帶 AI 鏡像的頁面庫、AI 助手維運、多 Agent 部署、即時 QR 中繼、AI 公開資訊收集、Webhook、IP 黑名單、稽核與角色管理。

單一 Go 二進位內嵌前端（`frontend/dist`），在同一埠提供 API 與控制台。技術棧：Go 1.24 / Gin / GORM+SQLite / Next.js 靜態匯出 / 可選 Docker SDK。示範網址：[https://spearhook.pages.dev/](https://spearhook.pages.dev/)。

---

## 語言

控制台以英語為源語言，專案文件亦以英語為主；缺失翻譯時回退至英語。介面支援簡體中文、繁體中文、日語和韓語，可透過登入頁或控制台右上角切換，選擇按瀏覽器儲存。README 提供以上五種語言版本；內容不一致時，以英文版為準。

## 核心能力

- **AI 助手（AI Assistant）** — 自然語言維運助手（`/chat`），透過 SSE 逐 token 流式輸出。模型僅可呼叫白名單工具（`list_agents`、`clone_page`、`create_project`、`start_info_gather`、`create_mail_draft`、`send_mail_campaign`，以及 QR、憑證、黑名單等工具）。唯讀工具自動執行，破壞性操作需人工確認，所有寫操作均記入稽核日誌。助手不會直接生成 HTML；URL 鏡像由頁面庫管線處理。
- **頁面庫 / AI 鏡像** — 上傳 HTML，或將公開登入頁、著陸頁 URL 經 AI 改寫後存入頁面庫（支援 OpenAI 相容 `chat/completions` 或 Anthropic `/messages`）。相對路徑 CSS 與靜態資源將按來源網站轉換為絕對位址，確保預覽樣式完整。
- **專案** — 綁定頁面庫中的頁面，以 **Docker** 或**本機 Flask** 執行，支援 TLS、一個或多個 Agent 及 QR 佔位符。提交、日誌與部署資訊按專案彙總。
- **郵件** — 支援多套 SMTP、模板、定時與群發；每個活動獨立追蹤開啟和點選（`/api/<slug>`）。可向 HTML 注入雜訊內容。追蹤公網地址與 HMAC 在「系統 → 郵件追蹤」中設定。
- **推送通道** — 支援飛書、企業微信、Telegram、Slack、釘釘與 Discord。將 Webhook 綁定至專案，憑證提交時觸發通知；開啟或點選不會觸發。
- **QR 釣魚** — 透過穩定地址 `/q/{slug}.png` 提供即時二維碼。本機指令碼上傳螢幕幀，頁面使用 `{{QR_RELAY_URL}}` / `{{QR_RELAY_IMG}}`。
- **資訊收集** — 接入自有 AI 介面，並在「系統 → 搜尋設定」中選擇網路搜尋提供者（原生 `web_search`、DeepSeek Search、Brave、Tavily、Serper 或博查）。蒐集公開電子郵件地址、電話號碼及來源，匯出 CSV 後用於郵件投遞。
- **Agent** — 在其他主機上執行 `spearhook-agent`。單一專案可部署至多個節點，單一節點也可承載多個專案。支援心跳；存取日誌寫入 `pending-logs.json`，待控制平面恢復後補傳。憑證提交即時轉發至控制平面；控制平面不可達時，訪客會收到 502（不提供提交佇列）。
- **認證與角色** — JWT 登入；可選 HTTP Basic 外層驗證；**管理員 / 操作員**角色與歸屬隔離；稽核日誌僅可附加，產品內不可刪除。
- **安全加固與維運** — 儀表盤與主機 CPU/記憶體監控、金鑰與密碼靜態加密、QQWry 定位、CSV 匯出、**IP 黑名單**（批次提交來源自動封鎖），以及多語言介面。AI 設定支援 OpenAI 相容與 Anthropic Messages，供頁面鏡像、AI 助手和資訊收集共用。

**標準流程：** 設定 SMTP 與 Webhook → 鏡像或上傳頁面 → 建立專案（綁定 Agent 與推送通道）→ 傳送郵件 → 等待提交通知 → 檢視記錄。若出現大量提交，可將來源 IP 加入黑名單。

---

## 架構

```text
  瀏覽器（多語言 UI · JWT · 可選 Basic Auth）
           │
           ▼
  控制平面  (Go / Gin · SQLite · 內嵌 Next UI · AI 助手 SSE)
           │
     ┌─────┴──────────────┐
     ▼                    ▼
  Docker / 本機 Flask   遠端 spearhook-agent
     │                    │
     └─────────┬──────────┘
               ▼
  憑證提交 · 存取日誌 · 郵件開啟/點選 · 即時 QR · Webhook
```

| 元件 | 職責 |
|------|------|
| 控制平面（`spearhook`） | 控制台、API、SQLite、郵件追蹤端點、QR 公網地址、AI 助手編排 |
| Agent（`spearhook-agent`） | 邊緣著陸頁執行環境；主動連出至控制平面 |
| 前端 | Next.js App Router → 靜態 `frontend/dist`，透過 `go:embed` 內嵌 |
| AI 設定 | 同時僅啟用一個端點（OpenAI 相容或 Anthropic Messages），供頁面鏡像、AI 助手工具與資訊收集使用 |
| 搜尋設定 | 資訊收集使用的網路搜尋：原生 `web_search` 或 DeepSeek / Brave / Tavily / Serper / 博查 |

---

## 快速開始

支援 Linux、macOS 與 Windows。

| 需求 | 說明 |
|------|------|
| 執行發行包 | 二進位 + `config.yaml` |
| 從原始碼構建 | Go **1.24+**、Node.js（前端） |
| Docker 執行模式 | 可存取 Docker Engine；僅使用本機 Flask 或 Agent 時無需安裝 |
| 本機 Flask | `python3`，以及 `flask`、`flask_cors`、`requests` |
| QR 截圖 | Python 3 + **工作台 → QR 釣魚** 下的 zip（或 [`scripts/qr-relay`](scripts/qr-relay/README.zh.md)）；macOS 解碼需要 `zbar` |
| Agent | 需能透過 HTTP(S) 存取控制平面 |

| 用途 | 預設值 |
|------|--------|
| 控制台 / API | `:8000` |
| 前端開發伺服器 | `:8081`，將 `/api` 代理至 `:8000` |
| Agent 著陸頁埠 | 該主機 `20000–30000/TCP` |

### 1. 設定

```bash
cp config.yaml.example config.yaml
```

部署前請替換以下佔位金鑰：

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

加密金鑰可透過 `cmd/generate-encryption-key` 生成。郵件追蹤（公網地址、HMAC、跳轉白名單）在啟動後於「系統 → 郵件追蹤」中設定；YAML 僅作為初始值。開啟與點選路徑按活動生成。

請勿提交 `config.yaml`、`data/` 或私鑰檔案（已在 `.gitignore` 中忽略）。

### 2. 執行

```bash
./spearhook
# 或使用 ./build-all.sh 後生成的 ./release/ 發行包
```

啟動時若檢測到空金鑰或預設金鑰，會顯示警告。實驗室環境可確認繼續；生產環境請先更換金鑰。

### 3. 從原始碼構建

```bash
./build-all.sh
```

先構建前端，再生成控制平面與 Agent（Linux / Windows / darwin，amd64 與 arm64），並將兩個示例設定複製至 `release/`。

僅啟動前端（API 已執行）：

```bash
cd frontend && npm install && npm run dev   # http://127.0.0.1:8081
```

### 4. 登入

開啟 `http://localhost:8000`（或已設定的埠）。首次啟動時，管理員密碼寫入：

```bash
cat data/admin_password.txt
```

請妥善儲存後刪除該檔案。後續可透過 `cmd/reset-password` 重設密碼。

---

<a id="agents"></a>

## 遠端 Agent

若不希望所有著陸頁都執行於控制平面，可將 **spearhook-agent** 部署到其他主機。單一專案可覆蓋多個節點，單一節點也可承載多個專案。每個綁定擁有獨立埠、版本與日誌。憑證提交會即時轉發至控制平面；存取日誌寫入 `pending-logs.json`，待控制平面恢復後補傳。

專案埠預設 **`20000–30000/TCP`**。Agent 主動連出至控制平面；存取者直接存取 Agent（若位於反向代理後，請設定 `advertise_host`，詳見完整手冊）。

**權杖** — 在控制平面單獨設定 `security.agent_registration_token`（不要複用 JWT 金鑰），然後重啟。首次註冊使用該共享權杖；之後 `{work_dir}/state.json` 儲存該 Agent 的 ID 與權杖（Linux/macOS 許可權為 `0600`）。

**啟動** — 執行 `./build-all.sh` 後：

```bash
cp agent.yaml.example agent.yaml
```

```yaml
server_url: http://control-plane.example.com:8000   # 末尾不要加斜槓
registration_token: replace-with-a-long-random-agent-token
```

```bash
chmod +x ./spearhook-agent
./spearhook-agent -config ./agent.yaml
```

```powershell
.\spearhook-agent-windows-amd64.exe -config .\agent.yaml
```

日誌出現 `agent … connected to http://…` 即表示連線成功。

**控制台操作** — 在「系統 → Agent」等待狀態變為線上（約 90 秒無心跳則視為離線）。在「專案 → 新增」中選擇 Agent 並綁定頁面。每個 Agent 為獨立執行個體（`pending` → `deploying` → `running`）。修改專案會產生新版本；新增 Agent 會觸發部署；取消選擇會下達刪除。

systemd、Windows 服務與反向代理設定：**[docs/agent-deployment.md](docs/agent-deployment.md)**。

---

## 控制台

| 區域 | 功能 |
|------|------|
| **儀表盤** | 計數與主機 CPU/記憶體 |
| **AI 助手** | 透過自然語言呼叫平台工具（鏡像、建立專案、列出 Agent 等） |
| **專案** | 概覽、部署、日誌、憑證記錄 |
| **工作台 → 頁面製作** | 上傳 HTML，或透過 AI 鏡像 URL 入庫 |
| **工作台 → 傳送郵件** | 模板、活動、開啟與點選 |
| **工作台 → 資訊收集** | AI 網路搜尋 → 聯絡人 CSV |
| **工作台 → QR 釣魚** | 即時二維碼中繼與截圖指令碼 zip |
| **推送通道** | 憑證提交時觸發 Webhook |
| **Agent** | 線上狀態與最後心跳 |
| **IP 黑名單** | 將批次提交來源 IP 加入黑名單（從憑證記錄新增） |
| **郵件服務** | SMTP 設定 |
| **使用者 / AI 設定 / 郵件追蹤** | 管理員 |
| **稽核日誌** | 管理員；產品內不可刪除 |

頁面預設提交至 `/api/submit`。HTML 佔位符：`{{SUBMIT_URL}}`、`{{REDIRECT_URL}}`、`{{QR_RELAY_URL}}`、`{{QR_RELAY_IMG}}` 等。郵件佔位符：`{{email}}`、`{{click_url}}`、`{{open_pixel}}`、`{{landing_url}}` 等。

說明：[中文](docs/placeholders.zh.md) · [英文](docs/placeholders.en.md)

存取日誌：`data/access-YYYY-MM-DD.log`。儀表盤的高頻輪詢不會寫入控制台事件流，但仍會記錄到日誌檔案。

---

## 免責聲明

Spearhook 僅用於獲得授權的紅隊釣魚演練與安全意識培訓。對未獲授權的人員或系統使用本軟體，風險與責任由使用者自行承擔；本專案不支援此類用途。

程式碼採用 [MIT License](LICENSE)。

歡迎提交 Issue 與 PR；如遇問題，請透過 GitHub Issue 反饋。
