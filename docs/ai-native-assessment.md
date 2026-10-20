# Spearhook：能否做成 AI Native？架构评估

> 基于当前代码库（控制面 Go/Gin、Chat SSE 编排、AI Profile、页面镜像、信息搜集、Agent）的现状评估与路径建议。  
> 结论先行：**可以做成 AI Native，且底座已经具备；当前是「AI 增强的作业台」，距离「AI Native 作业台」差的是工具覆盖面、默认入口与编排深度，不是推倒重来。**

---

## 1. 什么叫 AI Native（对本项目的定义）

对 Spearhook 这类红队作业台，**AI Native** 不等于「到处挂个聊天框」，而是：

| 维度 | 含义 |
|------|------|
| **默认入口** | 高频操作优先用自然语言完成；表单/工作台是精细编辑与审计回看，不是唯一路径 |
| **能力即工具** | 平台真实能力（克隆、建项、部署、发信、搜集、QR、黑名单…）全部以可调用 Tool 暴露；模型不直连 DB、不编造状态 |
| **多步编排** | 一句话能串起「搜集 → 镜像 → 建项 → 绑 Agent → 草稿发信」等闭环，结果可跳转控制台 |
| **人在回路** | 高危写操作可确认/可审计；权限仍走 JWT + ownership |
| **同一大脑** | 镜像、对话、搜集共用 AI Profile；提示词与工具描述一致，不各自为政 |

对照业界：更接近「带 Tool Calling 的控制面 Agent」（如部分 C2/运维 Copilot），而不是通用 ChatGPT 皮肤。

---

## 2. 当前架构（As-Is）

```text
                    ┌─────────────────────────────────────┐
  Browser SPA       │  /chat · 工作台 · 项目 · Agent …     │
  (Next embed)      └──────────────┬──────────────────────┘
                                   │ JWT (+ optional Basic)
                                   ▼
  Control plane (spearhook)  Gin + SQLite + go:embed UI
                                   │
         ┌─────────────────────────┼─────────────────────────┐
         ▼                         ▼                         ▼
  Chat orchestrator          Page mirror / AI rewrite    Info gather
  handlers/chat*.go          handlers/page_mirror.go     handlers/info_gathering.go
  SSE: thinking/token/       SSE progress                job + findings
  tool_*/done                utils/ai_rewrite.go         utils/ai_web_search.go
         │                         │                         │
         └────────────┬────────────┴────────────┬────────────┘
                      ▼                         ▼
              ActiveAIConfig()            OpenAI-compatible
              (单一启用 AIProfile)         chat/completions
                                           /responses + web_search
                      │
                      ▼
              Platform tools (白名单)
              list_agents / list_projects / get_project
              clone_page / create_project
              list_smtp / list_robots
                      │
                      ▼
              现有 handlers / GORM / Agent runtime
```

### 2.1 已具备的 AI 相关能力

| 模块 | 实现要点 | 成熟度 |
|------|----------|--------|
| **AI Settings** | 多 Profile、同时只启用一个；密钥加密；探测 chat/completions 与 web_search | 高 |
| **Chat** | 会话持久化、SSE、tool calling 多轮、thinking、意图强制 `clone_page`、ownership 进工具 | 中高（P0+P1） |
| **页面 AI 镜像** | URL/上传 → fetch → Absolutize 资源 → LLM 改写 → 入库 | 高 |
| **信息搜集** | `/responses`+`web_search`（含 GLM 回退）→ 结构化 finding → CSV | 高 |
| **Agent** | 远程部署、心跳、日志缓冲；Chat 可 `create_project` 绑 Agent | 高（运维面） |

### 2.2 Chat 工具白名单（今天）

已暴露：

- 读：`list_agents`、`list_projects`、`get_project`、`list_smtp`、`list_robots`
- 写：`clone_page`（=`create_phishing_page`）、`create_project`

**明确未暴露（系统提示也写了不做）：** 发信、删除、改 AI/安全配置、任意 shell、QR 全流程、黑名单、信息搜集任务、项目启停/重建等。

### 2.3 关键约束（设计正确、也限制「全能 AI」）

1. **LLM 不得发明 HTML / 假数据** — 镜像必须走 `clone_page` 流水线（正确，应保留）。
2. **单 AI Profile** — 对话 / 镜像 / 搜集共用启用项（简单，后续可拆「编排模型 vs 改写模型」）。
3. **LLM 调用非流式** — `ChatCompletionsWithTools` 整包返回，再由控制面 SSE 给前端（延迟体感不如真 token 流）。
4. **工具面 ≪ 产品面** — 大量控制台能力只能点 UI，Chat 说「没这个工具」就停。

---

## 3. 现状定位：AI 增强，尚未 AI Native

| 能力 | 今天 | AI Native 目标态 |
|------|------|------------------|
| 入口 | Chat 是并列 Tab；主路径仍是工作台表单 | Chat / 自然语言是默认作业入口之一 |
| 覆盖 | 克隆 + 建项闭环基本通；发信/搜集/QR 断在 Chat 外 | 核心作业路径工具齐全 |
| 编排 | 多 tool 轮次有；缺任务级状态机与长任务进度回灌 Chat | 长任务（镜像、搜集、发信）在对话里可跟进度 |
| 体验 | thinking + tool 卡片有；LLM 侧非流式 | 真流式 token + 可中断 + 建议下一步 |
| 数据 | 会话/消息入库；与 audit 弱关联 | 每次 tool 可审计、可复盘 |
| 安全 | 白名单 + ownership；确认开关可关 | 分级：读自动 / 写确认 / 销毁类强确认 |

**一句话：** 骨架是 AI Native 的（Tool Calling 控制面），肌肉还不够（工具与入口）。

---

## 4. 可行性结论

### 4.1 能做吗？

**能。** 不需要换技术栈，也不需要把控制台改成「纯对话产品」。

理由：

1. 已有 **服务端编排器 + OpenAI 兼容 tools + SSE UI** — 这是最贵的一块。
2. 业务能力已在 handlers 里，扩展 AI Native ≈ **把现有 API 包成 Tool**，而不是重写业务。
3. 镜像 / 搜集已经是「AI 流水线」；Chat 只需 **调用这些流水线** 并回灌进度。
4. 权限模型（JWT、admin/operator、created_by）可直接复用进工具层。

### 4.2 不应该做成什么样？

- **不要**让模型直接写 HTML 当落地页（绕过页面库与审计）。
- **不要**让模型持有 DB 连接或执行任意 SQL/shell。
- **不要**把所有按钮删掉只留 Chat（红队仍需表格、日志、CSV、人工核对）。
- **不要**在未授权语境下弱化系统提示的「授权演练」边界。

推荐产品形态：**AI Native Ops Console** = Chat 驱动闭环 + 工作台精细编辑 + 全量审计。

---

## 5. 缺口清单（按优先级）

### P0 — 不扩能力也先「像 AI Native」

| 项 | 说明 |
|----|------|
| 真流式 LLM | `chat/completions` `stream=true`，边生成边 `token`（工具轮仍可非流） |
| 工具结果可操作 | 统一 `links[]`（打开项目 / 页面库 / 搜集任务）— 部分已有，需全覆盖 |
| 失败可恢复 | 工具失败时给出「下一步建议」+ 可重试同一 tool_call |
| 审计打通 | `clone_page` / `create_project` 等写工具写入 `audit_logs` |

### P1 — 补齐作业闭环工具（高杠杆）

| 工具建议 | 包装现有能力 | 风险 |
|----------|--------------|------|
| `start_info_gather` / `get_info_gather` | 信息搜集 job | 中（耗额度） |
| `list_pages` / `get_page` | 页面库 | 低 |
| `bind_robot` / `list_project_hooks` | 项目绑 Webhook | 中 |
| `create_mail_draft` / `preview_mail` | 模板+活动草稿，**不真发** | 低 |
| `send_mail_campaign` | 真发信 | **高 — 默认确认** |
| `list_credentials` / `export_credentials` | 凭据查看/导出 | 高 — 确认 + 权限 |
| `create_qr_relay` / `get_qr_relay` | QR | 中 |
| `project_action`（start/stop/redeploy） | 部署生命周期 | 中高 |
| `add_ip_blacklist` | 黑名单 | 中 |

原则：**读自动、写可配确认、不可逆强确认。**

### P2 — 编排与产品化

| 项 | 说明 |
|----|------|
| **Playbook / 配方** | 预置「克隆并部署」「搜集并导入发信名单」；Chat 可一键跑或逐步确认 |
| **长任务桥** | 镜像/搜集进度 SSE 汇入同一会话（`progress` 事件），避免「调完就静音」 |
| **多模型角色** | 可选：编排用快模型，HTML 改写用强模型（仍可同一 Provider） |
| **上下文压缩** | 长会话摘要，避免 history 撑爆 token |
| **建议芯片** | 空会话 / 工具成功后展示下一句自然语言建议（已有 suggestion 雏形可加强） |

### P3 — 可选深水区

- Agent 侧轻量「边缘助手」（一般不需要）
- 本地/私有模型（已兼容任意 OpenAI 根地址，主要是运维问题）
- 跨会话记忆（项目级 notebook，慎做「永久记住密码」类）

---

## 6. 推荐落地路径（不推倒重来）

```text
Phase A（1–2 周量级）
  真流式 + 审计 + list_pages + info_gather 读写工具
  → Chat 能：镜像 / 建项 / 发起搜集 / 查结果

Phase B
  邮件草稿工具 + 确认后 send + 绑 Webhook + 项目启停
  → Chat 覆盖「从页到信」主路径

Phase C
  Playbook + 长任务进度回灌 + 上下文压缩
  → 体验上达到「AI Native 作业台」
```

架构改动集中在：

- `handlers/chat_tools.go` — 加工具，薄封装现有 service/handler 逻辑  
- `handlers/chat.go` — 流式、progress、确认策略  
- `utils/chat_llm.go` — stream 解析  
- 前端 `ChatPage` — progress / playbook chips  
- **不必**新建第二套业务后端

---

## 7. 风险与约束

| 风险 | 缓解 |
|------|------|
| 模型拒答 / 乱调工具 | 保留意图探测与强制 tool（已有 clone）；关键 tool 描述与 few-shot |
| 误发信 / 误删 | 写工具分级确认；确认卡默认开启发信类 |
| SSRF / 任意 URL 克隆 | 继续复用镜像侧 URL 策略 |
| Token / 费用暴涨 | 每用户 QPS、日限额、最大 tool 轮数（计划里已有，需产品化开关） |
| DeepSeek 等 thinking 模式字段 | 已遇 `reasoning_content` 回传问题；对接时按厂商协议扩展消息结构 |
| 「AI Native」营销过度 | README 如实写：平台工具驱动，非通用 LLM |

---

## 8. 总评

| 问题 | 答案 |
|------|------|
| 当前架构能否支撑 AI Native？ | **能** |
| 要不要重写？ | **不用**；扩展 Tool + 流式 + 长任务桥即可 |
| 现在算不算 AI Native？ | **不算完整**；算「AI-assisted + 局部 Agent」 |
| 最短路径到可用 AI Native？ | 补搜集/页面库/邮件草稿工具 + 发信确认 + 真流式 |
| 核心护城河 | 不是聊天 UI，而是 **白名单工具 = 真实红队作业能力** 且不可被模型绕过 |

---

## 9. 相关代码索引

| 路径 | 作用 |
|------|------|
| `handlers/chat.go` | SSE 编排、强制克隆、thinking |
| `handlers/chat_tools.go` | 工具定义与执行 |
| `handlers/chat_intent.go` | URL/克隆意图与拒答探测 |
| `utils/chat_llm.go` | OpenAI 兼容 tool calling |
| `utils/ai_rewrite.go` | 页面镜像改写 |
| `utils/ai_web_search.go` | 信息搜集 |
| `handlers/ai_settings.go` | AI Profile |

---

*文档性质：架构评估与路线建议，不承诺排期。实现时以工具白名单增量为主，保持「模型只编排、平台只执行」。*
