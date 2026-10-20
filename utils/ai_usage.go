package utils

import "context"

// AITokenUsage is one provider usage record. Prompt/Completion are input/output
// tokens; Total falls back to their sum when the provider omits it.
type AITokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// aiUsageJSON matches OpenAI chat and Responses usage shapes.
type aiUsageJSON struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (u aiUsageJSON) normalized() AITokenUsage {
	prompt := u.PromptTokens
	if prompt == 0 {
		prompt = u.InputTokens
	}
	completion := u.CompletionTokens
	if completion == 0 {
		completion = u.OutputTokens
	}
	total := u.TotalTokens
	if total == 0 {
		total = prompt + completion
	}
	if prompt < 0 {
		prompt = 0
	}
	if completion < 0 {
		completion = 0
	}
	if total < 0 {
		total = 0
	}
	return AITokenUsage{PromptTokens: prompt, CompletionTokens: completion, TotalTokens: total}
}

type aiProfileIDContextKey struct{}

// WithAIProfileID attaches the AI profile id to a request context so low-level
// LLM helpers can attribute token usage without changing every signature.
func WithAIProfileID(ctx context.Context, profileID string) context.Context {
	if ctx == nil || profileID == "" {
		return ctx
	}
	return context.WithValue(ctx, aiProfileIDContextKey{}, profileID)
}

// AIProfileIDFromContext returns the profile id attached by WithAIProfileID.
func AIProfileIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(aiProfileIDContextKey{}).(string)
	return id
}

var aiUsageRecorder func(profileID string, usage AITokenUsage)

// SetAIUsageRecorder wires the persistence callback. platform.init sets this to
// RecordAIUsage; tests may replace it with a collector.
func SetAIUsageRecorder(fn func(profileID string, usage AITokenUsage)) {
	aiUsageRecorder = fn
}

// recordAIUsage attributes one provider call to the context's profile.
func recordAIUsage(ctx context.Context, usage AITokenUsage) {
	if usage.TotalTokens <= 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if usage.TotalTokens <= 0 {
		return
	}
	profileID := AIProfileIDFromContext(ctx)
	if profileID == "" || aiUsageRecorder == nil {
		return
	}
	aiUsageRecorder(profileID, usage)
}
