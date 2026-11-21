package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateBaseURL(t *testing.T) {
	t.Setenv("FORGE_AI_ALLOW_PRIVATE_URLS", "")
	for _, bad := range []string{"", "ftp://x.com", "http://api.example.com", "https://user:pw@x.com", "https://169.254.169.254", "https://127.0.0.1", "https://10.0.0.5/v1", "https://[::1]/v1", "not a url"} {
		if _, err := ValidateBaseURL(bad); err == nil {
			t.Errorf("expected %q to be rejected", bad)
		}
	}
	if got, err := ValidateBaseURL(" https://api.openai.com/v1/ "); err != nil || got != "https://api.openai.com/v1" {
		t.Errorf("got %q %v", got, err)
	}
	t.Setenv("FORGE_AI_ALLOW_PRIVATE_URLS", "true")
	if _, err := ValidateBaseURL("http://localhost:11434/v1"); err != nil {
		t.Errorf("private allowed: %v", err)
	}
	if _, err := ValidateBaseURL("http://169.254.169.254"); err == nil {
		t.Error("metadata address must stay blocked even in private mode")
	}
}

func TestCompleteOpenAIAndScrub(t *testing.T) {
	t.Setenv("FORGE_AI_ALLOW_PRIVATE_URLS", "true")
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		if strings.Contains(r.Header.Get("X-Fail"), "1") {
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer srv.Close()
	out, err := Complete(context.Background(), Config{Kind: KindOpenAI, BaseURL: srv.URL + "/v1", APIKey: "sk-secret-123456", Model: "m"}, "s", "u")
	if err != nil || out != "hello" || gotAuth != "Bearer sk-secret-123456" || gotPath != "/v1/chat/completions" {
		t.Fatalf("out=%q err=%v auth=%q path=%q", out, err, gotAuth, gotPath)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key sk-secret-123456"}}`))
	}))
	defer bad.Close()
	_, err = Complete(context.Background(), Config{Kind: KindOpenAI, BaseURL: bad.URL, APIKey: "sk-secret-123456", Model: "m"}, "s", "u")
	if err == nil || strings.Contains(err.Error(), "sk-secret-123456") || !strings.Contains(err.Error(), "401") {
		t.Fatalf("key must be scrubbed from errors: %v", err)
	}
}

func TestCompleteAnthropic(t *testing.T) {
	t.Setenv("FORGE_AI_ALLOW_PRIVATE_URLS", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k-123456" || r.URL.Path != "/v1/messages" {
			w.WriteHeader(400)
			return
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"a"},{"type":"tool_use"},{"type":"text","text":"b"}]}`))
	}))
	defer srv.Close()
	out, err := Complete(context.Background(), Config{Kind: KindAnthropic, BaseURL: srv.URL, APIKey: "k-123456", Model: "m"}, "s", "u")
	if err != nil || out != "ab" {
		t.Fatalf("%q %v", out, err)
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	t.Setenv("FORGE_AI_ALLOW_PRIVATE_URLS", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest", http.StatusFound)
	}))
	defer srv.Close()
	if _, err := Complete(context.Background(), Config{Kind: KindOpenAI, BaseURL: srv.URL, Model: "m"}, "s", "u"); err == nil {
		t.Fatal("redirect must surface as an error, not be followed")
	}
}

func TestPrivateBlockedAtDialTime(t *testing.T) {
	t.Setenv("FORGE_AI_ALLOW_PRIVATE_URLS", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("{}")) }))
	defer srv.Close()
	// localhost resolves to loopback: the hostname passes URL validation but the
	// dial-time check must still refuse it.
	u := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	if _, err := Complete(context.Background(), Config{Kind: KindOpenAI, BaseURL: u, Model: "m"}, "s", "u"); err == nil {
		t.Fatal("dialling a loopback address must be refused")
	}
}

func TestExtractJSON(t *testing.T) {
	got, ok := ExtractJSON("pre ```json\n{\"a\":\"}\",\"b\":{\"c\":1}}\n``` post")
	if !ok || got != `{"a":"}","b":{"c":1}}` {
		t.Fatalf("%q %v", got, ok)
	}
	if _, ok := ExtractJSON("none"); ok {
		t.Fatal("expected no json")
	}
}
