package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestFrontendEmbedIncludesNextAssets(t *testing.T) {
	var chunkJS, chunkCSS, loginCover bool
	err := fs.WalkDir(frontendFS, "frontend/dist", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.Contains(path, "/_next/static/chunks/") && strings.HasSuffix(path, ".js") {
			chunkJS = true
		}
		if strings.Contains(path, "/_next/static/chunks/") && strings.HasSuffix(path, ".css") {
			chunkCSS = true
		}
		if strings.HasSuffix(path, "/login-cover.jpg") {
			loginCover = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded frontend: %v", err)
	}
	if !chunkJS {
		t.Fatal("embedded frontendFS missing _next/static/chunks/*.js; use //go:embed all:frontend/dist")
	}
	if !chunkCSS {
		t.Fatal("embedded frontendFS missing _next/static/chunks/*.css; use //go:embed all:frontend/dist")
	}
	if !loginCover {
		t.Fatal("embedded frontendFS missing login-cover.jpg (LoginPage background)")
	}
}

func TestStaticPathJoinTrimsWildcardSlash(t *testing.T) {
	// Mirrors the Gin wildcard join used by static handlers in main.go.
	staticPath := func(prefix, filepath string) string {
		return prefix + strings.TrimPrefix(filepath, "/")
	}
	got := staticPath("/_next/", "/static/chunks/app.js")
	want := "/_next/static/chunks/app.js"
	if got != want {
		t.Fatalf("staticPath=%q want %q", got, want)
	}
}
