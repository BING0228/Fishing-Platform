package platform

import (
	"testing"

	"spearhook/models"
)

func TestLocalContainerIDRoundTrip(t *testing.T) {
	pid, ok := ParseLocalContainerPID(MakeLocalContainerID(4321))
	if !ok || pid != 4321 {
		t.Fatalf("pid=%d ok=%v", pid, ok)
	}
	if _, ok := ParseLocalContainerPID("docker:1"); ok {
		t.Fatal("non-local id must not parse")
	}
}

func TestHasProjectSSLCert(t *testing.T) {
	if !HasProjectSSLCert(models.Project{SSLCertPath: "cert.pem", SSLKeyPath: "key.pem"}) {
		t.Fatal("expected cert present")
	}
	if HasProjectSSLCert(models.Project{SSLCertPath: "cert.pem"}) {
		t.Fatal("missing key must be false")
	}
}

func TestProjectServiceURL(t *testing.T) {
	got := ProjectServiceURL("http://example.com:8000", models.Project{Port: 8080, FrontendRoute: "/login"})
	if got != "http://example.com:8080/login" {
		t.Fatalf("url = %q", got)
	}
	got = ProjectServiceURL("", models.Project{Port: 8080, FrontendRoute: "/"})
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("fallback url = %q", got)
	}
}
