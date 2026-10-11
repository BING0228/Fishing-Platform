package utils

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Landing-page placeholders replaced when HTML is prepared for a project runtime.
const (
	PlaceholderSubmitURL   = "{{SUBMIT_URL}}"
	PlaceholderRedirectURL = "{{REDIRECT_URL}}"
	PlaceholderQRRelayURL  = "{{QR_RELAY_URL}}"
	PlaceholderQRRelayImg  = "{{QR_RELAY_IMG}}"
)

// ProjectHTMLVars are values substituted into uploaded/mirrored landing HTML.
type ProjectHTMLVars struct {
	SubmitURL   string
	RedirectURL string
	QRRelayURL  string // absolute or path URL to live QR image
}

// ApplyProjectHTMLPlaceholders replaces supported tokens in landing HTML.
func ApplyProjectHTMLPlaceholders(html string, vars ProjectHTMLVars) string {
	submit := strings.TrimSpace(vars.SubmitURL)
	if submit == "" {
		submit = "/api/submit"
	}
	redirect := strings.TrimSpace(vars.RedirectURL)
	qrURL := strings.TrimSpace(vars.QRRelayURL)
	qrImg := ""
	if qrURL != "" {
		qrImg = fmt.Sprintf(`<img src="%s" alt="qr" width="240" height="240" />`, qrURL)
	}

	out := PrepareLandingHTML(html)
	out = strings.ReplaceAll(out, PlaceholderSubmitURL, submit)
	out = strings.ReplaceAll(out, PlaceholderRedirectURL, redirect)
	out = strings.ReplaceAll(out, PlaceholderQRRelayURL, qrURL)
	out = strings.ReplaceAll(out, PlaceholderQRRelayImg, qrImg)
	out = RewriteQuotedSubmitPaths(out, submit)
	return out
}

// PrepareLandingHTML keeps landing HTML in library form: no file:// base,
// login forms/actions use {{SUBMIT_URL}} so project build can substitute.
func PrepareLandingHTML(html string) string {
	return NormalizeRewrittenLoginHTML(html, PlaceholderSubmitURL)
}

// RenderProjectHTMLFile reads a source HTML file and applies project placeholders.
func RenderProjectHTMLFile(srcPath string, vars ProjectHTMLVars) ([]byte, error) {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	return []byte(ApplyProjectHTMLPlaceholders(string(raw), vars)), nil
}

// IsAbsoluteHTTPURL reports whether value is an http(s) URL with a host.
func IsAbsoluteHTTPURL(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	return err == nil && u != nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// AbsoluteHTTPURL turns a path like /api/submit into a full URL using base.
// If raw is already http(s), it is returned unchanged.
func AbsoluteHTTPURL(raw, base string) string {
	raw = strings.TrimSpace(raw)
	if parsed, err := url.Parse(raw); err == nil && parsed != nil {
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https":
			if parsed.Host != "" {
				return strings.TrimRight(raw, "/")
			}
		case "file", "blob", "about", "javascript", "data":
			raw = ""
		}
	}
	if raw == "" {
		raw = DefaultMirrorSubmitURL
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if !IsAbsoluteHTTPURL(base) {
		return raw
	}
	if strings.HasPrefix(raw, "/") {
		return base + raw
	}
	return base + "/" + raw
}

// LandingSubmitToken is the token written into page-library HTML.
// Project build/start replaces {{SUBMIT_URL}} with the running container route.
func LandingSubmitToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == DefaultMirrorSubmitURL {
		return PlaceholderSubmitURL
	}
	return raw
}

// LandingRedirectToken is the token written into page-library HTML.
// Project build/start replaces {{REDIRECT_URL}} with the project's original URL.
func LandingRedirectToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return PlaceholderRedirectURL
	}
	return raw
}
