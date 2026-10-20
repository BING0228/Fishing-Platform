// Package chattools is the whitelisted platform-tool registry used by the chat
// orchestrator. Models only orchestrate; every state change goes through a
// registered tool. Tools wrap existing platform behaviour and never expose raw
// SQL, shell, or HTML generation.
package chattools

import (
	"context"
	"encoding/json"

	"spearhook/models"
)

// Risk classifies a tool for the confirmation gate and audit.
type Risk string

const (
	// RiskRead is always executed automatically.
	RiskRead Risk = "read"
	// RiskWrite mutates state; optional confirmation via ConfirmPolicy.ConfirmWrites.
	RiskWrite Risk = "write"
	// RiskDestructive is irreversible/high-impact; confirms by default.
	RiskDestructive Risk = "destructive"
)

// Link is a deep link surfaced to the SPA after a tool call.
type Link struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

// Result is the canonical tool outcome. It is persisted as the tool message and
// fed back to the model on the next round.
type Result struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Summary string `json:"summary,omitempty"`
	Result  any    `json:"result,omitempty"`
	Links   []Link `json:"links,omitempty"`
	// Ephemeral is emitted once in the live SSE tool_result but never persisted
	// to chat history (e.g. one-time upload tokens).
	Ephemeral map[string]any `json:"-"`
}

// ExecContext carries the caller identity and optional progress emitter.
type ExecContext struct {
	Ctx        context.Context
	User       models.User
	SessionID  uint
	ToolCallID string
	// Emit optionally pushes an SSE event (used for long-running progress).
	Emit func(event string, payload any)
}

// Context returns a usable context even when the caller left it nil.
func (ec ExecContext) Context() context.Context {
	if ec.Ctx == nil {
		return context.Background()
	}
	return ec.Ctx
}

// Progress emits a progress event when an emitter is present.
func (ec ExecContext) Progress(stage string, percent int, message string) {
	if ec.Emit == nil {
		return
	}
	ec.Emit("progress", map[string]any{
		"tool_call_id": ec.ToolCallID,
		"stage":        stage,
		"percent":      percent,
		"message":      message,
	})
}

// ToolMeta is the static description of one tool.
type ToolMeta struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Risk           Risk            `json:"risk"`
	Parameters     json.RawMessage `json:"parameters"`
	ConfirmMessage string          `json:"confirm_message,omitempty"`
	// Hidden tools are callable but not advertised to the model (aliases).
	Hidden bool `json:"hidden,omitempty"`
}

// Tool is one callable platform capability.
type Tool interface {
	Meta() ToolMeta
	Execute(ec ExecContext, args map[string]any) Result
}

type funcTool struct {
	meta ToolMeta
	run  func(ec ExecContext, args map[string]any) Result
}

func (t funcTool) Meta() ToolMeta { return t.meta }

func (t funcTool) Execute(ec ExecContext, args map[string]any) Result { return t.run(ec, args) }

// ConfirmPolicy controls which tools pause for explicit user approval.
type ConfirmPolicy struct {
	// ConfirmWrites asks before write tools (default false, matching legacy behaviour).
	ConfirmWrites bool
	// ConfirmDestructive asks before destructive tools (default true).
	ConfirmDestructive bool
	// AlwaysConfirm forces confirmation regardless of risk/other flags.
	AlwaysConfirm []string
}

// DefaultConfirmPolicy is the shipped policy: read auto, write auto, destructive
// confirm, and mail sending always confirms.
var DefaultConfirmPolicy = ConfirmPolicy{
	ConfirmWrites:      false,
	ConfirmDestructive: true,
	AlwaysConfirm:      []string{"send_mail_campaign"},
}

// Playbook is a preset natural-language recipe surfaced as a chat chip.
type Playbook struct {
	ID        string   `json:"id"`
	TitleZH   string   `json:"title_zh"`
	TitleEN   string   `json:"title_en"`
	PromptZH  string   `json:"prompt_zh"`
	PromptEN  string   `json:"prompt_en"`
	ToolHints []string `json:"tool_hints,omitempty"`
}
