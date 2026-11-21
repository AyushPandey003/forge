// Package llm is a small client for bring-your-own-key model providers.
// It speaks the two wire formats that cover almost every provider: OpenAI-style
// chat completions (OpenAI, OpenRouter, Ollama, vLLM, LM Studio, ...) and the
// Anthropic Messages API.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

const (
	KindOpenAI    = "openai"
	KindAnthropic = "anthropic"

	maxResponseBytes = 8 << 20
	callTimeout      = 5 * time.Minute
)

// Config describes one provider call. APIKey is secret: it is never logged
// and is scrubbed from upstream error text.
type Config struct {
	Kind    string
	BaseURL string
	APIKey  string
	Model   string
}

// Error is an upstream failure that is safe to show to the user.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("model provider returned %d: %s", e.Status, e.Message)
	}
	return e.Message
}

func allowPrivate() bool { return os.Getenv("FORGE_AI_ALLOW_PRIVATE_URLS") == "true" }

// ValidateBaseURL rejects URLs a user must not be able to point the server at.
// The dial-time check in newHTTPClient is the real guard (it sees the resolved
// address); this gives an early, friendly error.
func ValidateBaseURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errors.New("base URL must be a full http(s) URL")
	}
	if u.User != nil {
		return "", errors.New("base URL must not contain credentials")
	}
	// Plain http and private addresses (Ollama, LM Studio, vLLM) are only for
	// self-hosters who opt in with FORGE_AI_ALLOW_PRIVATE_URLS=true.
	if u.Scheme == "http" && !allowPrivate() {
		return "", errors.New("base URL must use https")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && isBlocked(ip, !allowPrivate()) {
		return "", errors.New("base URL points to a blocked address")
	}
	return raw, nil
}

// isBlocked reports addresses never reachable (link-local incl. cloud
// metadata, unspecified) and, when private is true, loopback/private ranges.
func isBlocked(ip net.IP, private bool) bool {
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	return private && (ip.IsLoopback() || ip.IsPrivate())
}

func newHTTPClient() *http.Client {
	private := !allowPrivate()
	dialer := &net.Dialer{
		Timeout: 15 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || isBlocked(ip, private) {
				return errors.New("connection to this address is not allowed")
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: callTimeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			Proxy:                 nil,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: callTimeout,
		},
		// Never follow redirects: a provider URL must not bounce us inward.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// Complete sends one system+user exchange and returns the model's text.
func Complete(ctx context.Context, cfg Config, system, user string) (string, error) {
	if strings.TrimSpace(cfg.Model) == "" {
		return "", &Error{Message: "no model selected"}
	}
	base, err := ValidateBaseURL(cfg.BaseURL)
	if err != nil {
		return "", &Error{Message: err.Error()}
	}
	var (
		endpoint string
		body     any
		headers  = map[string]string{"Content-Type": "application/json"}
	)
	switch cfg.Kind {
	case KindAnthropic:
		endpoint = base + "/v1/messages"
		headers["x-api-key"] = cfg.APIKey
		headers["anthropic-version"] = "2023-06-01"
		body = map[string]any{
			"model": cfg.Model, "max_tokens": 16000, "system": system,
			"messages": []map[string]string{{"role": "user", "content": user}},
		}
	case KindOpenAI:
		endpoint = base + "/chat/completions"
		if cfg.APIKey != "" {
			headers["Authorization"] = "Bearer " + cfg.APIKey
		}
		body = map[string]any{
			"model": cfg.Model,
			"messages": []map[string]string{
				{"role": "system", "content": system},
				{"role": "user", "content": user},
			},
		}
	default:
		return "", &Error{Message: "unknown provider kind"}
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", &Error{Message: "invalid provider URL"}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := newHTTPClient().Do(req)
	if err != nil {
		return "", &Error{Message: scrub("could not reach the model provider: "+rootCause(err), cfg.APIKey)}
	}
	defer func() { _ = res.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return "", &Error{Message: "could not read the model response"}
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", &Error{Status: res.StatusCode, Message: scrub(upstreamMessage(data), cfg.APIKey)}
	}
	text, err := extractText(cfg.Kind, data)
	if err != nil {
		return "", &Error{Message: err.Error()}
	}
	return text, nil
}

func extractText(kind string, data []byte) (string, error) {
	if kind == KindAnthropic {
		var r struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(data, &r) != nil {
			return "", errors.New("the model returned an unreadable response")
		}
		var b strings.Builder
		for _, c := range r.Content {
			if c.Type == "text" {
				b.WriteString(c.Text)
			}
		}
		if strings.TrimSpace(b.String()) == "" {
			return "", errors.New("the model returned an empty response")
		}
		return b.String(), nil
	}
	var r struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(data, &r) != nil || len(r.Choices) == 0 {
		return "", errors.New("the model returned an unreadable response")
	}
	var text string
	switch c := r.Choices[0].Message.Content.(type) {
	case string:
		text = c
	case []any: // content parts
		for _, p := range c {
			if m, ok := p.(map[string]any); ok {
				if s, ok := m["text"].(string); ok {
					text += s
				}
			}
		}
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New("the model returned an empty response")
	}
	return text, nil
}

func upstreamMessage(data []byte) string {
	var e struct {
		Error any    `json:"error"`
		Msg   string `json:"message"`
	}
	msg := ""
	if json.Unmarshal(data, &e) == nil {
		switch v := e.Error.(type) {
		case string:
			msg = v
		case map[string]any:
			if s, ok := v["message"].(string); ok {
				msg = s
			}
		}
		if msg == "" {
			msg = e.Msg
		}
	}
	if msg == "" {
		msg = strings.TrimSpace(string(data))
	}
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	if msg == "" {
		msg = "no details"
	}
	return msg
}

func rootCause(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return err.Error()
}

func scrub(s, secret string) string {
	if len(secret) >= 6 {
		s = strings.ReplaceAll(s, secret, "[redacted]")
	}
	return s
}

// ExtractJSON pulls the first top-level JSON object out of model text, which
// may be wrapped in prose or a code fence.
func ExtractJSON(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr:
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

// ListModels asks the provider which models it offers. Providers that do not
// implement the endpoint return an error; callers should fall back to free text.
func ListModels(ctx context.Context, cfg Config) ([]string, error) {
	base, err := ValidateBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, &Error{Message: "invalid provider URL"}
	}
	if cfg.Kind == KindAnthropic {
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.URL.Path = strings.TrimSuffix(req.URL.Path, "/models") + "/v1/models"
	} else if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	ctx2, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	res, err := newHTTPClient().Do(req.WithContext(ctx2))
	if err != nil {
		return nil, &Error{Message: scrub("could not reach the model provider: "+rootCause(err), cfg.APIKey)}
	}
	defer func() { _ = res.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, &Error{Status: res.StatusCode, Message: scrub(upstreamMessage(data), cfg.APIKey)}
	}
	var r struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &r) != nil {
		return nil, &Error{Message: "the provider returned an unreadable model list"}
	}
	var out []string
	for _, m := range r.Data {
		if m.ID != "" {
			out = append(out, m.ID)
		}
	}
	return out, nil
}
