package utils

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestLoginAuthSequence(t *testing.T) {
	auth := smtpLoginAuth("user@163.com", "auth-code")
	mech, initial, err := auth.Start(&smtp.ServerInfo{Name: "smtp.163.com", TLS: true})
	if err != nil {
		t.Fatal(err)
	}
	if mech != "LOGIN" {
		t.Fatalf("mech = %q, want LOGIN", mech)
	}
	if len(initial) != 0 {
		t.Fatalf("LOGIN should not send an initial response, got %q", initial)
	}

	user, err := auth.Next([]byte("Username:"), true)
	if err != nil {
		t.Fatal(err)
	}
	if string(user) != "user@163.com" {
		t.Fatalf("username = %q", user)
	}

	pass, err := auth.Next([]byte("Password:"), true)
	if err != nil {
		t.Fatal(err)
	}
	if string(pass) != "auth-code" {
		t.Fatalf("password = %q", pass)
	}

	done, err := auth.Next(nil, false)
	if err != nil || done != nil {
		t.Fatalf("expected LOGIN to finish, got %q / %v", done, err)
	}
}

func TestPrefersSMTPLogin(t *testing.T) {
	for _, host := range []string{"smtp.163.com", "smtp.126.com", "smtp.yeah.net", "smtp.qq.com"} {
		if !prefersSMTPLogin(host) {
			t.Fatalf("expected LOGIN preference for %s", host)
		}
	}
	if prefersSMTPLogin("smtp.example.com") {
		t.Fatal("did not expect LOGIN preference for smtp.example.com")
	}
}

func TestChooseSmtpAuth(t *testing.T) {
	loginCfg := SmtpSendConfig{
		Host:      "smtp.163.com",
		Username:  "user@163.com",
		Password:  "code",
		FromEmail: "user@163.com",
	}
	auth := chooseSmtpAuth(loginCfg, "LOGIN PLAIN")
	mech, _, err := auth.Start(&smtp.ServerInfo{Name: "smtp.163.com", TLS: true})
	if err != nil {
		t.Fatal(err)
	}
	if mech != "LOGIN" {
		t.Fatalf("163 should use LOGIN, got %s", mech)
	}

	fallback := chooseSmtpAuth(SmtpSendConfig{
		Host:      "smtp.163.com",
		FromEmail: "user@163.com",
		Password:  "code",
	}, "")
	mech, _, err = fallback.Start(&smtp.ServerInfo{Name: "smtp.163.com", TLS: true})
	if err != nil {
		t.Fatal(err)
	}
	if mech != "LOGIN" {
		t.Fatalf("163 with empty AUTH ads should still LOGIN, got %s", mech)
	}
	if smtpAuthUsername(SmtpSendConfig{FromEmail: "user@163.com"}) != "user@163.com" {
		t.Fatal("username should fall back to from_email")
	}

	plain := chooseSmtpAuth(SmtpSendConfig{
		Host:      "smtp.example.com",
		Username:  "user@example.com",
		Password:  "secret",
		FromEmail: "user@example.com",
	}, "PLAIN")
	mech, _, err = plain.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: true})
	if err != nil {
		t.Fatal(err)
	}
	if mech != "PLAIN" {
		t.Fatalf("generic PLAIN-only server should use PLAIN, got %s", mech)
	}
}

func TestForceTLSAuthLetsPlainAuthRun(t *testing.T) {
	inner := smtp.PlainAuth("", "user@example.com", "secret", "smtp.example.com")
	wrapped := wrapSmtpAuth(SmtpSendConfig{Port: 465, UseSSL: true}, inner)
	_, _, err := wrapped.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: false})
	if err != nil {
		t.Fatalf("implicit TLS should not fail PlainAuth: %v", err)
	}
}

func TestSendSMTPMailAuthLogin(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	type captured struct {
		user string
		pass string
	}
	gotCh := make(chan captured, 1)
	errCh := make(chan error, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

		br := bufio.NewReader(conn)
		write := func(line string) error {
			_, err := conn.Write([]byte(line + "\r\n"))
			return err
		}
		if err := write("220 mock ESMTP"); err != nil {
			errCh <- err
			return
		}

		readLine := func() (string, error) {
			line, err := br.ReadString('\n')
			return strings.TrimSpace(line), err
		}

		ehlo, err := readLine()
		if err != nil {
			errCh <- err
			return
		}
		if !strings.HasPrefix(strings.ToUpper(ehlo), "EHLO") && !strings.HasPrefix(strings.ToUpper(ehlo), "HELO") {
			errCh <- fmt.Errorf("expected EHLO, got %q", ehlo)
			return
		}
		if err := write("250-mock"); err != nil {
			errCh <- err
			return
		}
		if err := write("250-AUTH LOGIN"); err != nil {
			errCh <- err
			return
		}
		if err := write("250 OK"); err != nil {
			errCh <- err
			return
		}

		authLine, err := readLine()
		if err != nil {
			errCh <- err
			return
		}
		if !strings.HasPrefix(strings.ToUpper(authLine), "AUTH LOGIN") {
			errCh <- fmt.Errorf("expected AUTH LOGIN, got %q", authLine)
			return
		}
		if err := write("334 " + base64.StdEncoding.EncodeToString([]byte("Username:"))); err != nil {
			errCh <- err
			return
		}
		userLine, err := readLine()
		if err != nil {
			errCh <- err
			return
		}
		userBytes, err := base64.StdEncoding.DecodeString(userLine)
		if err != nil {
			errCh <- err
			return
		}
		if err := write("334 " + base64.StdEncoding.EncodeToString([]byte("Password:"))); err != nil {
			errCh <- err
			return
		}
		passLine, err := readLine()
		if err != nil {
			errCh <- err
			return
		}
		passBytes, err := base64.StdEncoding.DecodeString(passLine)
		if err != nil {
			errCh <- err
			return
		}
		gotCh <- captured{user: string(userBytes), pass: string(passBytes)}
		if err := write("235 2.7.0 Authentication successful"); err != nil {
			errCh <- err
			return
		}

		for {
			line, err := readLine()
			if err != nil {
				errCh <- err
				return
			}
			upper := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(upper, "MAIL FROM:"):
				if err := write("250 OK"); err != nil {
					errCh <- err
					return
				}
			case strings.HasPrefix(upper, "RCPT TO:"):
				if err := write("250 OK"); err != nil {
					errCh <- err
					return
				}
			case upper == "DATA":
				if err := write("354 End data with <CR><LF>.<CR><LF>"); err != nil {
					errCh <- err
					return
				}
				for {
					bodyLine, err := readLine()
					if err != nil {
						errCh <- err
						return
					}
					if bodyLine == "." {
						break
					}
				}
				if err := write("250 OK"); err != nil {
					errCh <- err
					return
				}
			case upper == "QUIT":
				_ = write("221 Bye")
				errCh <- nil
				return
			default:
				if err := write("250 OK"); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	err = SendSMTPMail(SmtpSendConfig{
		Host:      "127.0.0.1",
		Port:      port,
		Username:  "user@163.com",
		Password:  "auth-code-xyz",
		FromEmail: "user@163.com",
	}, SmtpMessage{
		To:      []string{"to@example.com"},
		Subject: "hi",
		Body:    "body",
	})
	if err != nil {
		select {
		case srvErr := <-errCh:
			if srvErr != nil {
				t.Fatalf("SendSMTPMail: %v (server: %v)", err, srvErr)
			}
		default:
		}
		t.Fatalf("SendSMTPMail: %v", err)
	}

	select {
	case got := <-gotCh:
		if got.user != "user@163.com" || got.pass != "auth-code-xyz" {
			t.Fatalf("captured auth = %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for LOGIN credentials")
	}
}
