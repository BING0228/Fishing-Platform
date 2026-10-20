package chattools

import (
	"os"
	"strings"
	"testing"
)

// TestRegisteredToolsDocumented keeps the architecture doc's tool tables in sync
// with the registry. If you add a tool, document it in
// docs/ai-native-architecture.md §5.
func TestRegisteredToolsDocumented(t *testing.T) {
	raw, err := os.ReadFile("../docs/ai-native-architecture.md")
	if err != nil {
		t.Skipf("architecture doc not available: %v", err)
	}
	doc := string(raw)
	for _, tool := range All() {
		meta := tool.Meta()
		if meta.Hidden {
			continue
		}
		if !strings.Contains(doc, meta.Name) {
			t.Fatalf("tool %q is registered but missing from docs/ai-native-architecture.md", meta.Name)
		}
	}
}
