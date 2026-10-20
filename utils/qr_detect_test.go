package utils

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testQRPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAVwAAAFcAQAAAACsbTuBAAABA0lEQVR42u3ayw3DIBBF0VEacP9d0oHzwZgBb5JIWUQc5AXBdxVdwZvBsb8/SoDBYDAYDAZ/Dcc5tv3xPNafk7T4gm/xwQD/O9zc2PY+KUmS/JYbq7qRfajr0yI3uDHMucGN0Y0jb3CDG2PeyI+8wY2pTumbhjpleTfyyDvGWPD669Y9U+YU2gD7xtL7Rm2CleiRo74t3JA3mgYRQyKVN7ixNT1St7z/5MbibuRu+RAzQhZVp8wtjjhOGfuG/sZZpNTT5Mwh3FDD5nNknnBj7Sx67YjKG9yYr9Uu1/Tc4MYZNnLZYt+QN4bm+Xa5XuGGOiV/L+pM0d/wZTUYDAaDweBfwXdTX4EBUKuQ3wAAAABJRU5ErkJggg=="

func testQRBytes(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(testQRPNGBase64)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestQRCodeContent(t *testing.T) {
	text, ok := QRCodeContent(testQRBytes(t))
	if !ok || text != "QR-RELAY-TEST-123" {
		t.Fatalf("QRCodeContent = %q, %v", text, ok)
	}
}

func TestReplaceQRCodeImages(t *testing.T) {
	qr := testQRBytes(t)
	logo, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9ZQmcAAAAASUVORK5CYII=")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if r.URL.Path == "/logo.png" {
			_, _ = w.Write(logo)
			return
		}
		_, _ = w.Write(qr)
	}))
	defer srv.Close()

	input := `<div><img src="` + srv.URL + `/qr.png" alt="QR code" width="240" height="240"></div>` +
		`<img src="` + srv.URL + `/logo.png" alt="logo" width="240" height="240">`
	out := ReplaceQRCodeImages(context.Background(), input, srv.URL+"/login")
	if !strings.Contains(out, PlaceholderQRRelayImg) {
		t.Fatalf("QR placeholder missing: %s", out)
	}
	if strings.Contains(out, "/qr.png") {
		t.Fatalf("QR image was not replaced: %s", out)
	}
	if !strings.Contains(out, "/logo.png") {
		t.Fatalf("non-QR image must stay: %s", out)
	}
}

func TestDirectQRImageURL(t *testing.T) {
	qr := testQRBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(qr)
	}))
	defer srv.Close()

	htmlText, finalURL, err := FetchURLHTML(context.Background(), srv.URL, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if finalURL == nil || !IsDirectQRPage(htmlText) || !strings.Contains(htmlText, PlaceholderQRRelayImg) {
		t.Fatalf("direct QR fetch = %q, %v", htmlText, finalURL)
	}
	rewritten, err := RewriteHTMLWithAI(context.Background(), "https://example.com/v1", "key", "model", time.Second, MirrorRewriteInput{
		RawHTML: htmlText,
	})
	if err != nil {
		t.Fatal(err)
	}
	if IsDirectQRPage(rewritten) || !strings.Contains(rewritten, PlaceholderQRRelayImg) {
		t.Fatalf("direct QR rewrite = %q", rewritten)
	}
	if err := ValidateRewrittenHTML(rewritten, ""); err != nil {
		t.Fatalf("QR-only page should validate: %v", err)
	}
}

func TestNormalizeQRPlaceholderUsage(t *testing.T) {
	in := `<img src="{{QR_RELAY_IMG}}" width="180"><img src='{{QR_RELAY_IMG}}'>`
	out := NormalizeQRPlaceholderUsage(in)
	if strings.Contains(out, `src="{{QR_RELAY_IMG}}"`) || strings.Contains(out, `src='{{QR_RELAY_IMG}}'`) {
		t.Fatalf("src placeholder not normalized: %s", out)
	}
	if strings.Count(out, `src="{{QR_RELAY_URL}}"`) != 1 || strings.Count(out, `src='{{QR_RELAY_URL}}'`) != 1 {
		t.Fatalf("unexpected normalization: %s", out)
	}
}
