package utils

import (
	"strings"
	"testing"
)

func TestApplyProjectHTMLPlaceholders(t *testing.T) {
	in := `<form action="{{SUBMIT_URL}}"><a href="{{REDIRECT_URL}}">x</a>{{QR_RELAY_IMG}}</form>`
	out := ApplyProjectHTMLPlaceholders(in, ProjectHTMLVars{
		SubmitURL:   "/api/submit",
		RedirectURL: "https://origin.example/login",
		QRRelayURL:  "/q/abc.png",
	})
	for _, want := range []string{
		`action="/api/submit"`,
		`href="https://origin.example/login"`,
		`src="/q/abc.png"`,
	} {
		if !contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	if contains(out, "{{") {
		t.Fatalf("placeholders left: %s", out)
	}
}

func TestApplyProjectHTMLPlaceholdersRewritesLoginFormWithoutToken(t *testing.T) {
	in := `<form action="/login" method="get"><input name="email"><input type="password" name="pwd"></form>`
	out := ApplyProjectHTMLPlaceholders(in, ProjectHTMLVars{
		SubmitURL:   "/api/submit",
		RedirectURL: "https://origin.example/login",
	})
	if !contains(out, `action="/api/submit"`) {
		t.Fatalf("login form not pointed at submit: %s", out)
	}
	if contains(out, "{{SUBMIT_URL}}") || contains(out, "file:") {
		t.Fatalf("token or file leftover: %s", out)
	}
}

func TestApplyProjectHTMLPlaceholdersDefaultSubmit(t *testing.T) {
	out := ApplyProjectHTMLPlaceholders(`{{SUBMIT_URL}}`, ProjectHTMLVars{})
	if out != "/api/submit" {
		t.Fatalf("got %q", out)
	}
}

func TestApplyProjectHTMLPlaceholdersStripsFileBase(t *testing.T) {
	out := ApplyProjectHTMLPlaceholders(
		`<base href="file:///Users/me/page/"><form action="{{SUBMIT_URL}}"></form>`,
		ProjectHTMLVars{SubmitURL: "https://host.example/api/submit"},
	)
	if contains(out, "<base") || contains(out, "file:") {
		t.Fatalf("file base not stripped: %s", out)
	}
	if !contains(out, `action="https://host.example/api/submit"`) {
		t.Fatalf("submit not applied: %s", out)
	}
}

func TestAbsoluteHTTPURL(t *testing.T) {
	base := "http://127.0.0.1:8000"
	cases := []struct {
		raw, want string
	}{
		{"", "http://127.0.0.1:8000/api/submit"},
		{"/api/submit", "http://127.0.0.1:8000/api/submit"},
		{"https://other.example/recv", "https://other.example/recv"},
		{"file:///api/submit", "http://127.0.0.1:8000/api/submit"},
	}
	for _, tc := range cases {
		got := AbsoluteHTTPURL(tc.raw, base)
		if got != tc.want {
			t.Fatalf("AbsoluteHTTPURL(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
	if got := AbsoluteHTTPURL("/api/submit", ""); got != "/api/submit" {
		t.Fatalf("empty base should keep path, got %q", got)
	}
}

func TestLandingSubmitToken(t *testing.T) {
	if got := LandingSubmitToken(""); got != PlaceholderSubmitURL {
		t.Fatalf("empty: %q", got)
	}
	if got := LandingSubmitToken("/api/submit"); got != PlaceholderSubmitURL {
		t.Fatalf("default path: %q", got)
	}
	if got := LandingSubmitToken("https://custom.example/recv"); got != "https://custom.example/recv" {
		t.Fatalf("custom: %q", got)
	}
}

func TestApplyProjectHTMLPlaceholdersNormalizesQRSrc(t *testing.T) {
	out := ApplyProjectHTMLPlaceholders(
		`<div><img src="{{QR_RELAY_IMG}}" width="180"></div>`,
		ProjectHTMLVars{QRRelayURL: "http://example.com/q/x.png"},
	)
	if !strings.Contains(out, `src="http://example.com/q/x.png"`) {
		t.Fatalf("QR src not replaced: %s", out)
	}
	if strings.Contains(out, "<img src=\"<img") {
		t.Fatalf("nested img from placeholder: %s", out)
	}
}
