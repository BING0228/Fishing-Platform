package utils

import (
	"strings"
	"testing"
)

func TestAbsolutizeHTMLAssetsRewritesRootRelativeCSS(t *testing.T) {
	in := `<!doctype html><html><head>
<link rel="stylesheet" href="/ekp/resource/style/login.css?s_cache=1"/>
<script src="/ekp/resource/js/common.js"></script>
<img src="/ekp/img/logo.png"/>
<style>.x{background:url(/ekp/a.png)}</style>
<form action="{{SUBMIT_URL}}"><input name="username"></form>
<a href="https://cdn.example/x.css">keep</a>
</head></html>`
	base := "https://oa.langdeng.com.cn:8090/ekp/login.jsp"
	out := AbsolutizeHTMLAssets(in, base)

	want := []string{
		`href="https://oa.langdeng.com.cn:8090/ekp/resource/style/login.css?s_cache=1"`,
		`src="https://oa.langdeng.com.cn:8090/ekp/resource/js/common.js"`,
		`src="https://oa.langdeng.com.cn:8090/ekp/img/logo.png"`,
		`url(https://oa.langdeng.com.cn:8090/ekp/a.png)`,
		`action="{{SUBMIT_URL}}"`,
		`href="https://cdn.example/x.css"`,
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in:\n%s", w, out)
		}
	}
	if strings.Contains(out, `href="/ekp/`) || strings.Contains(out, `src="/ekp/`) {
		t.Fatalf("root-relative asset left behind:\n%s", out)
	}
}

func TestAbsolutizeHTMLAssetsResolvesPathRelative(t *testing.T) {
	in := `<link href="css/app.css" rel="stylesheet"><img src="../img/a.png">`
	out := AbsolutizeHTMLAssets(in, "https://example.com/app/login/")
	if !strings.Contains(out, `href="https://example.com/app/login/css/app.css"`) {
		t.Fatalf("path-relative css: %s", out)
	}
	if !strings.Contains(out, `src="https://example.com/app/img/a.png"`) {
		t.Fatalf("parent-relative img: %s", out)
	}
}

func TestAbsolutizeHTMLAssetsNoopOnBadBase(t *testing.T) {
	in := `<link href="/x.css">`
	if AbsolutizeHTMLAssets(in, "") != in {
		t.Fatal("empty base should noop")
	}
	if AbsolutizeHTMLAssets(in, "not-a-url") != in {
		t.Fatal("bad base should noop")
	}
}
