package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Required English section headings / TOC anchors (GitHub slug style).
var readmeENRequired = []string{
	"## Legal notice",
	"## Overview",
	"## Languages",
	"## Features",
	"## Architecture",
	"## Quick start",
	"## Agents",
	"## Console",
	"## Disclaimer",
}

// Required Chinese section headings (parity with EN section set).
var readmeZHRequired = []string{
	"## 法律声明",
	"## 项目简介",
	"## 语言",
	"## 核心能力",
	"## 架构",
	"## 快速开始",
	"## 远程 Agent",
	"## 控制台",
	"## 免责声明",
}

// Feature surfaces that must appear in the Features section body.
var readmeFeatureNeedlesEN = []string{
	"AI Assistant",
	"Page library",
	"Projects",
	"Mail",
	"Push channels",
	"QR",
	"Information gathering",
	"Agents",
	"Auth & roles",
	"Security and operations",
	"IP blacklist",
}

var readmeFeatureNeedlesZH = []string{
	"AI 助手",
	"页面库",
	"项目",
	"邮件",
	"推送通道",
	"QR 钓鱼",
	"信息搜集",
	"Agent",
	"认证与角色",
	"安全加固与运维",
	"IP 黑名单",
}

var mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

func TestREADMERequiredSectionsAndLinks(t *testing.T) {
	root := repoRoot(t)

	enPath := filepath.Join(root, "README.md")
	zhPath := filepath.Join(root, "README.zh.md")
	docsIndex := filepath.Join(root, "docs", "README.md")

	en := mustRead(t, enPath)
	zh := mustRead(t, zhPath)
	docs := mustRead(t, docsIndex)

	for _, h := range readmeENRequired {
		if !strings.Contains(en, h) {
			t.Errorf("README.md missing heading %q", h)
		}
	}
	for _, h := range readmeZHRequired {
		if !strings.Contains(zh, h) {
			t.Errorf("README.zh.md missing heading %q", h)
		}
	}

	enFeatures := sectionBody(en, "## Features", "## ")
	zhFeatures := sectionBody(zh, "## 核心能力", "## ")
	if enFeatures == "" || zhFeatures == "" {
		t.Fatal("could not extract Features sections")
	}
	for _, n := range readmeFeatureNeedlesEN {
		if !strings.Contains(enFeatures, n) {
			t.Errorf("README.md Features missing %q", n)
		}
	}
	for _, n := range readmeFeatureNeedlesZH {
		if !strings.Contains(zhFeatures, n) {
			t.Errorf("README.zh.md Features missing %q", n)
		}
	}

	// TOC anchors referenced from the top nav must exist as headings we already require,
	// plus resolve every relative markdown / asset link.
	for _, item := range []struct {
		label string
		path  string
		body  string
	}{
		{"README.md", enPath, en},
		{"README.zh.md", zhPath, zh},
		{"docs/README.md", docsIndex, docs},
	} {
		baseDir := filepath.Dir(item.path)
		for _, raw := range mdLinkRe.FindAllStringSubmatch(item.body, -1) {
			target := strings.TrimSpace(raw[1])
			if target == "" {
				continue
			}
			// Strip title / angle form: url "title"
			if i := strings.IndexAny(target, " \t"); i >= 0 {
				target = target[:i]
			}
			target = strings.Trim(target, `"'`)
			if skipLinkTarget(target) {
				continue
			}
			// Drop URL fragment for file existence check.
			filePart := target
			if i := strings.IndexByte(filePart, '#'); i >= 0 {
				filePart = filePart[:i]
			}
			if filePart == "" {
				continue // pure in-page anchor
			}
			resolved := filepath.Clean(filepath.Join(baseDir, filePart))
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("%s: broken link %q → %s (%v)", item.label, target, resolved, err)
			}
		}
	}

	assertAgentBufferingDocsMatchCode(t, root, en, zh)

}

// assertAgentBufferingDocsMatchCode ties README claims to shipped agent behavior:
// access logs use pending-logs.json; proxySubmission returns 502 with no submit queue.
func assertAgentBufferingDocsMatchCode(t *testing.T, root, en, zh string) {
	t.Helper()
	agentSrc := mustRead(t, filepath.Join(root, "agent", "main.go"))
	if !strings.Contains(agentSrc, "pending-logs.json") {
		t.Fatal("agent/main.go missing pending-logs.json path — README claim would be wrong")
	}
	if !strings.Contains(agentSrc, "func (a *Agent) proxySubmission") {
		t.Fatal("agent/main.go missing proxySubmission")
	}
	// Live proxy failure path (no local submit buffer).
	if !strings.Contains(agentSrc, `"upstream unavailable"`) || !strings.Contains(agentSrc, "StatusBadGateway") {
		t.Fatal("agent/main.go proxySubmission no longer returns BadGateway on upstream failure")
	}
	// Docs must describe logs-only buffering, not durable submit queues.
	for _, doc := range []struct{ name, body string }{{"README.md", en}, {"README.zh.md", zh}} {
		if !strings.Contains(doc.body, "pending-logs.json") {
			t.Errorf("%s must mention pending-logs.json for access-log buffering", doc.name)
		}
		if !strings.Contains(doc.body, "502") {
			t.Errorf("%s must mention 502 when submit proxy fails (no submit queue)", doc.name)
		}
		forbidden := []string{
			"buffered submits/logs",
			"buffers and retries",
			"提交/日志可缓冲",
			"会先攒着再补传",
		}
		for _, bad := range forbidden {
			if strings.Contains(doc.body, bad) {
				t.Errorf("%s still claims overstated buffering %q", doc.name, bad)
			}
		}
	}
}

func skipLinkTarget(target string) bool {
	lower := strings.ToLower(target)
	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return true
	case strings.HasPrefix(lower, "mailto:"):
		return true
	case strings.HasPrefix(target, "//"):
		return true
	default:
		return false
	}
}

func sectionBody(doc, startHeading, _ string) string {
	i := strings.Index(doc, startHeading)
	if i < 0 {
		return ""
	}
	rest := doc[i+len(startHeading):]
	lines := strings.Split(rest, "\n")
	var b strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Test may run from module root (go test .) — require README.md here.
	if _, err := os.Stat(filepath.Join(wd, "README.md")); err == nil {
		return wd
	}
	t.Fatalf("README.md not found from working directory %s (run go test from module root)", wd)
	return ""
}
