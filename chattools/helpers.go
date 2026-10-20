package chattools

import (
	"encoding/json"
	"fmt"
	"strings"
)

func jsonString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return ""
	}
}

func jsonBool(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func jsonInt(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int:
		return int64(t), true
	case int64:
		return t, true
	case json.Number:
		i, err := t.Int64()
		return i, err == nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		var n int64
		_, err := fmt.Sscanf(s, "%d", &n)
		return n, err == nil
	default:
		return 0, false
	}
}

// jsonUintPtr returns a pointer to a positive id, or nil when absent/zero.
func jsonUintPtr(v any) *uint {
	n, ok := jsonInt(v)
	if !ok || n <= 0 {
		return nil
	}
	id := uint(n)
	return &id
}

func jsonUintSlice(v any) []uint {
	arr, ok := v.([]any)
	if !ok {
		switch typed := v.(type) {
		case []uint:
			return typed
		case []int:
			out := make([]uint, 0, len(typed))
			for _, item := range typed {
				if item > 0 {
					out = append(out, uint(item))
				}
			}
			return out
		}
		return nil
	}
	out := make([]uint, 0, len(arr))
	for _, item := range arr {
		if n, ok := jsonInt(item); ok && n > 0 {
			out = append(out, uint(n))
		}
	}
	return out
}

func jsonStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		if typed, ok := v.([]string); ok {
			return typed
		}
		if s := jsonString(v); s != "" {
			return []string{s}
		}
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s := jsonString(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
