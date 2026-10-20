package platform

import "testing"

func TestValidatePublicPort(t *testing.T) {
	if err := ValidatePublicPort(6000); err == nil {
		t.Fatal("6000 is browser-blocked and must be rejected")
	}
	if err := ValidatePublicPort(8080); err != nil {
		t.Fatalf("8080 should be allowed: %v", err)
	}
	if err := ValidatePublicPort(0); err == nil {
		t.Fatal("0 must be rejected")
	}
}
