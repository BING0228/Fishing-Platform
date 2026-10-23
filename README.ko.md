<p align="right">
  <a href="README.md">English</a> · <a href="README.zh.md">简体中文</a> · <a href="README.zh-TW.md">繁體中文</a> · <a href="README.ja.md">日本語</a> · <b>한국어</b>
</p>


<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/readme/logo-dark.svg" />
    <p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/readme/logo-dark.svg" />
    <img src="assets/readme/logo.svg" alt="Spearhook" width="96" height="96" title="Spearhook" />
  </picture>
</p>
  </picture>
</p>

# Spearhook

**승인된 레드팀을 위한 자체 호스팅 피싱 시뮬레이션 콘솔**

Go + Gin · Next.js · SQLite · 단일 바이너리 · 선택적 원격 에이전트

[데모](https://spearhook.pages.dev/) · [법적 고지](#legal-warning) · [개요](#what-it-is) · [기능](#features) · [아키텍처](#architecture) · [빠른 시작](#quick-start) · [에이전트](#agents) · [콘솔](#console) · [면책 조항](#disclaimer)

---

<a id="legal-warning"></a>

## 법적 고지

Spearhook은 **승인된** 보안 테스트, 레드팀 훈련, 피싱 인식 제고 활동**만**을 위해 만들어졌습니다.

테스트에 대한 명시적 허가가 없는 시스템, 계정 또는 사람을 대상으로 사용하는 것은 불법입니다. 승인을 받고 이 소프트웨어를 운영하는 책임은 전적으로 사용자에게 있습니다. 작성자와 기여자는 오용에 대해 어떠한 책임도 지지 않습니다.

서면(또는 이에 준하는 명확한 문서) 허가가 없다면 Spearhook을 사용하지 마십시오.

---

<a id="what-it-is"></a>

## 개요

**Spearhook**은 승인된 레드팀을 위한 자체 호스팅 피싱 **시뮬레이션 콘솔**입니다. 랜딩 페이지 게시, 메일 발송, 열람·클릭 추적, 자격 증명 수집, 제출 시 알림을 한곳에서 처리합니다. 페이지는 컨트롤 플레인에서 실행하거나 원격 에이전트로 배포할 수 있습니다.

Gophish를 알고 있다면: 해당 툴킷은 캠페인과 템플릿 관리에 중점을 둡니다. Spearhook은 훈련 전반을 지원하는 운영 콘솔로, AI 미러가 있는 페이지 라이브러리, AI 어시스턴트 기반 운영, 멀티 에이전트 배포, 라이브 QR 릴레이, AI OSINT 수집, Webhook, IP 블랙리스트, 감사, 역할 관리를 통합합니다.

Go 바이너리 하나에 UI(`frontend/dist`)를 내장하고 같은 포트에서 API와 콘솔을 제공합니다. 스택: Go 1.24 / Gin / GORM+SQLite / Next.js 정적 내보내기 / 선택적 Docker SDK. 데모: [https://spearhook.pages.dev/](https://spearhook.pages.dev/).


---

<a id="features"></a>

## 기능

- **AI 어시스턴트** — 자연어 운영 어시스턴트(`/chat`). 화이트리스트 도구 레지스트리를 통한 토큰 단위 SSE 스트리밍(`list_agents`, `clone_page`, `create_project`, `start_info_gather`, `create_mail_draft`, `send_mail_campaign`, QR/자격 증명/블랙리스트 등). 읽기 도구는 자동 실행되고, 파괴적 작업은 확인이 필요하며, 모든 쓰기 작업은 감사 로그에 기록됩니다. 어시스턴트는 HTML을 직접 생성하지 않으며, URL 미러링은 페이지 라이브러리 파이프라인에서 처리됩니다.
- **페이지 라이브러리 / AI 미러** — HTML을 업로드하거나 공개 로그인/랜딩 URL을 AI 재작성으로 라이브러리에 미러링합니다(OpenAI 호환 `chat/completions` 또는 Anthropic `/messages`). 상대 경로 CSS/자산은 원본 오리진 기준 절대 URL로 변환되어 미리보기 스타일이 유지됩니다.
- **프로젝트** — 라이브러리 페이지를 바인딩하고 **Docker** 또는 **로컬 Flask**, 선택적 TLS, 하나 이상의 에이전트, QR 릴레이 플레이스홀더로 실행합니다. 자격 증명 제출, 로그, 배포는 프로젝트별로 집계됩니다.
- **메일** — SMTP 프로필, 템플릿, 예약/대량 발송, 캠페인별 열람·클릭 추적(`/api/<slug>`). 선택적으로 HTML 노이즈를 추가할 수 있습니다. 추적용 공개 URL과 HMAC은 시스템 → 메일 추적에서 설정합니다.
- **푸시 채널** — Feishu, WeCom, Telegram, Slack, DingTalk, Discord. Webhook을 프로젝트에 바인딩하고 자격 증명 제출 시 알림을 보냅니다. 열람·클릭으로는 알림을 보내지 않습니다.
- **QR 피싱** — `/q/{slug}.png`에서 안정적인 공개 이미지를 제공합니다. 로컬 캡처 스크립트가 화면 프레임을 업로드하고, 페이지는 `{{QR_RELAY_URL}}` / `{{QR_RELAY_IMG}}`를 사용합니다.
- **정보 수집** — AI 엔드포인트와 설정 가능한 웹 검색 제공자(네이티브 `web_search`, DeepSeek Search, Brave, Tavily, Serper, Bocha)를 사용합니다. 공개된 이메일/전화번호를 출처와 함께 수집하고 CSV로 내보내 메일 발송에 전달할 수 있습니다.
- **에이전트** — 다른 호스트에서 실행되는 `spearhook-agent`. 에이전트 하나로 여러 프로젝트를, 프로젝트 하나를 여러 에이전트에 배포할 수 있습니다. 하트비트, `pending-logs.json` 액세스 로그 버퍼링, 복구 후 재전송을 지원합니다. 자격 증명 제출은 컨트롤 플레인으로 실시간 중계되며, 연결할 수 없으면 방문자에게 502를 반환합니다(제출 대기열은 없습니다).
- **인증과 역할** — JWT 로그인, 선택적 HTTP Basic 외부 인증, **관리자** / **운영자** 소유권 범위, 추가 전용 감사 로그(제품 내 삭제 불가).
- **보안 강화와 운영** — 대시보드와 호스트 CPU/메모리, 저장 시 시크릿/비밀번호 암호화, QQWry 지리 위치, CSV 내보내기, **IP 블랙리스트**(대량 제출 IP 차단), 영어/중국어 간체/번체/일본어/한국어 UI. AI 설정은 OpenAI 호환과 Anthropic Messages를 지원하며 미러·AI 어시스턴트·정보 수집에서 공통 사용됩니다.

**표준 운영 흐름:** SMTP와 Webhook 설정 → 페이지 미러링 또는 업로드 → 프로젝트 생성(에이전트와 푸시 채널 바인딩) → 메일 발송 → 제출 알림 대기 → 기록 확인. 대량 제출이 발생하면 소스 IP를 블랙리스트에 추가합니다.


---

<a id="architecture"></a>

## 아키텍처

```text
  Browser (i18n UI · JWT · optional Basic Auth)
           │
           ▼
  Control plane  (Go / Gin · SQLite · embedded Next UI · Assistant SSE)
           │
     ┌─────┴──────────────┐
     ▼                    ▼
  Docker / 로컬 Flask   Remote spearhook-agent(s)
     │                    │
     └─────────┬──────────┘
               ▼
  submits · access logs · mail open/click · live QR · webhooks
```

| 구성 요소 | 역할 |
|------|------|
| 컨트롤 플레인(`spearhook`) | 콘솔, API, SQLite, 메일 추적 엔드포인트, QR 공개 URL, AI 어시스턴트 오케스트레이션 |
| 에이전트(`spearhook-agent`) | 엣지 랜딩 페이지 런타임. 컨트롤 플레인으로 **아웃바운드** 연결합니다 |
| 프론트엔드 | Next.js App Router → 정적 `frontend/dist`, `go:embed`로 내장 |
| AI 프로필 | 페이지 미러, 어시스턴트 도구, 정보 수집에 사용하는 하나의 활성 OpenAI 호환 / Anthropic Messages 엔드포인트 |
| 검색 제공자 | 정보 수집용 웹 검색: 네이티브 `web_search` 또는 DeepSeek / Brave / Tavily / Serper / Bocha |


---

<a id="quick-start"></a>

## 빠른 시작

Linux, macOS, Windows를 지원합니다.

| 필요한 것 | 참고 |
|----|----|
| 릴리스 실행 | 바이너리 + `config.yaml` |
| 소스에서 빌드 | Go **1.24+**, Node.js(프런트엔드) |
| Docker 실행 모드 | Docker 엔진 접근 가능. 로컬 Flask/에이전트만 사용하면 불필요 |
| 로컬 Flask 실행 모드 | `python3` + `flask`, `flask_cors`, `requests` |
| QR 캡처 | Python 3 + **워크벤치 → QR 피싱**의 zip(또는 [`scripts/qr-relay`](scripts/qr-relay/README.md)). macOS 디코딩에는 `zbar` 필요 |
| 에이전트 | HTTP(S)로 컨트롤 플레인에 연결할 수 있어야 함 |

| 항목 | 기본값 |
|------|--------|
| 콘솔 / API | `:8000` |
| 프론트엔드 개발 서버 | `:8081` → `/api`를 `:8000`으로 프록시 |
| 에이전트 랜딩 포트 | 에이전트 호스트의 `20000–30000/TCP` |

### 1. 설정

``` bash
cp config.yaml.example config.yaml
```

실제 훈련 전에 플레이스홀더를 교체합니다.

``` yaml
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

`cmd/generate-encryption-key`로 `encryption_key`를 생성할 수 있습니다. 메일 추적(공용 URL, HMAC, 리디렉션 허용 목록)은 부팅 후 **시스템 → 메일 추적**에서 구성됩니다. YAML은 초기값/대체값입니다. 클릭/열기 경로는 캠페인별로 생성됩니다.

`config.yaml`, `data/` 또는 TLS 키를 커밋하지 마세요(gitignored).

### 2. 실행

``` bash
./spearhook
# or a build from ./release/ after ./scripts/build-all.sh
```

비어 있거나 기본값인 시크릿은 시작 시 경고를 표시합니다. 랩 환경에서는 괜찮지만 실제 작업 전에 교체하십시오.

### 3. 빌드

``` bash
./scripts/build-all.sh
```

프런트엔드를 빌드한 다음 linux/windows/darwin(amd64 및 arm64)용 컨트롤 플레인 + 에이전트를 빌드하고 `config.yaml.example` + `agent.yaml.example`을 `release/`에 복사합니다.

UI 전용(API이(가) 이미 작동 중):

``` bash
cd frontend && npm install && npm run dev   # http://127.0.0.1:8081
```

### 4. 로그인

`http://localhost:8000`(또는 `server.port`)을 엽니다. 처음 부팅하면 다음 위치에 관리자 자격 증명이 기록됩니다.

``` bash
cat data/admin_password.txt
```

비밀번호를 저장한 후 파일을 삭제하세요. 나중에 재설정: `cmd/reset-password`.

---

<a id="agents"></a>

## 에이전트

모든 랜딩 페이지를 컨트롤 플레인에서 실행하고 싶지 않다면 다른 호스트에 **spearhook-agent**를 배포할 수 있습니다. 프로젝트 하나를 여러 노드로 배포하거나, 노드 하나에서 여러 프로젝트를 호스팅할 수 있습니다. 각 바인딩은 독립적인 포트, 리비전, 로그를 가집니다. 자격 증명 제출은 컨트롤 플레인으로 실시간 중계됩니다. 액세스 로그는 `pending-logs.json`에 버퍼링되었다가 복구 후 재전송됩니다.

프로젝트 포트의 기본값은 **`20000–30000/TCP`**입니다. 에이전트는 컨트롤 플레인으로 **아웃바운드** 연결합니다. 방문자는 에이전트에 접속합니다(리버스 프록시 뒤에서는 `advertise_host` 설정, 자세한 내용은 가이드 참조).

**토큰** — 컨트롤 플레인에서 `security.agent_registration_token`(JWT 시크릿 아님)을 설정하고 재시작합니다. 첫 등록 시 공유 토큰을 한 번 사용하며, 이후 `{work_dir}/state.json`에 에이전트별 ID/토큰을 보관합니다(Linux/macOS는 `0600`).

**시작** — `./scripts/build-all.sh` 후:

``` bash
cp agent.yaml.example agent.yaml
```

``` yaml
server_url: http://control-plane.example.com:8000   # no trailing slash
registration_token: replace-with-a-long-random-agent-token
```

``` bash
chmod +x ./spearhook-agent
./spearhook-agent -config ./agent.yaml
```

``` powershell
.\spearhook-agent-windows-amd64.exe -config .\agent.yaml
```

`agent … connected to http://…` 같은 로그가 보이면 성공입니다.

**UI** — **시스템 → 에이전트**에서 온라인이 될 때까지 기다립니다(약 90초 동안 하트비트가 없으면 오프라인으로 간주). **프로젝트 → 새 프로젝트**에서 에이전트를 선택하고 페이지를 첨부합니다. 에이전트별로 독립 인스턴스(`pending` → `deploying` → `running`)가 생성됩니다. 프로젝트를 편집하면 새 리비전이 생성되고, 에이전트를 추가하면 배포가 실행되며, 선택을 해제하면 삭제 작업이 전송됩니다.

systemd, Windows 서비스, 리버스 프록시: **[docs/agent-deployment.md](docs/agent-deployment.md)**.

---

<a id="console"></a>

## 콘솔

| 영역 | 내용 |
|----|----|
| **대시보드** | 개수와 호스트 CPU/메모리 |
| **AI 어시스턴트** | 플랫폼 도구를 통한 자연어 작업(복제, 프로젝트 생성, 에이전트 목록 등) |
| **프로젝트** | 개요, 배포, 로그, 자격 증명 기록 |
| **워크벤치 → 페이지 빌더** | HTML 또는 AI 미러링한 URL을 페이지 라이브러리에 저장 |
| **워크벤치 → 메일 보내기** | 템플릿, 캠페인, 열람/클릭 |
| **워크벤치 → 정보 수집** | AI 웹 검색 → 연락처 CSV |
| **워크벤치 → QR 피싱** | 라이브 QR 릴레이 + 캡처 스크립트 zip |
| **푸시 채널** | 자격 증명 제출 시 Webhook |
| **에이전트** | 온라인 상태 / 마지막 확인 |
| **IP 블랙리스트** | 자격 증명 기록에서 대량 제출 IP 차단 |
| **메일 서비스** | SMTP 프로필 |
| **사용자 / AI 설정 / 메일 추적** | 관리자 |
| **감사 로그** | 관리자 전용. 제품 내에서 삭제 불가 |

페이지는 기본적으로 `/api/submit`에 POST됩니다. HTML 플레이스홀더: `{{SUBMIT_URL}}`, `{{REDIRECT_URL}}`, `{{QR_RELAY_URL}}`, `{{QR_RELAY_IMG}}` 등. 메일: `{{email}}`, `{{click_url}}`, `{{open_pixel}}`, `{{landing_url}}` 등.

가이드: [EN](docs/placeholders.en.md) · [ZH](docs/placeholders.zh.md) · [ZH-TW](docs/placeholders.zh-TW.md) · [JA](docs/placeholders.ja.md) · [KO](docs/placeholders.ko.md)

액세스 로그: `data/access-YYYY-MM-DD.log`. 빈도가 높은 대시보드 폴링은 콘솔 스트림에서 제외되지만 파일에는 기록됩니다.

---

<a id="disclaimer"></a>

## 면책 조항

Spearhook은 승인된 레드팀 피싱 훈련 및 보안 인식 교육에만 사용해야 합니다. 권한이 없는 사람이나 시스템에 사용할 경우 발생하는 위험과 책임은 전적으로 사용자에게 있으며, 본 프로젝트는 이러한 사용을 지원하지 않습니다.

코드는 [MIT License](LICENSE)에 따라 제공됩니다.

이슈와 PR을 환영합니다. 문제가 발생하면 GitHub 이슈로 알려 주십시오.
