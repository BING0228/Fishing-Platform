package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

var renderSem = make(chan struct{}, 1)

const renderCollectJS = `(() => {
  const items = [];
  document.querySelectorAll('canvas').forEach((c, i) => {
    let hints = [];
    let el = c;
    for (let d = 0; d < 5 && el; d++, el = el.parentElement) {
      hints.push(el.className || '', el.id || '', el.getAttribute('aria-label') || '');
    }
    let data = '';
    try { data = c.toDataURL('image/png'); } catch (e) {}
    const r = c.getBoundingClientRect();
    const st = getComputedStyle(c);
    const visible = r.width > 0 && r.height > 0 && r.bottom > 0 && r.right > 0 &&
      r.top < innerHeight && r.left < innerWidth &&
      st.display !== 'none' && st.visibility !== 'hidden' && parseFloat(st.opacity) > 0.1;
    items.push({
      i: i,
      x: r.left,
      y: r.top,
      w: r.width || c.width || c.clientWidth || 0,
      h: r.height || c.height || c.clientHeight || 0,
      visible: visible,
      hint: hints.join(' ').toLowerCase(),
      data: data
    });
  });
  return JSON.stringify(items);
})()`

const renderReplaceJS = `(() => {
  const idx = new Set(__INDEX__);
  document.querySelectorAll('canvas').forEach((c, i) => {
    if (!idx.has(i)) return;
    const img = document.createElement('img');
    img.setAttribute('src', '{{QR_RELAY_URL}}');
    img.setAttribute('alt', 'QR code');
    const w = c.width || c.clientWidth || 180;
    const h = c.height || c.clientHeight || 180;
    img.setAttribute('width', String(w));
    img.setAttribute('height', String(h));
    if (c.className) img.setAttribute('class', c.className);
    const style = c.getAttribute('style');
    if (style) img.setAttribute('style', style);
    c.replaceWith(img);
  });
  document.querySelectorAll('script').forEach((s) => s.remove());
  return document.documentElement.outerHTML;
})()`

type renderedCanvas struct {
	Index   int     `json:"i"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	Visible bool    `json:"visible"`
	Hint    string  `json:"hint"`
	Data    string  `json:"data"`
}

const (
	renderViewportW    = 1440.0
	renderViewportH    = 900.0
	qrScreenshotMarker = "<!--spearhook:qr-screenshot-->"
)

// FetchURLHTMLForClone uses the fast static fetch first and falls back to a
// headless Chromium render for SPA pages / URLs with client-side fragments.
func FetchURLHTMLForClone(ctx context.Context, rawURL string, timeout time.Duration) (string, *url.URL, error) {
	htmlText, finalURL, err := FetchURLHTML(ctx, rawURL, timeout)
	if err != nil {
		return "", nil, err
	}
	if !ShouldRenderClone(rawURL, htmlText) {
		return htmlText, finalURL, nil
	}
	rendered, renderedURL, renderErr := FetchURLHTMLRendered(ctx, rawURL, timeout)
	if renderErr != nil {
		return "", nil, fmt.Errorf("page needs JS rendering but headless Chromium failed: %w", renderErr)
	}
	if strings.TrimSpace(renderedURL) != "" {
		if u, perr := url.Parse(renderedURL); perr == nil {
			finalURL = u
		}
	}
	return rendered, finalURL, nil
}

// ShouldRenderClone reports whether a static clone is insufficient.
func ShouldRenderClone(rawURL, htmlText string) bool {
	if IsDirectQRPage(htmlText) {
		return false
	}
	if strings.Contains(htmlText, PlaceholderQRRelayImg) || strings.Contains(htmlText, PlaceholderQRRelayURL) {
		return false
	}
	if u, err := url.Parse(strings.TrimSpace(rawURL)); err == nil && strings.TrimSpace(u.Fragment) != "" {
		return true
	}
	lower := strings.ToLower(htmlText)
	for _, marker := range []string{"<canvas", `id="root"`, `id="app"`, `id="container"`, "react", "vue", "ng-app"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// FetchURLHTMLRendered renders the page in headless Chromium, replaces QR
// canvases with the relay URL placeholder, and removes scripts so the frozen
// DOM cannot re-render the original QR.
func FetchURLHTMLRendered(ctx context.Context, rawURL string, timeout time.Duration) (string, string, error) {
	chrome := chromeExecPath()
	if chrome == "" {
		return "", "", fmt.Errorf("Chromium/Chrome not found (set CHROME_PATH or install chromium)")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	select {
	case renderSem <- struct{}{}:
		defer func() { <-renderSem }()
	case <-ctx.Done():
		return "", "", ctx.Err()
	}

	allocOpts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	allocOpts = append(allocOpts,
		chromedp.ExecPath(chrome),
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.WindowSize(1440, 900),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer allocCancel()
	// Newer Chrome versions emit network event fields that older cdproto
	// releases cannot decode; these are noise and do not affect the DOM.
	runCtx, runCancel := chromedp.NewContext(allocCtx, chromedp.WithErrorf(func(string, ...interface{}) {}))
	defer runCancel()
	runCtx, timeoutCancel := context.WithTimeout(runCtx, timeout)
	defer timeoutCancel()

	var location string
	var canvasJSON string
	if err := chromedp.Run(runCtx,
		chromedp.Navigate(rawURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(4*time.Second),
		chromedp.Location(&location),
		chromedp.Evaluate(renderCollectJS, &canvasJSON),
	); err != nil {
		return "", "", err
	}

	var canvases []renderedCanvas
	_ = json.Unmarshal([]byte(canvasJSON), &canvases)
	indices := make([]int, 0, len(canvases))
	var qrCanvas *renderedCanvas
	for i := range canvases {
		item := canvases[i]
		if isQRCanvasHint(item.Hint) || canvasHasQR(item.Data) {
			indices = append(indices, item.Index)
			if qrCanvas == nil || (canvases[i].Visible && !qrCanvas.Visible) {
				qrCanvas = &canvases[i]
			}
		}
	}
	if qrCanvas != nil && qrCanvas.W > 0 && qrCanvas.H > 0 {
		var shot []byte
		if err := chromedp.Run(runCtx, chromedp.CaptureScreenshot(&shot)); err == nil && len(shot) > 0 {
			var viewportW, viewportH float64
			_ = chromedp.Run(runCtx,
				chromedp.Evaluate(`window.innerWidth`, &viewportW),
				chromedp.Evaluate(`window.innerHeight`, &viewportH),
			)
			if page, err := buildQRScreenshotPage(shot, *qrCanvas, viewportW, viewportH); err == nil {
				return page, location, nil
			}
		}
	}
	idxJSON, _ := json.Marshal(indices)
	replaceJS := strings.Replace(renderReplaceJS, "__INDEX__", string(idxJSON), 1)

	var htmlText string
	if err := chromedp.Run(runCtx, chromedp.Evaluate(replaceJS, &htmlText)); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(htmlText) == "" {
		return "", "", fmt.Errorf("rendered page is empty")
	}
	return htmlText, location, nil
}

// buildQRScreenshotPage freezes the original visual layout as a JPEG and
// covers the original QR with a live relay placeholder. This avoids broken
// absolute-position layouts when an SPA's scripts are removed.
func buildQRScreenshotPage(shotPNG []byte, canvas renderedCanvas, viewportW, viewportH float64) (string, error) {
	src, _, err := image.Decode(bytes.NewReader(shotPNG))
	if err != nil {
		return "", err
	}
	b := src.Bounds()
	if viewportW <= 0 {
		viewportW = float64(b.Dx())
	}
	if viewportH <= 0 {
		viewportH = float64(b.Dy())
	}
	img := image.NewRGBA(b)
	draw.Draw(img, b, src, b.Min, draw.Src)
	scaleX := float64(b.Dx()) / viewportW
	scaleY := float64(b.Dy()) / viewportH
	padCSS := 8.0
	x := int((canvas.X - padCSS) * scaleX)
	y := int((canvas.Y - padCSS) * scaleY)
	w := int((canvas.W + 2*padCSS) * scaleX)
	h := int((canvas.H + 2*padCSS) * scaleY)
	mask := image.Rect(x, y, x+w, y+h).Intersect(b)
	if mask.Dx() > 0 && mask.Dy() > 0 {
		draw.Draw(img, mask, image.White, image.Point{}, draw.Src)
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, &jpeg.Options{Quality: 82}); err != nil {
		return "", err
	}
	left := (canvas.X - padCSS) / viewportW * 100
	top := (canvas.Y - padCSS) / viewportH * 100
	width := (canvas.W + 2*padCSS) / viewportW * 100
	height := (canvas.H + 2*padCSS) / viewportH * 100
	dataURI := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpg.Bytes())
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<title>QR Login</title><style>html,body{margin:0;height:100%;background:#000;overflow:hidden}` +
		`.stage{position:relative;width:100vw;height:100vh;overflow:hidden;background:#000}` +
		`.bg{position:absolute;inset:0;width:100%;height:100%;object-fit:fill}` +
		`.qr{position:absolute;left:` + fmt.Sprintf("%.4f", left) + `%;top:` + fmt.Sprintf("%.4f", top) + `%;` +
		`width:` + fmt.Sprintf("%.4f", width) + `%;height:` + fmt.Sprintf("%.4f", height) + `%;background:#fff;` +
		`display:flex;align-items:center;justify-content:center;box-sizing:border-box}` +
		`.qr img{display:block;width:100%;height:100%;object-fit:contain}</style></head>` +
		`<body><div class="stage"><img class="bg" src="` + dataURI + `" alt="">` +
		`<div class="qr"><img src="{{QR_RELAY_URL}}" alt="QR code"></div></div>` +
		qrScreenshotMarker + `</body></html>`, nil
}

// IsQRScreenshotPage reports whether a cloned page is a frozen screenshot with
// a live relay QR overlay.
func IsQRScreenshotPage(htmlText string) bool {
	return strings.Contains(htmlText, qrScreenshotMarker)
}

// StripQRScreenshotMarker removes the internal marker before persisting.
func StripQRScreenshotMarker(htmlText string) string {
	return strings.ReplaceAll(htmlText, qrScreenshotMarker, "")
}

func canvasHasQR(dataURI string) bool {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURI, prefix) {
		return false
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURI, prefix))
	if err != nil || len(data) == 0 {
		return false
	}
	_, ok := QRCodeContent(data)
	return ok
}

func isQRCanvasHint(hint string) bool {
	for _, token := range qrHintTokens {
		if strings.Contains(hint, token) {
			return true
		}
	}
	return false
}

func chromeExecPath() string {
	if p := strings.TrimSpace(os.Getenv("CHROME_PATH")); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, p := range []string{
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/usr/bin/google-chrome",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
