package chattools

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"spearhook/utils"
)

var (
	regMu    sync.RWMutex
	registry = map[string]Tool{}
)

// Register adds a tool. Duplicate names panic at init time so the process never
// starts with an ambiguous tool surface.
func Register(t Tool) {
	if t == nil {
		panic("chattools: nil tool")
	}
	meta := t.Meta()
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		panic("chattools: empty tool name")
	}
	if len(meta.Parameters) > 0 && !json.Valid(meta.Parameters) {
		panic("chattools: invalid JSON schema for tool " + name)
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, exists := registry[name]; exists {
		panic("chattools: duplicate tool " + name)
	}
	registry[name] = t
}

// Get returns a registered tool by name.
func Get(name string) (Tool, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	t, ok := registry[strings.TrimSpace(name)]
	return t, ok
}

// All returns every registered tool sorted by name.
func All() []Tool {
	regMu.RLock()
	defer regMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Tool, 0, len(names))
	for _, name := range names {
		out = append(out, registry[name])
	}
	return out
}

// DefinitionsForLLM converts visible tools into OpenAI tools[] entries.
func DefinitionsForLLM() []utils.ChatTool {
	out := make([]utils.ChatTool, 0, len(registry))
	for _, t := range All() {
		meta := t.Meta()
		if meta.Hidden {
			continue
		}
		out = append(out, utils.ChatTool{
			Type: "function",
			Function: utils.ChatToolFunction{
				Name:        meta.Name,
				Description: meta.Description,
				Parameters:  meta.Parameters,
			},
		})
	}
	return out
}

// RequiresConfirm reports whether a tool must pause for user approval.
func RequiresConfirm(name string, policy ConfirmPolicy) bool {
	t, ok := Get(name)
	if !ok {
		return false
	}
	for _, forced := range policy.AlwaysConfirm {
		if strings.TrimSpace(forced) == strings.TrimSpace(name) {
			return true
		}
	}
	switch t.Meta().Risk {
	case RiskDestructive:
		return policy.ConfirmDestructive
	case RiskWrite:
		return policy.ConfirmWrites
	default:
		return false
	}
}

// schema builds an object JSON Schema with additionalProperties=false.
func schema(props map[string]any, required []string) json.RawMessage {
	if props == nil {
		props = map[string]any{}
	}
	obj := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		obj["required"] = required
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		panic("chattools: marshal schema: " + err.Error())
	}
	return raw
}

func newTool(name, description string, risk Risk, params json.RawMessage, confirmMessage string, run func(ExecContext, map[string]any) Result) funcTool {
	return funcTool{
		meta: ToolMeta{
			Name:           name,
			Description:    description,
			Risk:           risk,
			Parameters:     params,
			ConfirmMessage: confirmMessage,
		},
		run: run,
	}
}
