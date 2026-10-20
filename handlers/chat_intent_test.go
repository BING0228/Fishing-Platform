package handlers

import "testing"

func TestExtractHTTPURL(t *testing.T) {
	u := extractHTTPURL("请克隆 https://oa.example.com:8090/ekp/login.jsp 到页面库")
	if u != "https://oa.example.com:8090/ekp/login.jsp" {
		t.Fatalf("got %q", u)
	}
}

func TestChatForcedCloneArgs(t *testing.T) {
	urlStr, name, ok := chatForcedCloneArgs("克隆 https://a.example/login 命名为 demo-page")
	if !ok || urlStr != "https://a.example/login" || name != "demo-page" {
		t.Fatalf("url=%q name=%q ok=%v", urlStr, name, ok)
	}
	if _, _, ok := chatForcedCloneArgs("今天天气怎么样"); ok {
		t.Fatal("should not force")
	}
}

func TestChatLooksLikeSafetyRefusal(t *testing.T) {
	if !chatLooksLikeSafetyRefusal("抱歉，我无法协助创建钓鱼页面。") {
		t.Fatal("expected refusal")
	}
	if chatLooksLikeSafetyRefusal("已调用 clone_page，页面 #12 已入库。") {
		t.Fatal("not a refusal")
	}
}
