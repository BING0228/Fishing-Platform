package handlers

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	httpURLRe = regexp.MustCompile(`(?i)https?://[^\s<>"')\]]+`)
	// Soft refusal markers from safety-tuned models.
	chatRefusalRe = regexp.MustCompile(`(?is)(无法协助|不能协助|不会协助|不能帮助|无法帮助|不能提供|拒绝|违法|非法|phishing|不能帮你|won't help|cannot help|can't help|i can't assist|i cannot assist|against (my|the) (policy|guidelines))`)
)

func extractHTTPURL(text string) string {
	m := httpURLRe.FindString(text)
	if m == "" {
		return ""
	}
	m = strings.TrimRight(m, ".,;:!?）)」』】\"'")
	u, err := url.Parse(m)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

func chatWantsPageMirror(text string) bool {
	low := strings.ToLower(text)
	keys := []string{
		"克隆", "镜像", "mirror", "clone", "抓取页面", "复制页面", "同步页面",
		"落地页", "登录页", "登陆页", "page library", "页面库",
	}
	for _, k := range keys {
		if strings.Contains(low, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func chatLooksLikeSafetyRefusal(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	return chatRefusalRe.MatchString(content)
}

// chatForcedCloneArgs returns clone_page arguments when the user clearly asked
// to mirror a URL but the model refused or skipped tools.
func chatForcedCloneArgs(userText string) (urlStr string, name string, ok bool) {
	urlStr = extractHTTPURL(userText)
	if urlStr == "" || !chatWantsPageMirror(userText) {
		return "", "", false
	}
	// Optional: 「克隆成 xxx」「命名为 xxx」
	name = ""
	if m := regexp.MustCompile(`(?i)(?:命名为|名叫|叫做|名称为|name\s*[:=]|as)\s*([A-Za-z0-9_-]{1,64})`).FindStringSubmatch(userText); len(m) == 2 {
		name = m[1]
	}
	return urlStr, name, true
}
