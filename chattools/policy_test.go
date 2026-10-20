package chattools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRequiresConfirmByRisk(t *testing.T) {
	policy := DefaultConfirmPolicy
	if RequiresConfirm("list_agents", policy) {
		t.Fatal("read tools never require confirmation")
	}
	if RequiresConfirm("clone_page", policy) {
		t.Fatal("write tools default to no confirmation")
	}
	if !RequiresConfirm("send_mail_campaign", policy) {
		t.Fatal("send_mail_campaign must always confirm")
	}
}

func TestRequiresConfirmWritesConfigurable(t *testing.T) {
	policy := ConfirmPolicy{ConfirmWrites: true, ConfirmDestructive: true}
	if !RequiresConfirm("clone_page", policy) {
		t.Fatal("ConfirmWrites should gate write tools")
	}
	if RequiresConfirm("list_projects", policy) {
		t.Fatal("read tools stay automatic")
	}
}

func TestRequiresConfirmUnknownTool(t *testing.T) {
	if RequiresConfirm("does_not_exist", DefaultConfirmPolicy) {
		t.Fatal("unknown tools do not confirm (they fail)")
	}
}

func TestSystemPromptListsTools(t *testing.T) {
	prompt := SystemPrompt()
	for _, name := range []string{
		"clone_page", "create_project", "start_info_gather",
		"create_mail_draft", "send_mail_campaign", "bind_robot",
		"project_action", "update_project", "build_project", "get_project_logs",
		"list_info_gather_jobs", "list_mail_campaigns", "get_mail_campaign",
		"list_mail_templates", "list_qr_relays", "list_ip_blacklist",
		"create_qr_relay", "list_credentials", "add_ip_blacklist",
		"preview_mail", "export_credentials", "list_project_hooks",
	} {
		if !strings.Contains(prompt, name) {
			t.Fatalf("system prompt missing tool %q", name)
		}
	}
	if !strings.Contains(prompt, "Never invent") && !strings.Contains(prompt, "no inventing") {
		t.Fatal("system prompt must forbid invented HTML")
	}
}

func TestPlaybooksPresent(t *testing.T) {
	if len(Playbooks) != 3 {
		t.Fatalf("expected 3 playbooks, got %d", len(Playbooks))
	}
	seen := map[string]bool{}
	for _, p := range Playbooks {
		if p.ID == "" || p.TitleZH == "" || p.PromptZH == "" || p.PromptEN == "" {
			t.Fatalf("playbook missing fields: %+v", p)
		}
		if seen[p.ID] {
			t.Fatalf("duplicate playbook id %q", p.ID)
		}
		seen[p.ID] = true
		if len(p.ToolHints) == 0 {
			t.Fatalf("playbook %q missing tool hints", p.ID)
		}
		for _, name := range p.ToolHints {
			if _, ok := Get(name); !ok {
				t.Fatalf("playbook %q references unregistered tool %q", p.ID, name)
			}
		}
	}
}

func TestEphemeralNotPersisted(t *testing.T) {
	res := Result{
		OK:        true,
		Summary:   "created",
		Ephemeral: map[string]any{"upload_token": "sekret-token"},
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sekret-token") {
		t.Fatalf("ephemeral secret leaked into persisted result: %s", raw)
	}
}
