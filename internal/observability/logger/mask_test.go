package logger

import "testing"

func TestMaskAuthorization(t *testing.T) {
	got := MaskAuthorization("Bearer abcdef1234")
	want := "Bearer ****1234"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMaskCookie(t *testing.T) {
	got := MaskCookie("session=abcdef1234; other=xyz")
	want := "session=****1234; other=****xyz"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMaskJSON(t *testing.T) {
	input := map[string]any{
		"password": "hunter2",
		"token":    "abc12345",
		"nested": map[string]any{
			"api_key": "key_12345678",
		},
	}
	masked := MaskJSON(input)
	if masked["password"] != "****ter2" {
		t.Fatalf("expected masked password, got %v", masked["password"])
	}
	if masked["token"] != "****2345" {
		t.Fatalf("expected masked token, got %v", masked["token"])
	}
	nested, ok := masked["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map")
	}
	if nested["api_key"] != "****5678" {
		t.Fatalf("expected masked api_key, got %v", nested["api_key"])
	}
}
