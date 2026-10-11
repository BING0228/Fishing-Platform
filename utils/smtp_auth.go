package utils

import (
	"fmt"
	"net/smtp"
	"strings"
)

// loginAuth implements SMTP AUTH LOGIN: username first, then password/auth-code.
// NetEase 163/126, QQ and similar providers require this two-step exchange
// instead of AUTH PLAIN.
type loginAuth struct {
	username, password string
	step               int
}

func smtpLoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username: username, password: password}
}

func (a *loginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	a.step = 0
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		a.step = 0
		return nil, nil
	}
	switch a.step {
	case 0:
		a.step++
		return []byte(a.username), nil
	case 1:
		a.step++
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected SMTP LOGIN challenge: %q", fromServer)
	}
}

// forceTLSAuth wraps an Auth so PlainAuth will accept implicit TLS (port 465).
// net/smtp only sets ServerInfo.TLS after STARTTLS, not after tls.Dial.
type forceTLSAuth struct {
	inner smtp.Auth
}

func (a forceTLSAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if server != nil && !server.TLS {
		clone := *server
		clone.TLS = true
		return a.inner.Start(&clone)
	}
	return a.inner.Start(server)
}

func (a forceTLSAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	return a.inner.Next(fromServer, more)
}

func smtpAuthUsername(cfg SmtpSendConfig) string {
	username := strings.TrimSpace(cfg.Username)
	if username != "" {
		return username
	}
	return strings.TrimSpace(cfg.FromEmail)
}

func containsAuthMech(advertised, mech string) bool {
	for _, part := range strings.Fields(advertised) {
		part = strings.TrimPrefix(part, "=")
		if strings.EqualFold(part, mech) {
			return true
		}
	}
	return false
}

func prefersSMTPLogin(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return false
	}
	for _, suffix := range []string{
		"163.com",
		"126.com",
		"yeah.net",
		"188.com",
		"qq.com",
		"foxmail.com",
		"sina.com",
		"sina.cn",
		"aliyun.com",
		"139.com",
		"189.cn",
	} {
		if h == suffix || strings.HasSuffix(h, "."+suffix) {
			return true
		}
	}
	return false
}

func chooseSmtpAuth(cfg SmtpSendConfig, advertised string) smtp.Auth {
	username := smtpAuthUsername(cfg)
	if username == "" {
		return nil
	}
	hasLogin := containsAuthMech(advertised, "LOGIN")
	if prefersSMTPLogin(cfg.Host) || hasLogin {
		return smtpLoginAuth(username, cfg.Password)
	}
	return smtp.PlainAuth("", username, cfg.Password, cfg.Host)
}

func wrapSmtpAuth(cfg SmtpSendConfig, auth smtp.Auth) smtp.Auth {
	if auth == nil {
		return nil
	}
	if cfg.UseSSL || cfg.Port == 465 {
		return forceTLSAuth{inner: auth}
	}
	return auth
}
