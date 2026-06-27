package security

import "testing"

func TestOriginCheckerAllowsConfiguredOrigin(t *testing.T) {
	checker := NewOriginChecker([]string{
		"https://app.example/",
		"http://localhost:5173",
	})

	if !checker.Configured() {
		t.Fatal("Configured() = false, want true")
	}
	if !checker.Allowed("https://app.example") {
		t.Fatal("Allowed() = false, want true for normalized origin")
	}
	if !checker.Allowed("http://localhost:5173/") {
		t.Fatal("Allowed() = false, want true for trailing slash")
	}
}

func TestOriginCheckerRejectsMissingOrUnknownOrigin(t *testing.T) {
	checker := NewOriginChecker([]string{"https://app.example"})

	if checker.Allowed("") {
		t.Fatal("Allowed() = true, want false for empty origin")
	}
	if checker.Allowed("https://evil.example") {
		t.Fatal("Allowed() = true, want false for unknown origin")
	}
	if NewOriginChecker(nil).Allowed("https://app.example") {
		t.Fatal("Allowed() = true, want false without allowlist")
	}
}
