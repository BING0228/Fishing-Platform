# Spearhook AI Native 架构说明（可直接实现）

> 目标读者：**后端 / 前端开发工程师**。  
> 本文是实现规格，不是可行性讨论。可行性见 [ai-native-assessment.md](ai-native-assessment.md)。  
> 原则：**模型只编排，平台只执行**；所有状态变更必须经过白名单 Tool；不新开第二套业务后端。

---

## 0. 一句话目标

把现有「Chat + 少量 Tool」升级为 **AI Native Ops Console**：

- 自然语言是高频作业的**一等入口**
- 每个平台能力对应 **1 个可调用 Tool**（薄封装现有 handlers / utils）
- 编排器负责：LLM tool calling → 权限 → 确认 → 执行 → SSE 回灌 → 审计
- 工作台 UI **保留**，用于精细编辑与回看；Chat 负责闭环与跳转

---

## 1. 非目标（禁止做成）

| 禁止 | 原因 |
|------|------|
| 模型直接生成落地页 HTML 写入项目 | 绕过页面库、Absolutize、审计 |
| 模型直连 SQLite / 任意 SQL | 权限与注入不可控 |
| 任意 shell / 读本地文件 Tool | 攻击面过大 |
| 删除现有工作台表单 | 红队仍需表格、CSV、人工核对 |
| Chat 内改 `encryption_key` / JWT / Basic Auth | 安全配置必须人工 |

---

## 2. 目标架构

```text
┌──────────────────────────────────────────────────────────────────┐
│ SPA (/chat + 工作台)                                              │
│  ChatPage: sessions · messages · thinking · tools · confirm ·     │
│            progress · playbook chips · deep links                 │
└────────────────────────────┬─────────────────────────────────────┘
                             │ JWT  Bearer (+ optional Basic)
                             │ POST /api/chat/sessions/:id/messages/  (SSE)
                             │ POST /api/chat/sessions/:id/confirm/   (SSE)
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ handlers/chat.go — Orchestrator                                   │
│  rate limit → load history → system prompt → LLM loop             │
│  intent hooks (force clone) → confirm gate → emit SSE             │
└───────────────┬─────────────────────────────┬────────────────────┘
                │                             │
                ▼                             ▼
┌───────────────────────────┐   ┌──────────────────────────────────┐
│ utils/chat_llm.go         │   │ chattools/ (NEW package)          │
│ ChatCompletions(Stream)   │   │ Registry · Definition · Execute   │
│ + tools / tool_choice     │   │ Risk: read|write|destructive      │
└───────────────────────────┘   └──────────────┬───────────────────┘
                                               │ wraps
                 ┌─────────────────────────────┼─────────────────────┐
                 ▼                             ▼                     ▼
          page mirror / AI rewrite      info gather jobs      projects/agents/
          (existing)                    (existing)            mail/qr/robots…
                                               │
                                               ▼
                                    ActiveAIConfig() · GORM · audit
```

### 2.1 相对现状的增量

| 现状 | AI Native 增量 |
|------|----------------|
| `handlers/chat_tools.go` 内联 switch | 抽成 **`chattools` 包**（注册表 + 元数据 + Execute） |
| LLM 非流式整包 | **`stream=true`** 解析 delta，SSE `token` 真流式 |
| 7 个工具 | 扩展到 **作业闭环工具集**（见 §5） |
| 无长任务回灌 | SSE 增加 **`progress`**，镜像/搜集可跟进度 |
| 确认开关全局空 map | **按 Risk 分级**（read 自动 / write 可配 / destructive 强制确认） |
| 写操作审计弱 | 每个写 Tool 成功后写 **`audit_logs`** |
| 无配方 | **Playbook** 常量 + 前端 chips（服务端仍走同一编排器） |

---

## 3. 目录与文件职责（工程师按此改）

```text
handlers/
  chat.go                 # HTTP + SSE 编排主循环（保留，变薄）
  chat_intent.go          # URL/克隆意图、拒答探测（保留并扩展）
  chat_confirm.go         # NEW: 确认策略（读 Risk 元数据）
  ai_settings.go          # 不变：ActiveAIConfig()

chattools/                # NEW package spearhook/chattools
  registry.go             # Register / All / Get / DefinitionsForLLM()
  types.go                # ToolMeta, Risk, ExecContext, Result, Link
  execute.go              # Execute(ctx, name, argsJSON) → Result
  tools_agents.go         # list_agents
  tools_projects.go       # list/get/create/project_action
  tools_pages.go          # clone_page, list_pages, get_page
  tools_mail.go           # list_smtp, create_mail_draft, preview_mail, send_mail_campaign
  tools_gather.go         # start_info_gather, get_info_gather, list_findings
  tools_robots.go         # list_robots, bind_robot, list_project_hooks
  tools_qr.go             # create_qr_relay, get_qr_relay
  tools_security.go       # list_credentials (scoped), export_credentials, add_ip_blacklist
  playbooks.go            # 预置话术/步骤说明（给 system 或前端）
  *_test.go

utils/
  chat_llm.go             # 扩展 StreamChatCompletionsWithTools
  chat_llm_stream.go      # NEW: SSE/chunk 解析

frontend/src/
  spa/pages/ChatPage.tsx  # progress / playbook / confirm by risk
  api/index.ts            # ChatSSEEvent 增加 progress
  i18n/messages.json      # 新文案
```

**迁移规则：**

1. 先把现有 `handlers/chat_tools.go` 的逻辑 **原样搬进** `chattools/`，行为不变，测试绿灯。
2. 再按 §11 分期加工具，禁止在 `handlers/` 里继续堆 switch。
3. `handlers/chat.go` 只依赖 `chattools.DefinitionsForLLM()` + `chattools.Execute()`。

---

## 4. 核心类型（Go 契约）

实现时类型名可微调，**字段语义必须一致**。

```go
// package chattools

type Risk string

const (
  RiskRead         Risk = "read"         // 自动执行
  RiskWrite        Risk = "write"        // 默认自动；可被配置要求确认
  RiskDestructive  Risk = "destructive"  // 强制 confirm_required
)

type Link struct {
  Label string `json:"label"`
  Href  string `json:"href"` // SPA path 或 http(s)
}

type Result struct {
  OK      bool   `json:"ok"`
  Error   string `json:"error,omitempty"`
  Summary string `json:"summary,omitempty"` // 给人看的一句话
  Result  any    `json:"result,omitempty"`  // 结构化，给下一轮模型
  Links   []Link `json:"links,omitempty"`
}

type ExecContext struct {
  Ctx    context.Context
  User   models.User          // 含 ID / Role
  Emit   func(event string, payload any) // 可选：长任务 progress
  // 禁止传入 *gorm.DB 以外的「上帝对象」；DB 从 config.GetDB() 取
}

type ToolMeta struct {
  Name        string
  Description string // 给 LLM，英文为主，可中英关键
  Risk        Risk
  Parameters  json.RawMessage // JSON Schema object
  // ConfirmMessage 可选：确认卡文案模板
  ConfirmMessage string
}

type Tool interface {
  Meta() ToolMeta
  Execute(ec ExecContext, args map[string]any) Result
}
```

### 4.1 注册表 API

```go
func Register(t Tool)
func Get(name string) (Tool, bool)
func DefinitionsForLLM() []utils.ChatTool   // 转 OpenAI tools[]
func RequiresConfirm(name string, cfg ConfirmPolicy) bool
```

`ConfirmPolicy`（可先写死常量，后接入配置）：

```go
type ConfirmPolicy struct {
  ConfirmWrites       bool // default false（与现状一致）
  ConfirmDestructive  bool // default true
  // 强制名单：send_mail_campaign 永远确认
  AlwaysConfirm []string
}
```

---

## 5. 工具目录（实现清单）

### 5.1 已有（迁移后行为不变）

| Name | Risk | 参数要点 | 包装 |
|------|------|----------|------|
| `list_agents` | read | 无 | 现 `toolListAgents` |
| `list_projects` | read | 无 | ownership 过滤 |
| `get_project` | read | `project_id` | 含 deployments / runtime_url |
| `clone_page` | write | `url`, `name?`, `description?` | 现镜像流水线；**禁止**模型写 HTML |
| `create_project` | write | `name`, `phishing_page_id`, `agent_ids?`, … | 现创建逻辑 |
| `list_smtp` | read | 无 | |
| `list_robots` | read | 无 | |

别名：`create_phishing_page` → 同 `clone_page`（兼容旧 prompt）。

### 5.2 Phase A 必做

| Name | Risk | 参数 | 包装现有 |
|------|------|------|----------|
| `list_pages` | read | `limit?` | `phishing_pages` 按 ownership |
| `get_page` | read | `page_id` | 单页元数据（不要回传整页 HTML 给模型，可截断） |
| `start_info_gather` | write | `target`, `notes?`, `include_x_search?` | `InfoGatherJob` 创建+异步跑 |
| `get_info_gather` | read | `job_id` | status / counts / summary |
| `list_info_findings` | read | `job_id`, `kind?` | findings 列表（限 100） |

### 5.3 Phase B 必做

| Name | Risk | 参数 | 说明 |
|------|------|------|------|
| `create_mail_draft` | write | `smtp_id`, `template` 或 `subject`+`html`, `recipients[]` | **只建草稿/活动，不发送** |
| `preview_mail` | read | `campaign_id` 或 `template_id` 或 `subject`+`body` | 渲染预览，不发送 |
| `send_mail_campaign` | destructive | `campaign_id` | **AlwaysConfirm** |
| `bind_robot` | write | `project_id`, `robot_id` | 绑 Webhook |
| `list_project_hooks` | read | `project_id` | 已绑定的 Webhook |
| `project_action` | write | `project_id`, `action`, `target?`: `local`\|`agent` | local = 主控本机（Docker 容器 / local Flask）；agent = 远端 Agent 部署 |
| `update_project` | write | `project_id`, `name?`, `run_mode?`, `phishing_page_id?`, `agent_ids?`, `robot_ids?` … | 修改已有项目；仅更新传入字段 |
| `build_project` | write | `project_id` | 构建 Docker 镜像（本机 Docker 启动前置） |
| `get_project_logs` | read | `project_id`, `limit?` | 构建/容器状态、容器日志、部署日志 |
| `list_info_gather_jobs` | read | `limit?` | 信息搜集历史 |
| `list_mail_campaigns` | read | `limit?` | 邮件活动列表 |
| `get_mail_campaign` | read | `campaign_id` | 活动状态/送达计数 |
| `list_mail_templates` | read | `limit?` | 邮件模板列表 |
| `list_qr_relays` | read | `limit?` | QR 中继列表 |
| `list_ip_blacklist` | read | `limit?` | IP 黑名单列表 |
| `create_qr_relay` | write | `name` | 返回 slug + 上传 token（token 仅一次进 tool_result，勿写入会话明文过久） |
| `get_qr_relay` | read | `relay_id` 或 `slug` | 健康状态 |
| `list_credentials` | read | `project_id`, `limit?` | operator 仅自己的项目 |
| `export_credentials` | destructive | `project_id?`, `limit?` | CSV 仅一次进 tool_result（ephemeral），不落库 |
| `add_ip_blacklist` | write | `ip`, `reason?`, `credential_id?` | 复用现规则（只能从凭据加则保持一致） |

### 5.4 永不做的 Tool

`delete_*`（批量销毁）、`update_ai_settings`、`update_security_config`、`run_sql`、`read_file`、`exec`。

### 5.5 工具 JSON Schema 约定

- `type: object`，`additionalProperties: false`
- 必填放 `required`
- ID 一律 `integer`（JSON number）
- 枚举用 `enum`
- Description 写清 **何时必须调用**（克隆场景已有范例，照抄风格）

---

## 6. 编排器状态机

文件：`handlers/chat.go` → `runChatOrchestrator`。

```text
                    ┌─────────────┐
                    │  load msgs  │
                    │ + sys prompt│
                    └──────┬──────┘
                           ▼
              ┌──── emit thinking:plan ────┐
              ▼                            │
     LLM (stream tokens if any)            │
              │                            │
              ├── tool_calls? ──no──► intent force clone?
              │                         │ yes: synth clone_page
              │                         │ no: emit done
              ▼ yes                        │
     emit thinking:tools                   │
              │                            │
              ▼                            │
     for each tool_call:                   │
       needs confirm? ──yes──► persist pending
              │                 emit confirm_required
              │                 emit done(awaiting)
              │                 STOP
              ▼ no
       emit tool_start
       Execute(...)
         └─ long job: emit progress
       persist tool message
       emit tool_result
       audit if write/destructive
              │
              ▼
       round++ < max? ──yes──► loop LLM
              │
              ▼ no
       emit error tool_round_limit + done
```

### 6.1 硬性参数（保持或配置化）

| 常量 | 建议值 | 位置 |
|------|--------|------|
| `chatMaxToolRounds` | 8 | 已有 |
| `chatMaxUserMsgRunes` | 8000 | 已有 |
| `chatRateMinInterval` | 800ms | 已有 |
| `chatDailyLimit` | 200 | 已有 |
| LLM `temperature` | 0.2 | `chat_llm.go` |
| LLM `max_tokens` | 4096（可升到 8192） | |

### 6.2 意图钩子（保留）

`handlers/chat_intent.go`：

- 用户含 HTTP(S) URL + 克隆/镜像语义，且本轮无 `tool_calls` → **强制** `clone_page` 一次（`forcedCloneDone` 防双跑）。
- 安全拒答文案不得作为终态（已有 `chatPostCloneSummary`）。

后续同类钩子（可选）：「搜集 xxx」→ 无 tool 时强制 `start_info_gather`。

### 6.3 System Prompt 维护规则

- 单一函数 `chattools.SystemPrompt()`（从 `chatSystemPrompt` 迁出）。
- **每加一个写工具**，同步改 prompt 的「必须调用 xxx」与「禁止项」。
- 测试：`TestChatSystemPromptListsTools` 断言关键工具名出现在 prompt。

---

## 7. SSE 协议（前后端契约）

`Content-Type: text/event-stream`  
每条：`event: <name>\ndata: <json>\n\n`

| event | data（JSON） | 前端行为 |
|-------|----------------|----------|
| `thinking` | `{phase, message?}` `phase=plan\|tools\|clone\|gather` | 显示思考气泡 |
| `token` | `{content}` | 追加 assistant 流式文本 |
| `tool_start` | `{tool_call_id, name, arguments}` | 工具运行中卡片 |
| `progress` | `{tool_call_id?, stage, percent, message}` | **NEW** 长任务进度条 |
| `tool_result` | `{tool_call_id, name, ok, summary?, error?, result?, links?}` | 更新卡片 + 链接 |
| `confirm_required` | `{tool_call_id, name, arguments, message, risk, session}` | 确认条 |
| `done` | `{message_id?, session?, awaiting_confirm?, error?}` | 结束本轮 |
| `error` | `{error}` | 展示错误；可仍跟 `done` |

### 7.1 前端类型扩展

`frontend/src/api/index.ts`：

```ts
| { event: "progress"; data: {
    tool_call_id?: string;
    stage?: string;
    percent?: number;
    message?: string;
  }}
```

`ChatPage.tsx`：在对应 `tool_call_id` 的 tool 行上显示 percent；无 id 则挂在最近 running tool。

---

## 8. LLM 客户端改造

### 8.1 非流式（保留）

`ChatCompletionsWithTools` — 工具轮、测试、不支持 stream 的供应商回退。

### 8.2 流式（新增）

```go
// utils/chat_llm_stream.go
func StreamChatCompletionsWithTools(
  ctx context.Context,
  baseURL, apiKey, model string,
  timeout time.Duration,
  messages []ChatLLMMessage,
  tools []ChatTool,
  onDelta func(text string), // 每个 content delta
) (ChatLLMResult, error)
```

要求：

1. `payload.Stream = true`
2. 解析 `data: {choices:[0].delta}`；拼 `content` 与 `tool_calls` 分片（按 index 合并 arguments 字符串）
3. 每收到 content delta 调 `onDelta` → orchestrator `emit("token", …)`
4. 供应商返回 `reasoning_content`（DeepSeek 等）：**下一轮 messages 必须按厂商要求回传**；在 `ChatLLMMessage` 增加可选字段 `ReasoningContent string \`json:"reasoning_content,omitempty"\``，编排器在 assistant 落库时一并保存/回灌
5. 流式失败则 **自动回退** 非流式一次，并打日志

---

## 9. 确认、权限、审计

### 9.1 确认

| Risk | 默认 |
|------|------|
| read | 永不确认 |
| write | 不确认（可用配置打开） |
| destructive | **总是**确认 |
| `AlwaysConfirm` 名单 | `send_mail_campaign` |

流程与现网一致：pending 字段写在 `chat_sessions`；`ConfirmChatTool` 继续 SSE。

### 9.2 权限

所有 Tool 内：

- 使用 `ExecContext.User`
- 复用现有 `ownership` / `scope*` 辅助函数（从 handlers 导出到 `ownership` 可访问处，或 chattools 调用 handlers 包内已有 unexported 时 **提升为 shared 小函数** 到 `handlers/ownership.go` / `internal/scope`）
- admin 看全局；operator 仅 `created_by = user.ID`

### 9.3 审计

写/破坏性 Tool 成功后：

```go
audit.Record(actor, action="chat_tool_"+name, resourceType, resourceID, detail JSON)
```

`detail` 含：`session_id`, `tool_call_id`, args 摘要（脱敏：勿记 API key、完整密码明文；凭据列表只记 count）。

---

## 10. Playbook（配方）

不另起工作流引擎。Playbook = **前端芯片 + system/user 提示增强**。

`chattools/playbooks.go`：

```go
type Playbook struct {
  ID          string   // "clone_deploy"
  TitleZH     string
  TitleEN     string
  PromptZH    string   // 填进输入框的完整用户话术模板
  PromptEN    string
  ToolHints   []string // 期望工具顺序，仅文档/测试用
}
```

预置至少：

1. `clone_deploy` — 克隆 URL → 建项目 → 绑在线 Agent  
2. `gather_contacts` — 对目标启动信息搜集  
3. `hook_notify` — 为项目绑定推送通道（需已有 robot）

前端：空会话展示 chips；点击填入 composer，用户可改 URL/名称后发送。

---

## 11. 分期实现（按 PR 拆）

### PR1 — 结构迁移（行为零变化）

**验收：**

- [ ] 新建 `chattools`，旧工具全迁入
- [ ] `go test ./handlers/ ./chattools/` 通过
- [ ] Chat 克隆 + 建项 E2E 与迁前一致
- [ ] 删除 `handlers/chat_tools.go` 中的重复实现（或变 thin wrapper 一周后删）

### PR2 — 真流式 + progress 事件壳

**验收：**

- [ ] 无 tool 的短回复可见逐字 `token`
- [ ] 前端 `progress` 类型编译通过（可暂无生产者）
- [ ] 流式失败回退非流式

### PR3 — Phase A 工具

**验收：**

- [ ] `list_pages` / `get_page` 单测 + Chat 话术
- [ ] `start_info_gather` 后会话内 `get_info_gather` 能查到状态
- [ ] 长任务至少一次 `progress`（可在 gather 回调里 `ec.Emit`）
- [ ] 写工具打 audit

### PR4 — Phase B 邮件/项目/QR

**验收：**

- [ ] `create_mail_draft` 不发信
- [ ] `send_mail_campaign` 必出 `confirm_required`，拒绝则不发送
- [ ] Playbook chips 3 个可见
- [ ] README / 本文工具表与代码 `Register` 列表一致（可加测试：注册名 ⊆ 文档扫描，或反过来）

### PR5 — 打磨

- 上下文压缩（超 N 轮摘要成一条 system/assistant）
- 建议芯片（tool 成功后）
- 限流配置化进 `config.yaml`

---

## 12. HTTP API（保持兼容，仅扩展语义）

| Method | Path | 说明 |
|--------|------|------|
| GET | `/api/chat/sessions/` | 列表 |
| POST | `/api/chat/sessions/` | 创建 |
| GET | `/api/chat/sessions/:id/messages/` | 历史 |
| DELETE | `/api/chat/sessions/:id/` | 删会话 |
| POST | `/api/chat/sessions/:id/messages/` | **SSE** 用户发言 |
| POST | `/api/chat/sessions/:id/confirm/` | **SSE** 确认/拒绝 |

可选后续（非必须）：

| GET | `/api/chat/playbooks/` | 返回 playbook 列表给前端 |

Body 不变：`{"content":"..."}` / `{"tool_call_id":"...","approve":true}`。

---

## 13. 数据模型

**不改表也可落地 Phase A/B。** 可选增强：

```text
chat_messages
  + reasoning_content TEXT  -- 厂商 thinking 回传

-- 可选独立表（若不想塞进 messages）
chat_tool_runs
  id, session_id, tool_call_id, name, args_json, result_json,
  risk, status, created_at
```

短期：继续用现有 `chat_messages.tool_*` + `chat_sessions.pending_*`。

---

## 14. 测试规格（必须写进 PR）

| 测试 | 断言 |
|------|------|
| `chattools/registry_test.go` | 无重名；每个 Tool Schema 可 JSON 解析；`additionalProperties=false` |
| `chattools/tools_*_test.go` | 用 sqlite 测试库：operator 看不到他人项目 |
| `handlers/chat_intent_test.go` | 强制克隆仍绿 |
| `utils/chat_llm_stream_test.go` | 用 `httptest` 喂两段 delta，合并出完整 content + tool_calls |
| `handlers` SSE 单测或集成 | `confirm`：destructive 工具先 `confirm_required` |
| 前端 | `ChatSSEEvent` 联合类型含 `progress`；ChatPage 不因未知事件崩溃 |

禁止：测试里 hardcode 业务成功结果却不调用 `Execute` 真逻辑。

---

## 15. 安全检查表（每个写 Tool PR）

- [ ] Schema 无开放对象乱塞
- [ ] URL 类参数走现有 SSRF/镜像策略
- [ ] 密钥/密码不进 `summary`、不进 audit 明文
- [ ] ownership 单测覆盖 operator
- [ ] destructive 确认路径手工或自动测过
- [ ] system prompt 已更新

---

## 16. 工程师「第一周」最小任务拆解

1. `mkdir chattools` + 搬迁 7 工具 + 注册表 + 改 `chat.go` import（PR1）。  
2. 实现 `StreamChatCompletionsWithTools` + orchestrator 接线（PR2）。  
3. 加 `list_pages`、`start_info_gather`、`get_info_gather`（PR3）。  
4. 前端：`progress` + 3 个 playbook chip（可并行）。  

完成以上即可在演示中声称：**AI Native 骨架落地，作业闭环工具可按表继续加**。

---

## 17. 与评估文档的关系

| 文档 | 用途 |
|------|------|
| [ai-native-assessment.md](ai-native-assessment.md) | 为什么能做、差距、优先级 |
| **本文** | 怎么做：包结构、类型、协议、工具表、PR 切片、验收 |

实现以 **本文为准**；若与旧对话方案冲突，以本文 + 当前代码行为（确认默认可关、强制克隆等）为准。

---

## 18. 附录：clone_page 执行要点（已有，勿回归）

1. `ActiveAIConfig()` 无启用项 → 清晰错误。  
2. Fetch → AbsolutizeHTMLAssets → AI rewrite → 写入 `phishing_pages` + `html_files`。  
3. `Result.Links` 含页面制作深链，例如 `/workbench/page-builder`。  
4. Chat 内禁止把改写后整页 HTML 再 echo 给用户（summary + id 即可）。

---

*维护：新增 Tool 时同步更新 §5 表格、system prompt、Playbook（如需要）、以及 `docs/README.md` 索引。*
