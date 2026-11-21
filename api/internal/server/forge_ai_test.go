package server

import (
	"strings"
	"testing"

	"github.com/razorpay/slashdocs/api/internal/auth"
	"github.com/razorpay/slashdocs/api/internal/llm"
)

func testPrincipal(id string) auth.Principal {
	return auth.Principal{Type: auth.PrincipalUser, ID: id}
}

func TestSealKeyRoundTrip(t *testing.T) {
	sealed, err := sealKey("secret-a", "sk-live-abcdef123456")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sealed), "sk-live") {
		t.Fatal("ciphertext must not contain the plaintext key")
	}
	got, err := openKey("secret-a", sealed)
	if err != nil || got != "sk-live-abcdef123456" {
		t.Fatalf("round trip: %q %v", got, err)
	}
	if _, err := openKey("secret-b", sealed); err == nil {
		t.Fatal("a different session secret must not decrypt the key")
	}
	if _, err := openKey("secret-a", sealed[:5]); err == nil {
		t.Fatal("truncated ciphertext must fail cleanly")
	}
	again, _ := sealKey("secret-a", "sk-live-abcdef123456")
	if string(again) == string(sealed) {
		t.Fatal("nonce must make each sealing unique")
	}
	// Domain separation: GitHub tokens and provider keys must not be interchangeable.
	gh, _ := sealGitHubToken("secret-a", "tok")
	if _, err := openKey("secret-a", gh); err == nil {
		t.Fatal("github-sealed data must not open as a provider key")
	}
}

func TestProviderInputValidate(t *testing.T) {
	t.Setenv("FORGE_AI_ALLOW_PRIVATE_URLS", "")
	key := "sk-abc"
	ok := providerInput{Label: "OR", Kind: llm.KindOpenAI, BaseURL: "https://openrouter.ai/api/v1/", APIKey: &key, DefaultModel: "m"}
	if msg, good := ok.validate(true); !good {
		t.Fatalf("valid input rejected: %s", msg)
	}
	if ok.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("base url not normalised: %q", ok.BaseURL)
	}
	nl := "a\nb"
	cases := map[string]providerInput{
		"no label":     {Kind: llm.KindOpenAI, BaseURL: "https://x.com", APIKey: &key},
		"long label":   {Label: strings.Repeat("a", 61), Kind: llm.KindOpenAI, BaseURL: "https://x.com", APIKey: &key},
		"bad kind":     {Label: "a", Kind: "azure", BaseURL: "https://x.com", APIKey: &key},
		"http":         {Label: "a", Kind: llm.KindOpenAI, BaseURL: "http://x.com", APIKey: &key},
		"metadata":     {Label: "a", Kind: llm.KindOpenAI, BaseURL: "https://169.254.169.254", APIKey: &key},
		"missing key":  {Label: "a", Kind: llm.KindOpenAI, BaseURL: "https://x.com"},
		"newline key":  {Label: "a", Kind: llm.KindOpenAI, BaseURL: "https://x.com", APIKey: &nl},
		"long model":   {Label: "a", Kind: llm.KindOpenAI, BaseURL: "https://x.com", APIKey: &key, DefaultModel: strings.Repeat("m", 121)},
		"creds in url": {Label: "a", Kind: llm.KindOpenAI, BaseURL: "https://u:p@x.com", APIKey: &key},
	}
	for name, in := range cases {
		if _, good := in.validate(true); good {
			t.Errorf("%s: expected rejection", name)
		}
	}
	// Editing without a new key is fine.
	edit := providerInput{Label: "a", Kind: llm.KindAnthropic, BaseURL: "https://api.anthropic.com"}
	if msg, good := edit.validate(false); !good {
		t.Errorf("edit without key rejected: %s", msg)
	}
}

func TestActiveJobsCountsOnlyOwnUnfinished(t *testing.T) {
	s := newForgeState()
	a := s.newJob(testPrincipal("u1"))
	s.newJob(testPrincipal("u1"))
	s.newJob(testPrincipal("u2"))
	s.update(a, func(j *forgeJob) { j.Status = "done" })
	if got := s.activeJobs("u1"); got != 1 {
		t.Fatalf("u1 active = %d, want 1", got)
	}
	if got := s.activeJobs("nobody"); got != 0 {
		t.Fatalf("nobody active = %d", got)
	}
}
