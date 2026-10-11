package utils

import (
	"strings"
	"testing"
)

func TestNormalizeRewrittenLoginHTMLRenamesCommonFields(t *testing.T) {
	in := `<form action="/login" method="get">
<input type="text" name="email" placeholder="邮箱">
<input type="password" name="pwd">
<button type="submit">登录</button>
</form>`
	out := NormalizeRewrittenLoginHTML(in, "/api/submit")
	if err := ValidateRewrittenHTML(out, "/api/submit"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `action="/api/submit"`) {
		t.Fatalf("form action not rewritten: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), `method="post"`) {
		t.Fatalf("form method not POST: %s", out)
	}
}

func TestNormalizeRewrittenLoginHTMLUsesIDWhenNameMissing(t *testing.T) {
	in := `<form>
<input id="username" type="text">
<input id="password" type="password">
</form>`
	out := NormalizeRewrittenLoginHTML(in, "/api/submit")
	if err := ValidateRewrittenHTML(out, "/api/submit"); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeRewrittenLoginHTMLLoginNameAndScriptUntouched(t *testing.T) {
	in := `<form action="/x">
<input name="loginName" type="text">
<input name="j_password" type="password">
</form>
<script>var name="email"; document.write('<input name="user">')</script>`
	out := NormalizeRewrittenLoginHTML(in, "/api/submit")
	if err := ValidateRewrittenHTML(out, "/api/submit"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `document.write('<input name="user">')`) {
		t.Fatalf("script rewritten: %s", out)
	}
}

func TestNormalizeRewrittenLoginHTMLPromotesTextBesidePassword(t *testing.T) {
	in := `<form>
<input type="text">
<input type="password">
</form>`
	out := NormalizeRewrittenLoginHTML(in, "/api/submit")
	if err := ValidateRewrittenHTML(out, "/api/submit"); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeRewrittenLoginHTMLSkipsSearchBox(t *testing.T) {
	in := `<form><input name="q" type="text" placeholder="搜索"></form>
<form>
<input name="account" type="text">
<input type="password">
</form>`
	out := NormalizeRewrittenLoginHTML(in, "/api/submit")
	if err := ValidateRewrittenHTML(out, "/api/submit"); err != nil {
		t.Fatal(err)
	}
	if strings.Count(strings.ToLower(out), `name="username"`) != 1 {
		t.Fatalf("search box should not become username: %s", out)
	}
	if !strings.Contains(out, `name="q"`) {
		t.Fatalf("search name lost: %s", out)
	}
}

func TestNormalizeRewrittenLoginHTMLStripsFileBaseAndUsesPlaceholder(t *testing.T) {
	in := `<base href="file:///Users/me/saved/">
<form action="/login">
<input type="text" name="email">
<input type="password" name="pwd">
</form>
<script>fetch("/api/submit")</script>`
	out := NormalizeRewrittenLoginHTML(in, "")
	if strings.Contains(out, "<base") || strings.Contains(out, "file:") {
		t.Fatalf("file base not stripped: %s", out)
	}
	if !strings.Contains(out, `action="`+PlaceholderSubmitURL+`"`) {
		t.Fatalf("form action not placeholder: %s", out)
	}
	if !strings.Contains(out, `fetch("`+PlaceholderSubmitURL+`")`) {
		t.Fatalf("script submit path not rewritten: %s", out)
	}
	if strings.Contains(out, "file:///api/submit") {
		t.Fatalf("file submit leaked: %s", out)
	}
}