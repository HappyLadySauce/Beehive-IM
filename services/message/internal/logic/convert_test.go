package logic

import (
	"strings"
	"testing"
)

func TestNormalizeContentAcceptsTextJSON(t *testing.T) {
	contentType, content, err := normalizeContent("text", `{"text":"hello"}`)
	if err != nil {
		t.Fatalf("normalizeContent() error = %v", err)
	}
	if contentType != "text" {
		t.Fatalf("contentType = %q, want text", contentType)
	}
	if string(content) != `{"text":"hello"}` {
		t.Fatalf("content = %s", content)
	}
}

func TestNormalizeContentRejectsInvalidJSON(t *testing.T) {
	if _, _, err := normalizeContent("text", `{"text":`); err == nil {
		t.Fatal("normalizeContent() error = nil, want invalid json error")
	}
}

func TestNormalizeContentRejectsUnsupportedType(t *testing.T) {
	if _, _, err := normalizeContent("image", `{"text":"hello"}`); err == nil {
		t.Fatal("normalizeContent() error = nil, want unsupported type error")
	}
}

func TestNormalizeContentRejectsLongText(t *testing.T) {
	body := `{"text":"` + strings.Repeat("a", maxTextBytes+1) + `"}`
	if _, _, err := normalizeContent("text", body); err == nil {
		t.Fatal("normalizeContent() error = nil, want long text error")
	}
}
