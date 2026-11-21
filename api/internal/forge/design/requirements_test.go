package design

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/razorpay/slashdocs/api/internal/llm"
)

func TestResolveRequestedDevice(t *testing.T) {
	for _, tc := range []struct{ prompt, old, want string }{
		{"I need desktop screens using Katalyst", "MOBILE", "DESKTOP"},
		{"Build a mobile dashboard", "DESKTOP", "MOBILE"},
		{"Make it denser", "TABLET", "TABLET"},
		{"Support desktop and mobile", "TABLET", "TABLET"},
	} {
		if got := ResolveDevice(tc.prompt, tc.old).Name; got != tc.want {
			t.Errorf("%s: got %s want %s", tc.prompt, got, tc.want)
		}
	}
}

func TestVerificationRepairsThenRechecks(t *testing.T) {
	t.Setenv("FORGE_AI_ALLOW_PRIVATE_URLS", "true")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		text := `{"passed":false,"issues":["Phone width clips desktop content"]}`
		if calls == 2 {
			text = `<div class="w-full">Fixed desktop screen</div>`
		}
		if calls == 3 {
			text = `{"passed":true,"issues":[]}`
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": text}}}})
	}))
	defer srv.Close()
	out, summary, err := VerifyScreen(context.Background(), llm.Config{Kind: "openai", BaseURL: srv.URL, Model: "mock"}, "<div>old</div>", "desktop", "", DeviceFor("DESKTOP"))
	if err != nil || calls != 3 || !strings.Contains(out, "Fixed desktop") || summary != "Repaired and rechecked" {
		t.Fatalf("%q %q calls=%d err=%v", out, summary, calls, err)
	}
}

func TestKatalystCanvasUsesAuthoritativeTokens(t *testing.T) {
	doc, err := BuildCanvas(CanvasData{Title: "Test", Theme: "neon", DesignSystem: "Katalyst", Screens: []RenderedScreen{{ID: "s", HTML: "<div>Test</div>"}}})
	if err != nil || !strings.Contains(string(doc), "--radius: 0.45rem") {
		t.Fatalf("missing Katalyst overrides: %v", err)
	}
}
