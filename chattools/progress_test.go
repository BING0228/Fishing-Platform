package chattools

import "testing"

func TestProgressEventShape(t *testing.T) {
	type emitted struct {
		name    string
		payload map[string]any
	}
	var got []emitted
	ec := ExecContext{
		ToolCallID: "call_1",
		Emit: func(name string, payload any) {
			m, _ := payload.(map[string]any)
			got = append(got, emitted{name: name, payload: m})
		},
	}
	ec.Progress("gather", 5, "queued")

	if len(got) != 1 {
		t.Fatalf("expected one event, got %d", len(got))
	}
	if got[0].name != "progress" {
		t.Fatalf("event = %q", got[0].name)
	}
	if got[0].payload["tool_call_id"] != "call_1" {
		t.Fatalf("tool_call_id = %v", got[0].payload["tool_call_id"])
	}
	if got[0].payload["percent"] != 5 || got[0].payload["stage"] != "gather" {
		t.Fatalf("payload = %+v", got[0].payload)
	}
}

func TestProgressNoEmitterIsSafe(t *testing.T) {
	ExecContext{}.Progress("clone", 50, "no emitter") // must not panic
}
