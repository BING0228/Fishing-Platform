<p align="right">
  <a href="README.md">English</a> · <a href="README.zh.md">简体中文</a> · <a href="README.zh-TW.md">繁體中文</a> · <b>日本語</b> · <a href="README.ko.md">한국어</a>
</p>


<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="README.assets/logo-dark.svg" />
    <p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="README.assets/logo-dark.svg" />
    <img src="README.assets/logo.svg" alt="Spearhook" width="96" height="96" title="Spearhook" />
  </picture>
</p>
  </picture>
</p>

# Spearhook

**許可されたレッドチーム向けのセルフホスト型フィッシング演習コンソール**

Go + Gin · Next.js · SQLite · 単一バイナリ · オプションのリモートエージェント

[デモ](https://spearhook.pages.dev/) · [法的通知](#legal-warning) · [概要](#what-it-is) · [機能](#features) · [アーキテクチャ](#architecture) · [クイックスタート](#quick-start) · [エージェント](#agents) · [コンソール](#console) · [免責事項](#disclaimer)

---

<a id="legal-warning"></a>

## 法的通知

Spearhook は、**許可された**セキュリティテスト、レッドチーム演習、フィッシング啓発活動**専用**のツールです。

テストの明示的な許可を持たないシステム、アカウント、または人物に対する不正使用は違法です。許可の取得および本ソフトウェアの運用方法については、利用者自身が全責任を負います。著者および貢献者は、誤用について一切の責任を負いません。

書面（または同等に明確な文書）による許可がない場合は、Spearhook を使用しないでください。

---

<a id="what-it-is"></a>

## 概要

**Spearhook** は、許可されたレッドチーム向けのセルフホスト型フィッシング**演習コンソール**です。ランディングページの公開、メール送信、開封・クリックの追跡、認証情報の取得、送信時の通知を一つのコンソールで行えます。ページはコントロールプレーン上でも、リモートエージェントでも実行できます。

Gophish をご存じの場合: 同ツールはキャンペーンとテンプレート管理に重点を置いています。Spearhook は演習全体を支援する運用コンソールであり、AI ミラーを備えたページライブラリ、AI アシスタントによる運用、マルチエージェント展開、ライブ QR リレー、AI OSINT 収集、Webhook、IP ブラックリスト、監査、ロール管理を統合しています。

Go バイナリ 1 つに UI（`frontend/dist`）を埋め込み、API とコンソールを同じポートで提供します。スタック: Go 1.24 / Gin / GORM+SQLite / Next.js 静的エクスポート / 任意の Docker SDK。本番フロントエンドとモックデータで構成した読み取り専用デモ：[デモ](https://spearhook.pages.dev/)。デモログイン: `demo` / `demo`（任意の認証情報でログインできます）。


---

<a id="languages"></a>

## 言語

コンソールのソース言語は英語で、ドキュメントの主要言語でもあり、翻訳が見つからない場合のフォールバックでもあります。Web コンソールは簡体字中国語、繁体字中国語、日本語、韓国語にも対応しています。ヘッダーまたはログイン画面の言語ピッカーで切り替えられ、選択はブラウザごとに保存されます。この README は 5 言語で維持されており、内容が異なる場合は英語版を正とします。

<a id="features"></a>

## 機能

- **AI アシスタント** — 自然言語の運用アシスタント（`/chat`）。ホワイトリスト化されたツールレジストリを介したトークン単位の SSE ストリーミング（`list_agents`、`clone_page`、`create_project`、`start_info_gather`、`create_mail_draft`、`send_mail_campaign`、QR/認証情報/ブラックリストなど）。読み取り系ツールは自動実行され、破壊的操作は確認を要し、すべての書き込みは監査ログに記録されます。アシスタントは HTML を直接生成しません。URL ミラーリングはページライブラリの処理系を経由します。
- **ページライブラリ / AI ミラー** — HTML をアップロードするか、公開ログイン/ランディング URL を AI による書き換えでライブラリにミラーリングします（OpenAI 互換 `chat/completions` または Anthropic `/messages`）。相対パス CSS/アセットは取得元オリジン基準の絶対 URL に変換されるため、プレビューのスタイルが保たれます。
- **プロジェクト** — ライブラリのページをバインドし、**Docker** または**ローカル Flask**、任意の TLS、1 つ以上のエージェント、QR リレーのプレースホルダーで実行します。認証情報の送信、ログ、デプロイはプロジェクト単位で集約されます。
- **メール** — SMTP プロファイル、テンプレート、予約/一括送信、キャンペーンごとの開封・クリック追跡（`/api/<slug>`）。任意で HTML ノイズを追加できます。追跡用公開 URL と HMAC は「システム → メール追跡」で設定します。
- **プッシュチャネル** — Feishu、WeCom、Telegram、Slack、DingTalk、Discord。Webhook をプロジェクトにバインドし、認証情報の送信時に通知します。開封・クリックでは通知しません。
- **QR フィッシング** — `/q/{slug}.png` で安定した公開画像を提供します。ローカル撮影スクリプトが画面フレームをアップロードし、ページは `{{QR_RELAY_URL}}` / `{{QR_RELAY_IMG}}` を使用します。
- **情報収集** — AI エンドポイントと設定可能な Web 検索プロバイダー（ネイティブ `web_search`、DeepSeek Search、Brave、Tavily、Serper、Bocha）を利用します。公開されているメールアドレス/電話番号を出典付きで収集し、CSV に書き出してメール送信に利用できます。
- **エージェント** — 別ホストで動く `spearhook-agent`。1 つのエージェントで複数プロジェクト、1 つのプロジェクトを複数エージェントに配備できます。ハートビート、アクセスログの `pending-logs.json` バッファリング、復旧後の再送に対応します。認証情報の送信はコントロールプレーンへリアルタイムに中継され、到達不能時は訪問者に 502 を返します（送信キューはありません）。
- **認証とロール** — JWT ログイン、任意の HTTP Basic 外周、**管理者** / **オペレーター** の所有権スコープ、追記専用の監査ログ（製品内から削除不可）。
- **セキュリティ強化と運用** — ダッシュボードとホスト CPU/メモリ、保存時のシークレット/パスワード暗号化、QQWry 地理位置、CSV エクスポート、**IP ブラックリスト**（大量送信元をブロック）、英語/簡体字/繁体字中国語/日本語/韓国語 UI。AI 設定は OpenAI 互換と Anthropic Messages に対応し、ミラー・AI アシスタント・情報収集で共通利用されます。

**標準的な運用フロー:** SMTP と Webhook を設定 → ページをミラーリングまたはアップロード → プロジェクトを作成（エージェントとプッシュチャネルをバインド）→ メールを送信 → 提交通知を待機 → 記録を確認。大量送信が発生した場合は、送信元 IP をブラックリストに追加します。


---

<a id="architecture"></a>

## アーキテクチャ

```text
  Browser (i18n UI · JWT · optional Basic Auth)
           │
           ▼
  Control plane  (Go / Gin · SQLite · embedded Next UI · Assistant SSE)
           │
     ┌─────┴──────────────┐
     ▼                    ▼
  Docker / ローカル Flask   Remote spearhook-agent(s)
     │                    │
     └─────────┬──────────┘
               ▼
  submits · access logs · mail open/click · live QR · webhooks
```

| コンポーネント | 役割 |
|------|------|
| コントロールプレーン（`spearhook`） | コンソール、API、SQLite、メール追跡エンドポイント、QR 公開 URL、AI アシスタントのオーケストレーション |
| エージェント（`spearhook-agent`） | エッジのランディングページランタイム。コントロールプレーンへ**アウトバウンド**接続します |
| フロントエンド | Next.js App Router → 静的 `frontend/dist`、`go:embed` で埋め込み |
| AI プロファイル | ページミラー、アシスタントツール、情報収集で使う単一の有効な OpenAI 互換 / Anthropic Messages エンドポイント |
| 検索プロバイダー | 情報収集用の Web 検索: ネイティブ `web_search` または DeepSeek / Brave / Tavily / Serper / Bocha |


---

<a id="quick-start"></a>

## クイックスタート

Linux、macOS、Windows に対応しています。

| 必要なもの | 補足 |
|----|----|
| リリースを実行 | バイナリ + `config.yaml` |
| ソースからビルド | Go **1.24+**、Node.js（フロントエンド） |
| Docker 実行モード | Docker エンジンへアクセス可能。ローカル Flask/エージェントのみなら不要 |
| ローカル Flask 実行モード | `python3` + `flask`、`flask_cors`、`requests` |
| QR キャプチャ | Python 3 + **ワークベンチ → QR フィッシング** の zip（または [`scripts/qr-relay`](scripts/qr-relay/README.md)）。macOS のデコードには `zbar` が必要 |
| エージェント | HTTP(S) でコントロールプレーンへ到達できること |

| 項目 | 既定値 |
|------|--------|
| コンソール / API | `:8000` |
| フロントエンド開発サーバー | `:8081` → `/api` を `:8000` にプロキシ |
| エージェントのランディングポート | エージェントホストの `20000–30000/TCP` |

### 1. 設定

``` bash
cp config.yaml.example config.yaml
```

実際の演習前にプレースホルダーを置き換えてください。

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

`cmd/generate-encryption-key` は `encryption_key` を生成できます。メール追跡 (パブリック URL、HMAC、リダイレクト許可リスト) は、起動後に **システム → メール追跡** で構成されます。YAML は初期値およびフォールバックとして扱われます。クリック/オープンパスはキャンペーンごとに生成されます。

`config.yaml`、`data/`、または TLS キー (gitignored) をコミットしないでください。

### 2. 実行

``` bash
./spearhook
# or a build from ./release/ after ./build-all.sh
```

空/デフォルトのシークレットは起動時の警告を表示します。実験環境では問題ありませんが、本番運用前に変更してください。

### 3. ビルド

``` bash
./build-all.sh
```

フロントエンドを構築し、次に Linux / Windows / darwin (amd64 および arm64) 用のコントロール プレーン + エージェントを構築し、`config.yaml.example` + `agent.yaml.example` を `release/` にコピーします。

UI のみ (API はすでに起動しています):

``` bash
cd frontend && npm install && npm run dev   # http://127.0.0.1:8081
```

### 4. ログイン

`http://localhost:8000` (または `server.port`) を開きます。最初の起動では、管理者の資格情報が次の場所に書き込まれます。

``` bash
cat data/admin_password.txt
```

パスワードを保存してからファイルを削除します。その後のリセット: `cmd/reset-password`。

---

<a id="agents"></a>

## エージェント

すべてのランディングページをコントロールプレーンで動かしたくない場合は、別ホストに **spearhook-agent** を配備できます。1 つのプロジェクトを複数ノードへ、1 つのノードで複数プロジェクトをホストできます。各バインディングは独立したポート、リビジョン、ログを持ちます。認証情報の送信はコントロールプレーンへリアルタイム中継されます。アクセスログは `pending-logs.json` にバッファされ、復旧後に再送されます。

プロジェクトポートの既定値は **`20000–30000/TCP`** です。エージェントはコントロールプレーンへ**アウトバウンド**接続します。訪問者はエージェントへアクセスします（リバースプロキシ配下では `advertise_host` を設定。詳細はガイド参照）。

**トークン** — コントロールプレーンで `security.agent_registration_token`（JWT シークレットではない）を設定して再起動します。初回登録時に共有トークンを 1 回使用します。以降は `{work_dir}/state.json` にエージェント固有の ID/トークンを保持します（Linux/macOS は `0600`）。

**起動** — `./build-all.sh` の後:

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

`agent … connected to http://…` のようなログに接続成功を示す行が表示されます。

**UI** — **システム → エージェント** でオンラインになるまで待機します（ハートビートが約 90 秒途切れるとオフラインと判定）。**プロジェクト → 新規** でエージェントを選択し、ページを添付します。エージェントごとに独立したインスタンス（`pending` → `deploying` → `running`）が作成されます。プロジェクトを編集すると新しいリビジョンが作成され、エージェントを追加するとデプロイが実行され、選択を解除すると削除タスクが送信されます。

systemd、Windows サービス、リバースプロキシ: **[docs/agent-deployment.md](docs/agent-deployment.md)**.

---

<a id="console"></a>

## コンソール

| エリア | 内容 |
|----|----|
| **ダッシュボード** | 件数とホスト CPU / メモリ |
| **AI アシスタント** | プラットフォームツールによる自然言語操作（クローン、プロジェクト作成、エージェント一覧など） |
| **プロジェクト** | 概要、デプロイ、ログ、認証情報レコード |
| **ワークベンチ → ページビルダー** | HTML または AI ミラーした URL をページライブラリに保存 |
| **ワークベンチ → メール送信** | テンプレート、キャンペーン、開封/クリック |
| **ワークベンチ → 情報収集** | AI Web 検索 → 連絡先 CSV |
| **ワークベンチ → QR フィッシング** | ライブ QR リレー + キャプチャスクリプト zip |
| **プッシュチャネル** | 認証情報送信時の Webhook |
| **エージェント** | オンライン状態 / 最終確認 |
| **IP ブラックリスト** | 資格情報レコードから大量送信元 IP をブロック |
| **メールサービス** | SMTP プロファイル |
| **ユーザー / AI 設定 / メール追跡** | 管理者 |
| **監査ログ** | 管理者のみ。製品内から削除不可 |

ページは既定で `/api/submit` に POST されます。HTML プレースホルダー: `{{SUBMIT_URL}}`、`{{REDIRECT_URL}}`、`{{QR_RELAY_URL}}`、`{{QR_RELAY_IMG}}` など。メール: `{{email}}`、`{{click_url}}`、`{{open_pixel}}`、`{{landing_url}}` など。

ガイド: [EN](docs/placeholders.en.md) · [ZH](docs/placeholders.zh.md)

アクセスログ: `data/access-YYYY-MM-DD.log`。高頻度のダッシュボードポーリングはコンソールストリームには含まれませんが、ファイルには記録されます。

---

<a id="disclaimer"></a>

## 免責事項

Spearhook は、許可されたレッドチームによるフィッシング演習およびセキュリティ意識向上トレーニングにのみ使用してください。許可のない人物やシステムに対して使用した場合のリスクと責任は利用者に帰属し、本プロジェクトはそのような用途を支持しません。

コードは [MIT License](LICENSE) に基づいて提供されます。

Issue と PR を歓迎します。問題が発生した場合は GitHub Issue でお知らせください。
