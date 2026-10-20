package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

const (
	directQRMarker   = "<!--spearhook:direct-qr-->"
	maxQRImageBytes  = 3 << 20
	maxQRImageChecks = 8
)

var (
	imgTagRe     = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	imgAttrRe    = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	qrHintTokens = []string{"qr", "qrcode", "qr-code", "二维码", "扫码", "scan", "wechat", "weixin", "ewm"}
)

// QRCodeContent decodes a standard QR image and returns its text.
func QRCodeContent(data []byte) (string, bool) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", false
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", false
	}
	result, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil || result == nil {
		return "", false
	}
	text := strings.TrimSpace(result.GetText())
	return text, text != ""
}

// QRCodePageHTML is the minimal landing page used when a cloned URL is itself
// a QR image. The project build replaces the placeholder with the bound relay.
func QRCodePageHTML() string {
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<title>QR Login</title><style>html,body{margin:0;min-height:100%;background:#0b1220}` +
		`body{display:grid;place-items:center;font-family:ui-sans-serif,system-ui,-apple-system,"Segoe UI",Arial,"Noto Sans SC",sans-serif}` +
		`main{width:min(420px,92vw);padding:28px;border-radius:20px;background:#fff;text-align:center}` +
		`main img{display:block;width:280px;height:280px;margin:0 auto 18px}` +
		`p{margin:0;color:#475569;font-size:14px}</style></head>` +
		`<body><main>{{QR_RELAY_IMG}}<p>请扫码继续</p></main>` + directQRMarker + `</body></html>`
}

// IsDirectQRPage reports whether HTML came from a directly-cloned QR image and
// must bypass AI rewriting.
func IsDirectQRPage(htmlText string) bool {
	return strings.Contains(htmlText, directQRMarker)
}

// StripDirectQRMarker removes the internal marker before persisting the page.
func StripDirectQRMarker(htmlText string) string {
	return strings.ReplaceAll(htmlText, directQRMarker, "")
}

// ReplaceQRCodeImages replaces QR-code <img> elements with the live relay
// placeholder. It only inspects likely candidates (QR-ish attributes or square
// dimensions) and verifies with a real QR decoder.
func ReplaceQRCodeImages(ctx context.Context, htmlText, basePageURL string) string {
	if !strings.Contains(strings.ToLower(htmlText), "<img") {
		return htmlText
	}
	base, _ := url.Parse(strings.TrimSpace(basePageURL))
	attempts := 0
	return imgTagRe.ReplaceAllStringFunc(htmlText, func(tag string) string {
		if attempts >= maxQRImageChecks {
			return tag
		}
		attrs := parseImgAttrs(tag)
		src := strings.TrimSpace(attrs["src"])
		if src == "" {
			return tag
		}
		hinted := hasQRHint(attrs, src)
		dataURI := strings.HasPrefix(strings.ToLower(src), "data:image/")
		square := squareImageAttrs(attrs)
		if !hinted && !dataURI && !square {
			return tag
		}
		attempts++
		data, ok := fetchQRImage(ctx, src, base)
		if !ok {
			return tag
		}
		if _, ok := QRCodeContent(data); !ok {
			return tag
		}
		return PlaceholderQRRelayImg
	})
}

func parseImgAttrs(tag string) map[string]string {
	out := map[string]string{}
	for _, m := range imgAttrRe.FindAllStringSubmatch(tag, -1) {
		if len(m) < 5 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(m[1]))
		val := m[2]
		if val == "" {
			val = m[3]
		}
		if val == "" {
			val = m[4]
		}
		out[key] = html.UnescapeString(strings.TrimSpace(val))
	}
	return out
}

func hasQRHint(attrs map[string]string, src string) bool {
	joined := strings.ToLower(strings.Join([]string{
		attrs["alt"], attrs["class"], attrs["id"], attrs["name"], attrs["title"], src,
	}, " "))
	for _, token := range qrHintTokens {
		if strings.Contains(joined, token) {
			return true
		}
	}
	return false
}

func squareImageAttrs(attrs map[string]string) bool {
	w, errW := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(attrs["width"]), "px"))
	h, errH := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(attrs["height"]), "px"))
	if errW != nil || errH != nil || w < 120 || h < 120 {
		return false
	}
	ratio := float64(w) / float64(h)
	return ratio >= 0.8 && ratio <= 1.25
}

func fetchQRImage(ctx context.Context, rawURL string, base *url.URL) ([]byte, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, false
	}
	if strings.HasPrefix(strings.ToLower(rawURL), "data:image/") {
		idx := strings.Index(rawURL, ",")
		if idx < 0 || !strings.Contains(strings.ToLower(rawURL[:idx]), ";base64") {
			return nil, false
		}
		data, err := base64.StdEncoding.DecodeString(rawURL[idx+1:])
		if err != nil || len(data) == 0 || len(data) > maxQRImageBytes {
			return nil, false
		}
		return data, true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, false
	}
	if base != nil {
		parsed = base.ResolveReference(parsed)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FishingPlatformMirror/1.0)")
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/gif,*/*;q=0.8")
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxQRImageBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxQRImageBytes {
		return nil, false
	}
	if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "image/") {
		if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
			return nil, false
		}
	}
	return data, true
}

func qrImageSummary(data []byte) string {
	text, ok := QRCodeContent(data)
	if !ok {
		return ""
	}
	return fmt.Sprintf("QR image (%s)", truncateForErr(text, 80))
}

func injectQRPlaceholder(htmlText string) string {
	block := `<div style="text-align:center;margin:24px 0">{{QR_RELAY_IMG}}</div>`
	if idx := strings.LastIndex(strings.ToLower(htmlText), "</body>"); idx >= 0 {
		return htmlText[:idx] + block + htmlText[idx:]
	}
	return htmlText + block
}

// NormalizeQRPlaceholderUsage fixes the common LLM mistake of putting the
// element-level {{QR_RELAY_IMG}} token inside an img src attribute.
func NormalizeQRPlaceholderUsage(htmlText string) string {
	for _, q := range []string{`"`, `'`} {
		htmlText = strings.ReplaceAll(htmlText, `src=`+q+`{{QR_RELAY_IMG}}`+q, `src=`+q+`{{QR_RELAY_URL}}`+q)
	}
	return strings.ReplaceAll(htmlText, `src={{QR_RELAY_IMG}}`, `src={{QR_RELAY_URL}}`)
}
