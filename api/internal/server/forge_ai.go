package server

// Bring-your-own-model support for Forge design generation. Users save
// provider connections (OpenAI-compatible or Anthropic) with their own API
// keys; keys are sealed at rest, never returned, and never logged. Generation
// with a saved provider produces a canvas document (see forge/design).

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/razorpay/slashdocs/api/internal/auth"
	forgepkg "github.com/razorpay/slashdocs/api/internal/forge"
	"github.com/razorpay/slashdocs/api/internal/forge/design"
	"github.com/razorpay/slashdocs/api/internal/llm"
)

const (
	maxProviderBody   = 16 * 1024
	maxActiveJobs     = 2
	keyDomain         = "forge/ai-provider-key/v1"
	forgeEngineStitch = "stitch"
	forgeEngineCanvas = "canvas"
)

func (h handlers) registerForgeAIRoutes(r *gin.RouterGroup) {
	r.GET("/forge/design/themes", h.forgeThemes)
	r.GET("/forge/ai/providers", h.aiListProviders)
	r.POST("/forge/ai/providers", h.aiCreateProvider)
	r.PUT("/forge/ai/providers/:id", h.aiUpdateProvider)
	r.DELETE("/forge/ai/providers/:id", h.aiDeleteProvider)
	r.GET("/forge/ai/providers/:id/models", h.aiModels)
}

// ---- Sealing ---------------------------------------------------------------

func sealKey(secret, plain string) ([]byte, error) {
	block, err := aes.NewCipher(keyFor(secret))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(plain), nil), nil
}

func openKey(secret string, sealed []byte) (string, error) {
	block, err := aes.NewCipher(keyFor(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(sealed) < gcm.NonceSize() {
		return "", errors.New("invalid key ciphertext")
	}
	plain, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
	return string(plain), err
}

func keyFor(secret string) []byte {
	sum := sha256.Sum256([]byte(keyDomain + "\x00" + secret))
	return sum[:]
}

// ---- Handlers --------------------------------------------------------------

// aiUser gates provider endpoints to signed-in people: service accounts must
// not hold or spend a person's model credentials.
func (h handlers) aiUser(c *gin.Context) (string, bool) {
	if !h.forgeEnabled(c) {
		return "", false
	}
	p := current(c)
	if p == nil || p.Type != auth.PrincipalUser {
		forbidden(c, "model providers can only be managed by signed-in users")
		return "", false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProviderBody)
	return p.ID, true
}

type providerInput struct {
	Label        string  `json:"label"`
	Kind         string  `json:"kind"`
	BaseURL      string  `json:"base_url"`
	APIKey       *string `json:"api_key"`
	DefaultModel string  `json:"default_model"`
}

func (in *providerInput) validate(needKey bool) (string, bool) {
	in.Label = strings.TrimSpace(in.Label)
	in.DefaultModel = strings.TrimSpace(in.DefaultModel)
	if in.Label == "" || len(in.Label) > 60 {
		return "name is required (60 characters max)", false
	}
	if in.Kind != llm.KindOpenAI && in.Kind != llm.KindAnthropic {
		return "provider type must be openai or anthropic", false
	}
	if len(in.DefaultModel) > 120 {
		return "model name is too long", false
	}
	base, err := llm.ValidateBaseURL(in.BaseURL)
	if err != nil {
		return err.Error(), false
	}
	in.BaseURL = base
	if in.APIKey != nil {
		k := strings.TrimSpace(*in.APIKey)
		in.APIKey = &k
		if len(k) > 512 || strings.ContainsAny(k, "\r\n") {
			return "API key is not valid", false
		}
	}
	if needKey && (in.APIKey == nil || *in.APIKey == "") && !isLocalURL(base) {
		return "API key is required", false
	}
	return "", true
}

// isLocalURL allows key-less providers (Ollama, LM Studio) on private hosts.
func isLocalURL(u string) bool {
	return allowPrivateAI() && (strings.HasPrefix(u, "http://") || strings.Contains(u, "localhost"))
}

func allowPrivateAI() bool {
	return strings.EqualFold(os.Getenv("FORGE_AI_ALLOW_PRIVATE_URLS"), "true")
}

func (h handlers) aiListProviders(c *gin.Context) {
	uid, ok := h.aiUser(c)
	if !ok {
		return
	}
	list, err := h.deps.forge.ListAIProviders(c.Request.Context(), uid)
	if err != nil {
		internalErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list, "allow_private_urls": allowPrivateAI(), "limit": forgepkg.MaxAIProviders})
}

func (h handlers) aiCreateProvider(c *gin.Context) {
	uid, ok := h.aiUser(c)
	if !ok {
		return
	}
	var in providerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid request body")
		return
	}
	if msg, ok := in.validate(true); !ok {
		badRequest(c, msg)
		return
	}
	p := forgepkg.AIProvider{Label: in.Label, Kind: in.Kind, BaseURL: in.BaseURL, DefaultModel: in.DefaultModel}
	if in.APIKey != nil && *in.APIKey != "" {
		sealed, err := sealKey(h.deps.sessionSecret, *in.APIKey)
		if err != nil {
			internalErr(c, err)
			return
		}
		p.KeyCiphertext, p.KeyHint = sealed, forgepkg.KeyHint(*in.APIKey)
	}
	out, err := h.deps.forge.CreateAIProvider(c.Request.Context(), uid, p)
	switch {
	case errors.Is(err, forgepkg.ErrConflict):
		c.JSON(http.StatusConflict, errorResponse("conflict", "you already have a provider with this name", nil))
	case errors.Is(err, forgepkg.ErrInvalid):
		badRequest(c, fmt.Sprintf("you can save at most %d providers", forgepkg.MaxAIProviders))
	case err != nil:
		internalErr(c, err)
	default:
		c.JSON(http.StatusCreated, out)
	}
}

func (h handlers) aiUpdateProvider(c *gin.Context) {
	uid, ok := h.aiUser(c)
	if !ok {
		return
	}
	var in providerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid request body")
		return
	}
	if msg, ok := in.validate(false); !ok {
		badRequest(c, msg)
		return
	}
	ctx := c.Request.Context()
	cur, err := h.deps.forge.GetAIProvider(ctx, uid, c.Param("id"))
	if err != nil {
		forgeErr(c, err)
		return
	}
	cur.Label, cur.Kind, cur.BaseURL, cur.DefaultModel = in.Label, in.Kind, in.BaseURL, in.DefaultModel
	var newKey []byte
	var hint string
	if in.APIKey != nil && *in.APIKey != "" {
		if newKey, err = sealKey(h.deps.sessionSecret, *in.APIKey); err != nil {
			internalErr(c, err)
			return
		}
		hint = forgepkg.KeyHint(*in.APIKey)
	}
	out, err := h.deps.forge.UpdateAIProvider(ctx, uid, cur, newKey, hint)
	switch {
	case errors.Is(err, forgepkg.ErrConflict):
		c.JSON(http.StatusConflict, errorResponse("conflict", "you already have a provider with this name", nil))
	case err != nil:
		forgeErr(c, err)
	default:
		c.JSON(http.StatusOK, out)
	}
}

func (h handlers) aiDeleteProvider(c *gin.Context) {
	uid, ok := h.aiUser(c)
	if !ok {
		return
	}
	if err := h.deps.forge.DeleteAIProvider(c.Request.Context(), uid, c.Param("id")); err != nil {
		forgeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h handlers) aiModels(c *gin.Context) {
	uid, ok := h.aiUser(c)
	if !ok {
		return
	}
	cfg, err := h.providerConfig(c.Request.Context(), uid, c.Param("id"), "")
	if err != nil {
		forgeErr(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	models, lerr := llm.ListModels(ctx, cfg)
	if lerr != nil {
		// Not fatal: many providers have no model list. The UI falls back to typing a name.
		c.JSON(http.StatusOK, gin.H{"models": []string{}, "error": lerr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (h handlers) forgeThemes(c *gin.Context) {
	type item struct {
		ID     string            `json:"id"`
		Name   string            `json:"name"`
		Colors map[string]string `json:"colors"`
	}
	var out []item
	for _, t := range design.Themes() {
		out = append(out, item{t.ID, t.Name, t.Colors})
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "default": design.DefaultTheme})
}

// providerConfig loads the caller's own provider and decrypts its key.
func (h handlers) providerConfig(ctx context.Context, userID, providerID, model string) (llm.Config, error) {
	p, err := h.deps.forge.GetAIProvider(ctx, userID, providerID)
	if err != nil {
		return llm.Config{}, err
	}
	key := ""
	if p.HasKey {
		if key, err = openKey(h.deps.sessionSecret, p.KeyCiphertext); err != nil {
			return llm.Config{}, fmt.Errorf("could not decrypt the saved API key; re-enter it: %w", forgepkg.ErrInvalid)
		}
	}
	if model == "" {
		model = p.DefaultModel
	}
	return llm.Config{Kind: p.Kind, BaseURL: p.BaseURL, APIKey: key, Model: model}, nil
}

// activeJobs counts a user's unfinished jobs so one person cannot fan out
// unbounded (and, for BYO keys, unbounded-cost) generations.
func (s *forgeState) activeJobs(ownerID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, j := range s.jobs {
		if j.ownerID == ownerID && (j.Status == "queued" || j.Status == "running") {
			n++
		}
	}
	return n
}

// ---- Canvas job ------------------------------------------------------------

type canvasJob struct {
	cfg        llm.Config
	providerID string
	model      string
	theme      string
	oneScreen  bool
	// revision
	reviseDocID string
	screenID    string
}

func (h handlers) runCanvasJob(job *forgeJob, p auth.Principal, in forgeDesignRequest, spec string, cj canvasJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	fail := func(err error) {
		var le *llm.Error
		msg := err.Error()
		if !errors.As(err, &le) {
			slog.Warn("forge canvas job failed", slog.String("job", job.ID), slog.Any("error", err))
			if !errors.Is(err, forgepkg.ErrInvalid) {
				msg = "Design generation failed. Please try again."
			}
		}
		forge.update(job, func(j *forgeJob) { j.Status, j.Error, j.FinishedAt = "error", msg, now() })
	}
	stage := func(s string) { forge.update(job, func(j *forgeJob) { j.Status, j.Stage = "running", s }) }
	dev := design.ResolveDevice(in.Prompt, in.DeviceType)

	designSystem := ""
	if strings.Contains(strings.ToLower(in.Prompt), "katalyst") {
		if len(forge.designMD) == 0 {
			fail(errors.New("Katalyst reference is unavailable on this server"))
			return
		}
		designSystem = "REQUIRED DESIGN SYSTEM: Katalyst. Follow this authoritative reference instead of decorative generic theme instructions:\n" + string(forge.designMD) + "\nAuthoritative Katalyst CSS token values:\n" + design.KatalystTokens
	}
	var data design.CanvasData
	title := strings.TrimSpace(in.Title)
	partial := ""

	if cj.reviseDocID != "" {
		stage("Reading current design")
		d, err := h.deps.repository.GetDocument(ctx, cj.reviseDocID)
		if err != nil {
			fail(err)
			return
		}
		src, err := h.deps.repository.GetVersionHTML(ctx, d.CurrentVersionID)
		if err != nil {
			fail(err)
			return
		}
		if data, err = design.ParseCanvas(src); err != nil {
			fail(err)
			return
		}
		dev = design.ResolveDevice(in.Prompt, data.Device)
		data.Device, data.Width, data.MinH = dev.Name, dev.Width, dev.MinHeight
		if designSystem == "" {
			designSystem = data.DesignSystem
		}
		data.DesignSystem = designSystem
		revised := 0
		for i := range data.Screens {
			if cj.screenID != "" && data.Screens[i].ID != cj.screenID {
				continue
			}
			stage(fmt.Sprintf("Revising %s with %s", data.Screens[i].Name, cj.cfg.Model))
			out, err := design.ReviseScreen(ctx, cj.cfg, data.Theme, data.Screens[i].Name, data.Screens[i].HTML, in.Prompt, dev, designSystem)
			if err != nil {
				if revised == 0 {
					fail(err)
					return
				}
				partial = fmt.Sprintf(" (revised %d of %d screens)", revised, len(data.Screens))
				break
			}
			stage("Checking requirements: " + data.Screens[i].Name)
			out, _, err = design.VerifyScreen(ctx, cj.cfg, out, in.Prompt, designSystem, dev)
			if err != nil {
				fail(err)
				return
			}
			data.Screens[i].HTML = out
			revised++
		}
		if revised == 0 {
			fail(errors.New("that screen no longer exists in this design"))
			return
		}
		if title == "" {
			title = data.Title
		}
	} else {
		stage("Planning screens with " + cj.cfg.Model)
		plan, err := design.Analyze(ctx, cj.cfg, design.Request{
			Prompt: in.Prompt, Spec: spec, Device: dev, ThemeID: cj.theme, OneScreen: cj.oneScreen, DesignSystem: designSystem,
		})
		if err != nil {
			fail(err)
			return
		}
		if title == "" {
			title = plan.Title
		}
		if title == "" {
			title = plan.Screens[0].Name
		}
		data = design.CanvasData{Title: title, Theme: plan.Theme, Device: dev.Name, Width: dev.Width, MinH: dev.MinHeight, DesignSystem: designSystem}
		var previous []string
		var lastErr error
		for i, sc := range plan.Screens {
			stage(fmt.Sprintf("Designing screen %d of %d: %s", i+1, len(plan.Screens), sc.Name))
			html, err := design.GenerateScreen(ctx, cj.cfg, plan, i, dev, previous, designSystem)
			if err != nil {
				lastErr = err
				if len(data.Screens) == 0 {
					fail(err)
					return
				}
				partial = fmt.Sprintf(" (%d of %d screens)", len(data.Screens), len(plan.Screens))
				break
			}
			stage("Checking requirements: " + sc.Name)
			html, _, err = design.VerifyScreen(ctx, cj.cfg, html, in.Prompt, designSystem, dev)
			if err != nil {
				fail(err)
				return
			}
			previous = append(previous, fmt.Sprintf("<!-- %s -->\n%s", sc.Name, html))
			data.Screens = append(data.Screens, design.RenderedScreen{ID: sc.ID, Name: sc.Name, HTML: html})
		}
		if lastErr != nil {
			slog.Info("forge canvas job published partial result", slog.String("job", job.ID), slog.Any("error", lastErr))
		}
	}

	stage("Building canvas")
	doc, err := design.BuildCanvas(data)
	if err != nil {
		fail(err)
		return
	}
	meta := forgeDesignMeta{
		Kind: "design", Engine: forgeEngineCanvas, Model: cj.model, Theme: data.Theme,
		Prompt: in.Prompt, DeviceType: dev.Name, SourceDocID: in.SourceDocID, UpdatedAt: now(),
	}
	if err := h.publishDesign(ctx, job, p, in, doc, title, cj.reviseDocID, meta, partial); err != nil {
		fail(err)
	}
}
