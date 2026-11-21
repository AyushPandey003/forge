// Package stitch is a minimal client for Google Stitch's remote MCP server
// (https://stitch.googleapis.com/mcp). Forge uses it to turn a spec into UI
// screens that follow the Katalyst design system (DESIGN.md).
//
// Auth, in order of preference:
//   - STITCH_API_KEY            -> X-Goog-Api-Key
//   - STITCH_ACCESS_TOKEN       -> Authorization: Bearer (static)
//   - STITCH_USE_GCLOUD=true    -> `gcloud auth print-access-token` (dev),
//     with STITCH_PROJECT as the quota project.
//   - STITCH_CREDENTIALS_FILE   -> a Google "authorized_user" JSON
//     (client_id/secret + refresh_token), refreshed via golang.org/x/oauth2.
//     Used when the server runs in a container without gcloud.
package stitch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const DefaultEndpoint = "https://stitch.googleapis.com/mcp"

// ErrNotConfigured is returned when no Stitch credentials are set.
var ErrNotConfigured = errors.New("stitch is not configured")

type Config struct {
	Endpoint     string
	APIKey       string
	AccessToken  string
	UseGcloud    bool
	QuotaProject string
	// CredentialsFile: path to an authorized_user JSON.
	CredentialsFile string
}

func ConfigFromEnv() Config {
	return Config{
		Endpoint:        envOr("STITCH_BASE_URL", DefaultEndpoint),
		APIKey:          os.Getenv("STITCH_API_KEY"),
		AccessToken:     os.Getenv("STITCH_ACCESS_TOKEN"),
		UseGcloud:       os.Getenv("STITCH_USE_GCLOUD") == "true",
		QuotaProject:    os.Getenv("STITCH_PROJECT"),
		CredentialsFile: os.Getenv("STITCH_CREDENTIALS_FILE"),
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

type Client struct {
	cfg  Config
	http *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time
	ts       oauth2.TokenSource
}

func New(cfg Config) *Client {
	if cfg.Endpoint == "" {
		cfg.Endpoint = DefaultEndpoint
	}
	// Generation can take minutes; the caller's context bounds each call.
	c := &Client{cfg: cfg, http: &http.Client{Timeout: 15 * time.Minute}}
	if cfg.CredentialsFile != "" {
		if ts, err := authorizedUserTokenSource(cfg.CredentialsFile); err == nil {
			c.ts = ts
		} else {
			slog.Warn("stitch: could not load STITCH_CREDENTIALS_FILE", slog.Any("error", err))
		}
	}
	return c
}

func authorizedUserTokenSource(path string) (oauth2.TokenSource, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f struct {
		Type         string `json:"type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	if f.Type != "authorized_user" || f.RefreshToken == "" {
		return nil, errors.New("expected an authorized_user credentials file with a refresh_token")
	}
	conf := &oauth2.Config{ClientID: f.ClientID, ClientSecret: f.ClientSecret, Endpoint: google.Endpoint}
	return conf.TokenSource(context.Background(), &oauth2.Token{RefreshToken: f.RefreshToken}), nil
}

func (c *Client) Configured() bool {
	return c != nil && (c.cfg.APIKey != "" || c.cfg.AccessToken != "" || c.cfg.UseGcloud || c.ts != nil)
}

func (c *Client) authHeaders(ctx context.Context) (http.Header, error) {
	h := http.Header{}
	switch {
	case c.cfg.APIKey != "":
		h.Set("X-Goog-Api-Key", c.cfg.APIKey)
		return h, nil
	case c.cfg.AccessToken != "":
		h.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	case c.ts != nil:
		tok, err := c.ts.Token()
		if err != nil {
			return nil, fmt.Errorf("stitch credentials: %w", err)
		}
		h.Set("Authorization", "Bearer "+tok.AccessToken)
	case c.cfg.UseGcloud:
		tok, err := c.gcloudToken(ctx)
		if err != nil {
			return nil, err
		}
		h.Set("Authorization", "Bearer "+tok)
	default:
		return nil, ErrNotConfigured
	}
	if c.cfg.QuotaProject != "" {
		h.Set("X-Goog-User-Project", c.cfg.QuotaProject)
	}
	return h, nil
}

func (c *Client) gcloudToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}
	out, err := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token").Output()
	if err != nil {
		return "", fmt.Errorf("gcloud auth print-access-token: %w", err)
	}
	c.token = strings.TrimSpace(string(out))
	c.tokenExp = time.Now().Add(45 * time.Minute)
	return c.token, nil
}

// Error is a tool-level failure reported by Stitch.
type Error struct {
	Tool    string
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("stitch %s: %s", e.Tool, e.Message) }

// Call invokes an MCP tool and decodes its structured result into out.
func (c *Client) Call(ctx context.Context, tool string, args any, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	h, err := c.authHeaders(ctx)
	if err != nil {
		return err
	}
	req.Header = h
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("stitch %s: %w", tool, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return &Error{Tool: tool, Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 400))}
	}
	raw = sseData(raw)
	var env struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			IsError           bool            `json:"isError"`
			StructuredContent json.RawMessage `json:"structuredContent"`
			Content           []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("stitch %s: decode: %w", tool, err)
	}
	if env.Error != nil {
		return &Error{Tool: tool, Message: env.Error.Message}
	}
	if env.Result.IsError {
		var msg []string
		for _, ct := range env.Result.Content {
			msg = append(msg, ct.Text)
		}
		return &Error{Tool: tool, Message: strings.Join(msg, " ")}
	}
	if out == nil {
		return nil
	}
	if len(env.Result.StructuredContent) > 0 && string(env.Result.StructuredContent) != "null" {
		return json.Unmarshal(env.Result.StructuredContent, out)
	}
	for _, ct := range env.Result.Content {
		if ct.Type == "text" && json.Unmarshal([]byte(ct.Text), out) == nil {
			return nil
		}
	}
	return &Error{Tool: tool, Message: "empty result"}
}

// sseData extracts the JSON payload when the server answers as an event stream.
func sseData(raw []byte) []byte {
	s := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(s, "event:") && !strings.HasPrefix(s, "data:") {
		return raw
	}
	var last string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "data:") {
			last = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	return []byte(last)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ---- Typed helpers ----------------------------------------------------------

type Project struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

// ID returns the numeric project id without the "projects/" prefix.
func (p Project) ID() string { return strings.TrimPrefix(p.Name, "projects/") }

func (c *Client) CreateProject(ctx context.Context, title string) (Project, error) {
	var p Project
	err := c.Call(ctx, "create_project", map[string]any{"title": title}, &p)
	return p, err
}

type ScreenInstance struct {
	ID           string `json:"id"`
	SourceScreen string `json:"sourceScreen"`
}

// InstallDesignMD uploads a DESIGN.md and turns it into a project design
// system. Returns the design system asset name ("assets/<id>").
func (c *Client) InstallDesignMD(ctx context.Context, projectID string, designMD []byte) (string, error) {
	var inst ScreenInstance
	if err := c.Call(ctx, "upload_design_md", map[string]any{
		"projectId":      projectID,
		"designMdBase64": base64Std(designMD),
	}, &inst); err != nil {
		return "", err
	}
	var ds struct {
		AssetID string `json:"assetId"`
	}
	if err := c.Call(ctx, "create_design_system_from_design_md", map[string]any{
		"projectId":              projectID,
		"selectedScreenInstance": inst,
		"deviceType":             "DESKTOP",
	}, &ds); err != nil {
		return "", err
	}
	if ds.AssetID == "" {
		return "", &Error{Tool: "create_design_system_from_design_md", Message: "no asset id"}
	}
	return "assets/" + ds.AssetID, nil
}

type File struct {
	Name        string `json:"name"`
	DownloadURL string `json:"downloadUrl"`
}

type Screen struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	Prompt     string `json:"prompt"`
	DeviceType string `json:"deviceType"`
	Width      string `json:"width"`
	Height     string `json:"height"`
	HTMLCode   *File  `json:"htmlCode"`
	Screenshot *File  `json:"screenshot"`
}

// Generation is the result of generate/edit/variants: screens and/or a
// clarifying question from Stitch's agent.
type Generation struct {
	Screens     []Screen
	Text        string
	Suggestions []string
}

type generationRaw struct {
	OutputComponents []struct {
		Design *struct {
			Screens []Screen `json:"screens"`
		} `json:"design"`
		Text       string `json:"text"`
		Suggestion string `json:"suggestion"`
	} `json:"outputComponents"`
}

func (g generationRaw) flatten() Generation {
	var out Generation
	var text []string
	for _, oc := range g.OutputComponents {
		if oc.Design != nil {
			out.Screens = append(out.Screens, oc.Design.Screens...)
		}
		if t := strings.TrimSpace(oc.Text); t != "" {
			text = append(text, t)
		}
		if s := strings.TrimSpace(oc.Suggestion); s != "" {
			out.Suggestions = append(out.Suggestions, s)
		}
	}
	out.Text = strings.Join(text, "\n\n")
	return out
}

type GenerateInput struct {
	ProjectID    string
	Prompt       string
	DesignSystem string // "assets/<id>"
	DeviceType   string // DESKTOP | MOBILE | TABLET | AGNOSTIC
	ModelID      string // optional, e.g. GEMINI_3_1_PRO
}

func (c *Client) GenerateScreen(ctx context.Context, in GenerateInput) (Generation, error) {
	args := map[string]any{"projectId": in.ProjectID, "prompt": in.Prompt}
	if in.DesignSystem != "" {
		args["designSystem"] = in.DesignSystem
	}
	if in.DeviceType != "" {
		args["deviceType"] = in.DeviceType
	}
	if in.ModelID != "" {
		args["modelId"] = in.ModelID
	}
	var raw generationRaw
	if err := c.Call(ctx, "generate_screen_from_text", args, &raw); err != nil {
		return Generation{}, err
	}
	return raw.flatten(), nil
}

// EditScreen revises an existing screen from a text instruction.
func (c *Client) EditScreen(ctx context.Context, projectID, screenID, prompt, deviceType string) (Generation, error) {
	args := map[string]any{
		"projectId":         projectID,
		"selectedScreenIds": []string{screenID},
		"prompt":            prompt,
	}
	if deviceType != "" {
		args["deviceType"] = deviceType
	}
	var raw generationRaw
	if err := c.Call(ctx, "edit_screens", args, &raw); err != nil {
		return Generation{}, err
	}
	return raw.flatten(), nil
}

// Download fetches a Stitch file (HTML or screenshot). Stitch wraps the bytes
// in JSON: {"dataString": "...", "imageMetadata": {...}}; images are base64.
func (c *Client) Download(ctx context.Context, f *File) (data []byte, isImage bool, err error) {
	if f == nil || f.DownloadURL == "" {
		return nil, false, &Error{Tool: "download", Message: "no download url"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.DownloadURL, nil)
	if err != nil {
		return nil, false, err
	}
	h, err := c.authHeaders(ctx)
	if err != nil {
		return nil, false, err
	}
	req.Header = h
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, &Error{Tool: "download", Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	var wrapped struct {
		DataString    *string         `json:"dataString"`
		ImageMetadata json.RawMessage `json:"imageMetadata"`
	}
	if json.Unmarshal(raw, &wrapped) != nil || wrapped.DataString == nil {
		return raw, false, nil
	}
	if len(wrapped.ImageMetadata) > 0 {
		b, err := base64Decode(*wrapped.DataString)
		return b, true, err
	}
	return []byte(*wrapped.DataString), false, nil
}
