package utils

import (
	"regexp"
	"strings"
)

var (
	scriptStyleBlockRe = regexp.MustCompile(`(?is)<!--.*?-->|<script\b[^>]*>.*?</script>|<style\b[^>]*>.*?</style>|<noscript\b[^>]*>.*?</noscript>`)
	inputTagRe         = regexp.MustCompile(`(?is)<(input|textarea)\b[^>]*>`)
	formOpenRe         = regexp.MustCompile(`(?is)<form\b[^>]*>`)
	formCloseRe        = regexp.MustCompile(`(?is)</form\s*>`)
	attrTokenRe        = regexp.MustCompile(`(?is)^\s*([^\s"'=<>/]+)(?:\s*=\s*("[^"]*"|'[^']*'|[^\s"'=<>]+))?`)
	documentBaseTagRe  = regexp.MustCompile(`(?is)<base\b[^>]*>`)
)

var usernameCanon = map[string]struct{}{
	"user": {}, "username": {}, "userid": {}, "useremail": {}, "userphone": {},
	"uname": {}, "uid": {}, "login": {}, "loginname": {}, "loginid": {},
	"loginemail": {}, "logid": {}, "account": {}, "accountname": {}, "acct": {},
	"email": {}, "mail": {}, "e-mail": {}, "phone": {}, "mobile": {}, "tel": {},
	"telephone": {}, "jusername": {}, "txtuser": {}, "txtusername": {},
	"txtaccount": {}, "txtemail": {}, "txtmobile": {},
}

var passwordCanon = map[string]struct{}{
	"password": {}, "passwd": {}, "pass": {}, "pwd": {}, "jpassword": {},
	"txtpassword": {}, "txtpwd": {}, "userpass": {}, "userpassword": {},
	"loginpassword": {}, "loginpwd": {},
}

var captchaCanon = map[string]struct{}{
	"captcha": {}, "captchacode": {}, "captchavalue": {}, "vcode": {},
	"verifycode": {}, "verify": {}, "validatecode": {}, "checkcode": {},
	"authcode": {}, "sms": {}, "smscode": {}, "imgcode": {}, "validcode": {},
}

var skipInputTypes = map[string]struct{}{
	"hidden": {}, "submit": {}, "button": {}, "reset": {}, "checkbox": {},
	"radio": {}, "file": {}, "image": {}, "color": {}, "range": {}, "date": {},
	"datetime-local": {}, "month": {}, "week": {}, "time": {},
}

// NormalizeRewrittenLoginHTML forces login inputs onto platform field names
// (username / password / captchavalue) and points login forms at submitURL.
// AI rewrite often keeps email/account/id-only fields; validation requires name=.
func NormalizeRewrittenLoginHTML(html, submitURL string) string {
	submitURL = strings.TrimSpace(submitURL)
	if submitURL == "" {
		submitURL = PlaceholderSubmitURL
	}
	if strings.TrimSpace(html) == "" {
		return html
	}
	html = StripUnsafeDocumentBase(html)

	type block struct {
		body string
		keep bool
	}
	var blocks []block
	last := 0
	for _, loc := range scriptStyleBlockRe.FindAllStringIndex(html, -1) {
		if loc[0] > last {
			blocks = append(blocks, block{body: html[last:loc[0]]})
		}
		blocks = append(blocks, block{body: html[loc[0]:loc[1]], keep: true})
		last = loc[1]
	}
	if last < len(html) {
		blocks = append(blocks, block{body: html[last:]})
	}

	var b strings.Builder
	b.Grow(len(html) + 64)
	for _, blk := range blocks {
		if blk.keep {
			b.WriteString(blk.body)
			continue
		}
		chunk := rewriteLoginInputs(blk.body)
		chunk = rewriteLoginForms(chunk, submitURL)
		b.WriteString(chunk)
	}
	out := b.String()
	return RewriteQuotedSubmitPaths(out, submitURL)
}

// StripUnsafeDocumentBase removes <base href> values that resolve relative
// URLs against file:// or other non-http origins (e.g. saved-as-complete pages).
func StripUnsafeDocumentBase(html string) string {
	return documentBaseTagRe.ReplaceAllStringFunc(html, func(tag string) string {
		href := strings.TrimSpace(parseTagAttrs(tag)["href"])
		if href == "" {
			return ""
		}
		low := strings.ToLower(href)
		if strings.HasPrefix(low, "file:") || strings.HasPrefix(low, "blob:") || strings.HasPrefix(low, "about:") {
			return ""
		}
		return tag
	})
}

// RewriteQuotedSubmitPaths replaces quoted "/api/submit" with submitURL
// (usually {{SUBMIT_URL}} so project build can substitute the running route).
func RewriteQuotedSubmitPaths(html, submitURL string) string {
	submitURL = strings.TrimSpace(submitURL)
	if submitURL == "" || submitURL == DefaultMirrorSubmitURL {
		return html
	}
	html = strings.ReplaceAll(html, `"`+DefaultMirrorSubmitURL+`"`, `"`+submitURL+`"`)
	html = strings.ReplaceAll(html, `'`+DefaultMirrorSubmitURL+`'`, `'`+submitURL+`'`)
	return html
}

func rewriteLoginInputs(html string) string {
	locs := inputTagRe.FindAllStringIndex(html, -1)
	if len(locs) == 0 {
		return html
	}

	type hit struct {
		start, end int
		tag        string
		role       string
		eligible   bool
	}
	hits := make([]hit, 0, len(locs))
	hasUsername := false
	passwordCount := 0
	for _, loc := range locs {
		tag := html[loc[0]:loc[1]]
		role, eligible := classifyLoginInput(tag)
		if role == "username" {
			hasUsername = true
		}
		if role == "password" {
			passwordCount++
			if passwordCount > 1 {
				role = ""
			}
		}
		hits = append(hits, hit{start: loc[0], end: loc[1], tag: tag, role: role, eligible: eligible})
	}

	if !hasUsername && passwordCount > 0 {
		for i := range hits {
			if hits[i].role == "" && hits[i].eligible {
				hits[i].role = "username"
				break
			}
		}
	}

	var b strings.Builder
	b.Grow(len(html) + 32)
	last := 0
	for _, h := range hits {
		b.WriteString(html[last:h.start])
		if h.role != "" {
			b.WriteString(setTagAttr(h.tag, "name", h.role))
		} else {
			b.WriteString(h.tag)
		}
		last = h.end
	}
	b.WriteString(html[last:])
	return b.String()
}

func rewriteLoginForms(html, submitURL string) string {
	opens := formOpenRe.FindAllStringIndex(html, -1)
	if len(opens) == 0 {
		return html
	}
	closes := formCloseRe.FindAllStringIndex(html, -1)

	nextClose := func(after int) int {
		for _, c := range closes {
			if c[0] >= after {
				return c[0]
			}
		}
		return len(html)
	}

	var b strings.Builder
	b.Grow(len(html) + 32)
	last := 0
	for i, open := range opens {
		endInner := len(html)
		if i+1 < len(opens) && opens[i+1][0] < endInner {
			endInner = opens[i+1][0]
		}
		closeAt := nextClose(open[1])
		if closeAt < endInner {
			endInner = closeAt
		}
		inner := html[open[1]:endInner]
		b.WriteString(html[last:open[0]])
		openTag := html[open[0]:open[1]]
		if formLooksLikeLogin(inner) {
			openTag = setTagAttr(openTag, "action", submitURL)
			openTag = setTagAttr(openTag, "method", "post")
		}
		b.WriteString(openTag)
		last = open[1]
	}
	b.WriteString(html[last:])
	return b.String()
}

func formLooksLikeLogin(inner string) bool {
	low := strings.ToLower(inner)
	if strings.Contains(low, `name="username"`) || strings.Contains(low, `name='username'`) {
		return true
	}
	if strings.Contains(low, `name="password"`) || strings.Contains(low, `name='password'`) {
		return true
	}
	return strings.Contains(low, `type="password"`) || strings.Contains(low, `type='password'`) ||
		regexp.MustCompile(`(?i)type\s*=\s*password\b`).MatchString(inner)
}

func classifyLoginInput(tag string) (role string, eligible bool) {
	attrs := parseTagAttrs(tag)
	typ := strings.ToLower(strings.TrimSpace(attrs["type"]))
	if _, skip := skipInputTypes[typ]; skip && !looksLikeUsername(attrs) && !looksLikePassword(attrs) && !looksLikeCaptcha(attrs) {
		return "", false
	}
	if looksLikePassword(attrs) || typ == "password" {
		return "password", false
	}
	if looksLikeCaptcha(attrs) {
		return "captchavalue", false
	}
	if looksLikeSearch(attrs) {
		return "", false
	}
	if looksLikeUsername(attrs) || typ == "email" || typ == "tel" {
		return "username", true
	}
	if typ == "" || typ == "text" || typ == "number" || typ == "url" || typ == "tel" || typ == "email" {
		return "", true
	}
	return "", false
}

func looksLikeUsername(attrs map[string]string) bool {
	if inCanon(attrs["name"], usernameCanon) || inCanon(attrs["id"], usernameCanon) {
		return true
	}
	ac := canonIdent(attrs["autocomplete"])
	if ac == "username" || ac == "email" || ac == "tel" || ac == "phone" {
		return true
	}
	blob := attrs["placeholder"] + " " + attrs["aria-label"] + " " + attrs["title"] + " " + attrs["alt"]
	low := strings.ToLower(blob)
	for _, key := range []string{
		"username", "user name", "email", "e-mail", "mobile", "phone",
		"用户名", "账号", "帐号", "账户", "邮箱", "手机号", "手机号码", "电话",
	} {
		if strings.Contains(low, key) || strings.Contains(blob, key) {
			return true
		}
	}
	return false
}

func looksLikeSearch(attrs map[string]string) bool {
	c := canonIdent(attrs["name"])
	switch c {
	case "q", "query", "search", "keyword", "keywords", "kw":
		return true
	}
	blob := strings.ToLower(attrs["placeholder"] + " " + attrs["aria-label"])
	return strings.Contains(blob, "search") || strings.Contains(blob, "搜索")
}

func looksLikePassword(attrs map[string]string) bool {
	if strings.EqualFold(strings.TrimSpace(attrs["type"]), "password") {
		return true
	}
	if inCanon(attrs["name"], passwordCanon) || inCanon(attrs["id"], passwordCanon) {
		return true
	}
	ac := canonIdent(attrs["autocomplete"])
	return ac == "currentpassword" || ac == "newpassword" || ac == "password"
}

func looksLikeCaptcha(attrs map[string]string) bool {
	if inCanon(attrs["name"], captchaCanon) || inCanon(attrs["id"], captchaCanon) {
		return true
	}
	blob := attrs["placeholder"] + " " + attrs["aria-label"] + " " + attrs["title"]
	return strings.Contains(blob, "验证码") || strings.Contains(strings.ToLower(blob), "captcha") ||
		strings.Contains(strings.ToLower(blob), "verify code")
}

func inCanon(value string, set map[string]struct{}) bool {
	c := canonIdent(value)
	if c == "" {
		return false
	}
	_, ok := set[c]
	return ok
}

func canonIdent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type htmlAttr struct {
	key    string
	value  string
	hasVal bool
}

func parseTagAttrs(tag string) map[string]string {
	out := map[string]string{}
	for _, a := range parseTagAttrList(tag) {
		out[strings.ToLower(a.key)] = a.value
	}
	return out
}

func parseTagAttrList(tag string) []htmlAttr {
	tag = strings.TrimSpace(tag)
	if len(tag) < 3 || tag[0] != '<' {
		return nil
	}
	inner := strings.TrimSuffix(strings.TrimSpace(tag[1:len(tag)-1]), "/")
	inner = strings.TrimSpace(inner)
	// skip tag name
	i := 0
	for i < len(inner) && !isHTMLSpace(inner[i]) {
		i++
	}
	rest := strings.TrimSpace(inner[i:])
	var attrs []htmlAttr
	for rest != "" {
		m := attrTokenRe.FindStringSubmatch(rest)
		if m == nil {
			break
		}
		attr := htmlAttr{key: m[1]}
		if m[2] != "" {
			attr.hasVal = true
			attr.value = unquoteAttr(m[2])
		}
		attrs = append(attrs, attr)
		rest = strings.TrimSpace(rest[len(m[0]):])
	}
	return attrs
}

func unquoteAttr(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

func isHTMLSpace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t' || b == '\f'
}

func setTagAttr(tag, key, value string) string {
	tag = strings.TrimSpace(tag)
	if len(tag) < 3 || tag[0] != '<' || tag[len(tag)-1] != '>' {
		return tag
	}
	selfClose := strings.HasSuffix(strings.TrimSpace(tag[:len(tag)-1]), "/")
	inner := strings.TrimSuffix(strings.TrimSpace(tag[1:len(tag)-1]), "/")
	inner = strings.TrimSpace(inner)

	i := 0
	for i < len(inner) && !isHTMLSpace(inner[i]) {
		i++
	}
	name := inner[:i]
	attrs := parseTagAttrList(tag)
	found := false
	want := strings.ToLower(key)
	for idx := range attrs {
		if strings.EqualFold(attrs[idx].key, want) {
			attrs[idx].value = value
			attrs[idx].hasVal = true
			found = true
			break
		}
	}
	if !found {
		attrs = append(attrs, htmlAttr{key: key, value: value, hasVal: true})
	}

	var b strings.Builder
	b.WriteByte('<')
	b.WriteString(name)
	for _, a := range attrs {
		b.WriteByte(' ')
		b.WriteString(a.key)
		if a.hasVal {
			b.WriteString(`="`)
			b.WriteString(escapeAttr(a.value))
			b.WriteByte('"')
		}
	}
	if selfClose {
		b.WriteString(" /")
	}
	b.WriteByte('>')
	return b.String()
}

func escapeAttr(v string) string {
	v = strings.ReplaceAll(v, `&`, "&amp;")
	v = strings.ReplaceAll(v, `"`, "&quot;")
	return v
}
