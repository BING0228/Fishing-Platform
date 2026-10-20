package utils

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	htmlAssetAttrRe = regexp.MustCompile(`(?i)(\b(?:href|src|poster|data-src|data-original)\s*=\s*)(["'])([^"']*)(["'])`)
	cssURLFuncRe = regexp.MustCompile(`(?i)url\(\s*(?:'([^']+)'|"([^"]+)"|([^)]+))\s*\)`)
)

// AbsolutizeHTMLAssets rewrites root-relative and path-relative asset URLs
// (CSS/JS/images/etc.) to absolute URLs against basePageURL so mirrored pages
// keep loading styles when served from Spearhook instead of the origin host.
//
// Leaves alone: http(s), protocol-relative //, data:, javascript:, mailto:,
// blob:, hash-only, empty, and {{TOKEN}} placeholders (submit/redirect).
func AbsolutizeHTMLAssets(html, basePageURL string) string {
	basePageURL = strings.TrimSpace(basePageURL)
	if html == "" || basePageURL == "" {
		return html
	}
	base, err := url.Parse(basePageURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return html
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return html
	}

	html = htmlAssetAttrRe.ReplaceAllStringFunc(html, func(m string) string {
		sub := htmlAssetAttrRe.FindStringSubmatch(m)
		if len(sub) != 5 {
			return m
		}
		prefix, q, raw, q2 := sub[1], sub[2], sub[3], sub[4]
		abs, ok := absolutizeRef(raw, base)
		if !ok {
			return m
		}
		return prefix + q + abs + q2
	})

	html = cssURLFuncRe.ReplaceAllStringFunc(html, func(m string) string {
		sub := cssURLFuncRe.FindStringSubmatch(m)
		if len(sub) != 4 {
			return m
		}
		raw, quote := "", ""
		switch {
		case sub[1] != "":
			raw, quote = sub[1], "'"
		case sub[2] != "":
			raw, quote = sub[2], `"`
		default:
			raw = strings.TrimSpace(sub[3])
		}
		abs, ok := absolutizeRef(raw, base)
		if !ok {
			return m
		}
		if quote == "" {
			return "url(" + abs + ")"
		}
		return "url(" + quote + abs + quote + ")"
	})

	return html
}

func absolutizeRef(raw string, base *url.URL) (string, bool) {
	ref := strings.TrimSpace(raw)
	if ref == "" {
		return "", false
	}
	low := strings.ToLower(ref)
	switch {
	case strings.HasPrefix(ref, "{{") && strings.HasSuffix(ref, "}}"):
		return "", false
	case strings.Contains(ref, "{{"):
		return "", false
	case strings.HasPrefix(low, "data:"):
		return "", false
	case strings.HasPrefix(low, "javascript:"):
		return "", false
	case strings.HasPrefix(low, "mailto:"):
		return "", false
	case strings.HasPrefix(low, "tel:"):
		return "", false
	case strings.HasPrefix(low, "blob:"):
		return "", false
	case strings.HasPrefix(low, "http://"), strings.HasPrefix(low, "https://"):
		return "", false
	case strings.HasPrefix(ref, "//"):
		// protocol-relative → pin to base scheme
		return base.Scheme + ":" + ref, true
	case ref == "#" || strings.HasPrefix(ref, "#"):
		return "", false
	}

	u, err := url.Parse(ref)
	if err != nil {
		return "", false
	}
	abs := base.ResolveReference(u)
	if abs.Scheme == "" || abs.Host == "" {
		return "", false
	}
	return abs.String(), true
}
