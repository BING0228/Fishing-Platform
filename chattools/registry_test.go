package chattools

import (
	"encoding/json"
	"testing"
)

func TestRegistryNoDuplicateNames(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range All() {
		name := tool.Meta().Name
		if seen[name] {
			t.Fatalf("duplicate tool name %q", name)
		}
		seen[name] = true
	}
	if len(seen) == 0 {
		t.Fatal("expected registered tools")
	}
}

func TestRegistrySchemasAreValidObjects(t *testing.T) {
	for _, tool := range All() {
		meta := tool.Meta()
		if len(meta.Parameters) == 0 {
			t.Fatalf("%s has no parameters schema", meta.Name)
		}
		var parsed map[string]any
		if err := json.Unmarshal(meta.Parameters, &parsed); err != nil {
			t.Fatalf("%s schema invalid: %v", meta.Name, err)
		}
		if parsed["type"] != "object" {
			t.Fatalf("%s schema type = %v, want object", meta.Name, parsed["type"])
		}
		if v, ok := parsed["additionalProperties"]; !ok || v != false {
			t.Fatalf("%s schema must set additionalProperties=false", meta.Name)
		}
		if meta.Description == "" {
			t.Fatalf("%s missing description", meta.Name)
		}
	}
}

func TestExpectedToolsRegistered(t *testing.T) {
	required := []string{
		"list_agents", "list_projects", "get_project", "clone_page", "create_project",
		"list_smtp", "list_robots",
		"list_pages", "get_page", "start_info_gather", "get_info_gather", "list_info_findings",
		"create_mail_draft", "send_mail_campaign", "bind_robot", "project_action",
		"update_project", "build_project", "get_project_logs",
		"list_info_gather_jobs", "list_mail_campaigns", "get_mail_campaign",
		"list_mail_templates", "list_qr_relays", "list_ip_blacklist",
		"create_qr_relay", "get_qr_relay", "list_credentials", "add_ip_blacklist",
		"preview_mail", "export_credentials", "list_project_hooks",
	}
	for _, name := range required {
		if _, ok := Get(name); !ok {
			t.Fatalf("tool %q not registered", name)
		}
	}
}

func TestDefinitionsExcludeHiddenAndCoverRegistered(t *testing.T) {
	defs := DefinitionsForLLM()
	names := map[string]bool{}
	for _, d := range defs {
		names[d.Function.Name] = true
	}
	for _, tool := range All() {
		meta := tool.Meta()
		if meta.Hidden {
			if names[meta.Name] {
				t.Fatalf("hidden tool %q leaked into LLM definitions", meta.Name)
			}
			continue
		}
		if !names[meta.Name] {
			t.Fatalf("tool %q missing from LLM definitions", meta.Name)
		}
	}
}

func TestClonePageAliasExecutes(t *testing.T) {
	// The alias must resolve to clone_page; validation happens before any I/O.
	ec := ExecContext{}
	res := Execute(ec, "create_phishing_page", `{"url":""}`)
	if res.OK {
		t.Fatal("empty url should fail")
	}
	if res.Error == "unknown tool: create_phishing_page" {
		t.Fatal("alias should resolve to clone_page")
	}
}

func TestUnknownTool(t *testing.T) {
	res := Execute(ExecContext{}, "nope", `{}`)
	if res.OK || res.Error == "" {
		t.Fatalf("expected unknown tool error, got %+v", res)
	}
}

func TestInvalidArgsJSON(t *testing.T) {
	res := Execute(ExecContext{}, "list_agents", `{bad`)
	if res.OK || res.Error == "" {
		t.Fatalf("expected invalid JSON error, got %+v", res)
	}
}
