package chattools

import (
	"encoding/json"
	"fmt"
	"strings"

	"spearhook/audit"
	"spearhook/config"
)

const aliasCreatePhishingPage = "create_phishing_page"

// Execute parses the JSON arguments and runs the named tool. Successful write
// and destructive tools are recorded in the audit log.
func Execute(ec ExecContext, name, argsJSON string) Result {
	name = strings.TrimSpace(name)
	if name == aliasCreatePhishingPage {
		name = "clone_page"
	}
	argsJSON = strings.TrimSpace(argsJSON)
	if argsJSON == "" {
		argsJSON = "{}"
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return Result{OK: false, Error: "invalid tool arguments JSON: " + err.Error()}
	}
	tool, ok := Get(name)
	if !ok {
		return Result{OK: false, Error: "unknown tool: " + name}
	}
	res := tool.Execute(ec, args)
	if res.OK && tool.Meta().Risk != RiskRead {
		recordToolAudit(ec, tool.Meta(), args, res)
	}
	return res
}

func recordToolAudit(ec ExecContext, meta ToolMeta, args map[string]any, res Result) {
	db := config.GetDB()
	if db == nil {
		return
	}
	detail, err := json.Marshal(map[string]any{
		"session_id":   ec.SessionID,
		"tool_call_id": ec.ToolCallID,
		"args":         args,
	})
	if err != nil {
		return
	}
	actorID := ec.User.ID
	audit.RecordAudit(db, audit.AuditEntry{
		ActorID:      &actorID,
		ActorName:    ec.User.Username,
		Action:       "chat_tool_" + meta.Name,
		ResourceType: "chat_tool",
		ResourceID:   resultResourceID(res),
		Method:       "POST",
		Path:         "/api/chat/tool",
		StatusCode:   200,
		Summary:      res.Summary,
		Detail:       string(detail),
	})
}

func resultResourceID(res Result) string {
	m, ok := res.Result.(map[string]any)
	if !ok {
		return ""
	}
	if id, ok := m["id"]; ok && id != nil {
		return fmt.Sprint(id)
	}
	return ""
}
