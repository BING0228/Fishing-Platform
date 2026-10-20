<p align="right">
  <a href="README.md">English</a> · <b>简体中文</b> · <a href="README.zh-TW.md">繁體中文</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="README.assets/logo-dark.svg" />
    <img src="README.assets/logo.svg" alt="Spearhook" width="96" height="96" title="Spearhook" />
  </picture>
</p>

<h1 align="center">Spearhook</h1>

<p align="center">
  <strong>专为授权红队设计的自托管钓鱼演练控制台</strong>
</p>

<p align="center">
  Go + Gin · Next.js · SQLite · 单一二进制 · 可选远程 Agent
</p>

<p align="center">
  <a href="https://spearhook.pages.dev/">在线演示</a> ·
  <a href="#法律声明">法律声明</a> ·
  <a href="#项目简介">项目简介</a> ·
  <a href="#核心能力">核心能力</a> ·
  <a href="#架构">架构</a> ·
  <a href="#快速开始">快速开始</a> ·
  <a href="#agents">远程 Agent</a> ·
  <a href="#控制台">控制台</a> ·
  <a href="#免责声明">免责声明</a>
</p>

---

## 法律声明

Spearhook 仅限用于获得明确授权的安全测试、红队演练与钓鱼意识培训。

未经明确授权，对系统、账号或人员使用本软件属于违法行为。授权的取得及软件的使用方式由使用者自行负责，作者与贡献者不对任何滥用行为承担责任。

如无书面授权或其他可核验的授权凭据，请勿使用 Spearhook。

---

## 项目简介

**Spearhook** 是一个自托管的钓鱼演练控制台，涵盖落地页搭建、邮件投递、打开与点击追踪、凭据采集以及提交实时通知。页面既可运行于控制面，也可下发至远程 Agent。

若熟悉 [Gophish](https://github.com/gophish/gophish)：其侧重于活动与模板管理；Spearhook 则面向演练全流程，提供带 AI 镜像的页面库、AI 助手运维、多 Agent 部署、实时 QR 中继、AI 公开信息搜集、Webhook、IP 黑名单、审计与角色管理。

单一 Go 二进制内嵌前端（`frontend/dist`），在同一端口提供 API 与控制台。技术栈：Go 1.24 / Gin / GORM+SQLite / Next.js 静态导出 / 可选 Docker SDK。使用生产前端与 mock 数据构建的只读演示站：[在线演示](https://spearhook.pages.dev/)。演示账号：`demo` / `demo`（任意账号密码均可登录）。

---

## 语言

控制台以英语为源语言，项目文档亦以英语为主；缺失翻译时回退至英语。界面支持简体中文、繁体中文、日语和韩语，可通过登录页或控制台右上角切换，选择按浏览器保存。README 提供以上五种语言版本；内容不一致时，以英文版为准。

## 核心能力

- **AI 助手（AI Assistant）** — 自然语言运维助手（`/chat`），通过 SSE 逐 token 流式输出。模型仅可调用白名单工具（`list_agents`、`clone_page`、`create_project`、`start_info_gather`、`create_mail_draft`、`send_mail_campaign`，以及 QR、凭据、黑名单等工具）。只读工具自动执行，破坏性操作需人工确认，所有写操作均记入审计日志。助手不会直接生成 HTML；URL 镜像由页面库流水线处理。
- **页面库 / AI 镜像** — 上传 HTML，或将公开登录页、落地页 URL 经 AI 改写后存入页面库（支持 OpenAI 兼容 `chat/completions` 或 Anthropic `/messages`）。相对路径 CSS 与静态资源将按源站转换为绝对地址，确保预览样式完整。
- **项目** — 绑定页面库中的页面，以 **Docker** 或**本机 Flask** 运行，支持 TLS、一个或多个 Agent 及 QR 占位符。提交、日志与部署信息按项目汇总。
- **邮件** — 支持多套 SMTP、模板、定时与群发；每个活动独立追踪打开和点击（`/api/<slug>`）。可向 HTML 注入噪声内容。追踪公网地址与 HMAC 在「系统 → 邮件追踪」中配置。
- **推送通道** — 支持飞书、企业微信、Telegram、Slack、钉钉与 Discord。将 Webhook 绑定至项目，凭据提交时触发通知；打开或点击不会触发。
- **QR 钓鱼** — 通过稳定地址 `/q/{slug}.png` 提供实时二维码。本机脚本上传屏幕帧，页面使用 `{{QR_RELAY_URL}}` / `{{QR_RELAY_IMG}}`。
- **信息搜集** — 接入自有 AI 接口，并在「系统 → 搜索配置」中选择联网搜索提供方（原生 `web_search`、DeepSeek Search、Brave、Tavily、Serper 或博查）。收集公开邮箱地址、电话号码及来源，导出 CSV 后用于邮件投递。
- **Agent** — 在其他主机上运行 `spearhook-agent`。单个项目可部署至多个节点，单个节点也可承载多个项目。支持心跳；访问日志写入 `pending-logs.json`，待控制面恢复后补传。凭据提交实时转发至控制面；控制面不可达时，访客会收到 502（不提供提交队列）。
- **认证与角色** — JWT 登录；可选 HTTP Basic 外围认证；**管理员 / 操作员**角色与归属隔离；审计日志仅追加，产品内不可删除。
- **安全加固与运维** — 仪表盘与主机 CPU/内存监控、密钥与密码落盘加密、QQWry 定位、CSV 导出、**IP 黑名单**（批量提交来源自动封禁），以及多语言界面。AI 设置支持 OpenAI 兼容与 Anthropic Messages，供页面镜像、AI 助手和信息搜集共用。

**标准流程：** 配置 SMTP 与 Webhook → 镜像或上传页面 → 创建项目（绑定 Agent 与推送通道）→ 发送邮件 → 等待提交通知 → 查看记录。若出现大量提交，可将来源 IP 加入黑名单。

---

## 架构

```text
  浏览器（多语言 UI · JWT · 可选 Basic Auth）
           │
           ▼
  控制面  (Go / Gin · SQLite · 内嵌 Next UI · AI 助手 SSE)
           │
     ┌─────┴──────────────┐
     ▼                    ▼
  Docker / 本机 Flask   远程 spearhook-agent
     │                    │
     └─────────┬──────────┘
               ▼
  凭据提交 · 访问日志 · 邮件打开/点击 · 实时 QR · Webhook
```

| 组件 | 职责 |
|------|------|
| 控制面（`spearhook`） | 控制台、API、SQLite、邮件追踪端点、QR 公网地址、AI 助手编排 |
| Agent（`spearhook-agent`） | 边缘落地页运行时；主动连出至控制面 |
| 前端 | Next.js App Router → 静态 `frontend/dist`，通过 `go:embed` 内嵌 |
| AI 配置 | 同时仅启用一个端点（OpenAI 兼容或 Anthropic Messages），供页面镜像、AI 助手工具与信息搜集使用 |
| 搜索配置 | 信息搜集使用的联网搜索：原生 `web_search` 或 DeepSeek / Brave / Tavily / Serper / 博查 |

---

## 快速开始

支持 Linux、macOS 与 Windows。

| 需求 | 说明 |
|------|------|
| 运行发行包 | 二进制 + `config.yaml` |
| 从源码构建 | Go **1.24+**、Node.js（前端） |
| Docker 运行模式 | 可访问 Docker Engine；仅使用本机 Flask 或 Agent 时无需安装 |
| 本机 Flask | `python3`，以及 `flask`、`flask_cors`、`requests` |
| QR 截屏 | Python 3 + **工作台 → QR 钓鱼** 下的 zip（或 [`scripts/qr-relay`](scripts/qr-relay/README.zh.md)）；macOS 解码需要 `zbar` |
| Agent | 需能通过 HTTP(S) 访问控制面 |

| 用途 | 默认值 |
|------|--------|
| 控制台 / API | `:8000` |
| 前端开发服务器 | `:8081`，将 `/api` 代理至 `:8000` |
| Agent 落地页端口 | 该主机 `20000–30000/TCP` |

### 1. 配置

```bash
cp config.yaml.example config.yaml
```

部署前请替换以下占位密钥：

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

加密密钥可通过 `cmd/generate-encryption-key` 生成。邮件追踪（公网地址、HMAC、跳转白名单）在启动后于「系统 → 邮件追踪」中配置；YAML 仅作为初始值。打开与点击路径按活动生成。

请勿提交 `config.yaml`、`data/` 或私钥文件（已在 `.gitignore` 中忽略）。

### 2. 运行

```bash
./spearhook
# 或使用 ./build-all.sh 后生成的 ./release/ 发行包
```

启动时若密钥为空或仍为默认值，将显示警告。实验室环境可确认继续；生产环境请先更换密钥。

### 3. 从源码构建

```bash
./build-all.sh
```

先构建前端，再生成控制面与 Agent（Linux / Windows / darwin，amd64 与 arm64），并将两个示例配置复制至 `release/`。

仅启动前端（API 已运行）：

```bash
cd frontend && npm install && npm run dev   # http://127.0.0.1:8081
```

### 4. 登录

打开 `http://localhost:8000`（或已配置的端口）。首次启动时，管理员密码写入：

```bash
cat data/admin_password.txt
```

请妥善保存后删除该文件。后续可通过 `cmd/reset-password` 重置密码。

---

<a id="agents"></a>

## 远程 Agent

若不希望所有落地页都运行在控制面，可将 **spearhook-agent** 部署到其他主机。单个项目可覆盖多个节点，单个节点也可承载多个项目。每个绑定拥有独立端口、版本与日志。凭据提交会实时代理至控制面；访问日志写入 `pending-logs.json`，待控制面恢复后补传。

项目端口默认 **`20000–30000/TCP`**。Agent 主动连出至控制面；访问者直接访问 Agent（若位于反向代理后，请配置 `advertise_host`，详见完整手册）。

**令牌** — 在控制面单独设置 `security.agent_registration_token`（不要复用 JWT 密钥），然后重启。首次注册使用该共享令牌；之后 `{work_dir}/state.json` 保存该 Agent 的 ID 与令牌（Linux/macOS 权限为 `0600`）。

**启动** — 执行 `./build-all.sh` 后：

```bash
cp agent.yaml.example agent.yaml
```

```yaml
server_url: http://control-plane.example.com:8000   # 末尾不要加斜杠
registration_token: replace-with-a-long-random-agent-token
```

```bash
chmod +x ./spearhook-agent
./spearhook-agent -config ./agent.yaml
```

```powershell
.\spearhook-agent-windows-amd64.exe -config .\agent.yaml
```

日志出现 `agent … connected to http://…` 即表示连接成功。

**控制台操作** — 在「系统 → Agent」等待状态变为在线（约 90 秒无心跳则视为离线）。在「项目 → 新建」中选择 Agent 并绑定页面。每个 Agent 为独立实例（`pending` → `deploying` → `running`）。修改项目会生成新版本；添加 Agent 会触发部署；取消选择会下发删除。

systemd、Windows 服务与反向代理配置：**[docs/agent-deployment.md](docs/agent-deployment.md)**。

---

## 控制台

| 区域 | 功能 |
|------|------|
| **仪表盘** | 计数与主机 CPU/内存 |
| **AI 助手** | 通过自然语言调用平台工具（镜像、创建项目、列出 Agent 等） |
| **项目** | 概览、部署、日志、凭据记录 |
| **工作台 → 页面制作** | 上传 HTML，或通过 AI 镜像 URL 入库 |
| **工作台 → 发送邮件** | 模板、活动、打开与点击 |
| **工作台 → 信息搜集** | AI 联网搜索 → 联系人 CSV |
| **工作台 → QR 钓鱼** | 实时二维码中继与截屏脚本 zip |
| **推送通道** | 凭据提交时触发 Webhook |
| **Agent** | 在线状态与最后心跳 |
| **IP 黑名单** | 将批量提交来源 IP 加入黑名单（从凭据记录添加） |
| **邮件服务** | SMTP 配置 |
| **用户 / AI 设置 / 邮件追踪** | 管理员 |
| **审计日志** | 管理员；产品内不可删除 |

页面默认提交至 `/api/submit`。HTML 占位符：`{{SUBMIT_URL}}`、`{{REDIRECT_URL}}`、`{{QR_RELAY_URL}}`、`{{QR_RELAY_IMG}}` 等。邮件占位符：`{{email}}`、`{{click_url}}`、`{{open_pixel}}`、`{{landing_url}}` 等。

说明：[中文](docs/placeholders.zh.md) · [英文](docs/placeholders.en.md)

访问日志：`data/access-YYYY-MM-DD.log`。仪表盘的高频轮询不会写入控制台事件流，但仍会记录到日志文件。

---

## 免责声明

Spearhook 仅用于获得授权的红队钓鱼演练与安全意识培训。对未获授权的人员或系统使用本软件，风险与责任由使用者自行承担；本项目不支持此类用途。

代码采用 [MIT License](LICENSE)。

欢迎提交 Issue 与 PR；如遇问题，请通过 GitHub Issue 反馈。
