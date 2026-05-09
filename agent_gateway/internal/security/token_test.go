package security

import "testing"

func TestGenerateTokenReturnsStableLengthRandomToken(t *testing.T) {
	first, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	second, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() second error = %v", err)
	}
	if len(first) < 40 {
		t.Fatalf("token length = %d, want at least 40", len(first))
	}
	if first == second {
		t.Fatal("GenerateToken() returned duplicate tokens")
	}
}

func TestBearerToken(t *testing.T) {
	token, err := BearerToken("Bearer abc123")
	if err != nil {
		t.Fatalf("BearerToken() error = %v", err)
	}
	if token != "abc123" {
		t.Fatalf("token = %q, want abc123", token)
	}

	if _, err := BearerToken("abc123"); err == nil {
		t.Fatal("BearerToken() error = nil, want missing bearer error")
	}
}
