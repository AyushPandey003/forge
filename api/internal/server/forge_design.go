package server

// Forge design: turn a prompt (usually a spec) into UI screens with Google
// Stitch, constrained by the Katalyst design system, and publish each result
// as a reviewable document (or a new version of one). Comments then flow back
// into Stitch as revision prompts.
//
// Generation takes ~1-3 minutes, so it runs as an in-process job; clients
// poll GET /v1/forge/jobs/:id. Jobs are kept in memory (single-node dev);
// the durable record is the published document + its KV metadata.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/razorpay/slashdocs/api/internal/auth"
	forgepkg "github.com/razorpay/slashdocs/api/internal/forge"
	"github.com/razorpay/slashdocs/api/internal/forge/design"
	"github.com/razorpay/slashdocs/api/internal/repo"
	"github.com/razorpay/slashdocs/api/internal/stitch"
)

// KV keys stored on a Forge design document.
const (
	forgeKVDesign = "forge.design"
)

type forgeDesignMeta struct {
	Kind          string `json:"kind"`             // "design"
	Engine        string `json:"engine,omitempty"` // "stitch" (default) or "canvas"
	Model         string `json:"model,omitempty"`  // canvas engine: model used
	Theme         string `json:"theme,omitempty"`  // canvas engine: theme id
	StitchProject string `json:"stitch_project,omitempty"`
	StitchScreen  string `json:"stitch_screen,omitempty"`
	DesignSystem  string `json:"design_system,omitempty"`
	Prompt        string `json:"prompt"`
	DeviceType    string `json:"device_type"`
	SourceDocID   string `json:"source_doc_id,omitempty"`
	UpdatedAt     string `json:"updated_at"`
}

type forgeJob struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"` // queued | running | done | needs_input | error
	Stage       string   `json:"stage"`
	Error       string   `json:"error,omitempty"`
	Question    string   `json:"question,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	DocumentID  string   `json:"document_id,omitempty"`
	VersionID   string   `json:"version_id,omitempty"`
	StartedAt   string   `json:"started_at"`
	FinishedAt  string   `json:"finished_at,omitempty"`
	ownerID     string
}

// forgeState is process-wide: the Stitch client, the shared Stitch project
// that holds the Katalyst design system, and running jobs.
type forgeState struct {
	client   *stitch.Client
	designMD []byte

	mu        sync.Mutex
	projectID string
	assetName string
	setupErr  error
	jobs      map[string]*forgeJob
}

var forge = newForgeState()

func newForgeState() *forgeState {
	s := &forgeState{client: stitch.New(stitch.ConfigFromEnv()), jobs: map[string]*forgeJob{}}
	s.projectID = os.Getenv("FORGE_STITCH_PROJECT_ID")
	if a := os.Getenv("FORGE_STITCH_DESIGN_SYSTEM"); a != "" {
		s.assetName = a
	}
	if path := os.Getenv("FORGE_DESIGN_MD"); path != "" {
		if b, err := os.ReadFile(path); err == nil {
			s.designMD = b
		} else {
			slog.Warn("forge: could not read FORGE_DESIGN_MD", slog.String("path", path), slog.Any("error", err))
		}
	}
	return s
}

// ensureProject lazily creates the shared Stitch project and installs the
// design system from DESIGN.md. Safe for concurrent callers.
func (s *forgeState) ensureProject(ctx context.Context) (projectID, asset string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.projectID != "" && (s.assetName != "" || len(s.designMD) == 0) {
		return s.projectID, s.assetName, nil
	}
	if s.projectID == "" {
		p, err := s.client.CreateProject(ctx, "Forge workspace")
		if err != nil {
			return "", "", err
		}
		s.projectID = p.ID()
		slog.Info("forge: created stitch project; set FORGE_STITCH_PROJECT_ID to reuse it", slog.String("id", s.projectID))
	}
	if s.assetName == "" && len(s.designMD) > 0 {
		a, err := s.client.InstallDesignMD(ctx, s.projectID, s.designMD)
		if err != nil {
			return s.projectID, "", fmt.Errorf("install DESIGN.md: %w", err)
		}
		s.assetName = a
		slog.Info("forge: installed design system; set FORGE_STITCH_DESIGN_SYSTEM to reuse it", slog.String("asset", a))
	}
	return s.projectID, s.assetName, nil
}

func (s *forgeState) newJob(owner auth.Principal) *forgeJob {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	j := &forgeJob{ID: hex.EncodeToString(b), Status: "queued", Stage: "Queued", StartedAt: now(), ownerID: owner.ID}
	s.mu.Lock()
	s.jobs[j.ID] = j
	s.mu.Unlock()
	return j
}

func (s *forgeState) update(j *forgeJob, fn func(*forgeJob)) {
	s.mu.Lock()
	fn(j)
	s.mu.Unlock()
}

func (s *forgeState) snapshot(id string) (forgeJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return forgeJob{}, false
	}
	return *j, true
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// ---- HTTP ------------------------------------------------------------------

type forgeStatusResponse struct {
	Enabled bool `json:"enabled"`
	// CustomModels is true when users can bring their own model provider.
	CustomModels  bool   `json:"custom_models"`
	DesignSystem  string `json:"design_system"`
	DesignMDBytes int    `json:"design_md_bytes"`
	StitchProject string `json:"stitch_project,omitempty"`
}

func (h handlers) forgeStatus(c *gin.Context) {
	forge.mu.Lock()
	defer forge.mu.Unlock()
	name := "Stitch default"
	if len(forge.designMD) > 0 {
		name = "Katalyst (DESIGN.md)"
	}
	c.JSON(http.StatusOK, forgeStatusResponse{
		Enabled:       forge.client.Configured(),
		CustomModels:  h.deps.forge != nil && current(c) != nil && current(c).Type == auth.PrincipalUser,
		DesignSystem:  name,
		DesignMDBytes: len(forge.designMD),
		StitchProject: forge.projectID,
	})
}

type forgeDesignRequest struct {
	Prompt      string `json:"prompt"`
	Title       string `json:"title"`
	DeviceType  string `json:"device_type"`
	ModelID     string `json:"model_id"`
	SourceDocID string `json:"source_doc_id"`
	// Bring-your-own-model: when ProviderID is set the canvas engine is used
	// instead of Stitch.
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
	Theme      string `json:"theme"`
	OneScreen  bool   `json:"one_screen"`
	// Optional: attach the published design to a solution stage.
	SolutionID string `json:"solution_id"`
	Stage      string `json:"stage"`
	// resolve lists comment ids addressed by a revision (internal).
	resolve []string
}

// POST /v1/forge/designs: generate a new design document.
func (h handlers) forgeCreateDesign(c *gin.Context) {
	var in forgeDesignRequest
	if err := c.ShouldBindJSON(&in); err != nil || strings.TrimSpace(in.Prompt) == "" {
		badRequest(c, "prompt is required")
		return
	}
	if in.ProviderID == "" && !forge.client.Configured() {
		c.JSON(http.StatusServiceUnavailable, errorResponse("forge_disabled", "Stitch is not configured on this server. Connect your own model provider to generate designs.", nil))
		return
	}
	in.DeviceType = normalizeDevice(in.DeviceType)
	p := *current(c)
	var sourceSpec string
	if in.SourceDocID != "" {
		if !h.needDocRole(c, in.SourceDocID, repo.RoleViewer) {
			return
		}
		sourceSpec = h.docPlainText(c.Request.Context(), in.SourceDocID)
	}
	if in.SolutionID != "" && h.deps.forge != nil {
		roles, err := h.deps.forge.RolesOf(c.Request.Context(), in.SolutionID, p.ID)
		sol, serr := h.deps.forge.GetSolution(c.Request.Context(), in.SolutionID)
		if err != nil || serr != nil || (len(roles) == 0 && sol.CreatedBy.ID != p.ID) {
			notFound(c)
			return
		}
		// Default spec: the solution's PRD, if one exists.
		if sourceSpec == "" {
			if arts, err := h.deps.forge.Artifacts(c.Request.Context(), sol.ID); err == nil {
				for _, a := range arts {
					if a.Kind == "prd" && a.DocumentID != "" {
						sourceSpec = h.docPlainText(c.Request.Context(), a.DocumentID)
						break
					}
				}
			}
		}
	}
	if in.ProviderID != "" {
		cj, ok := h.prepareCanvasJob(c, p, in)
		if !ok {
			return
		}
		cj.theme, cj.oneScreen = in.Theme, in.OneScreen
		job := forge.newJob(p)
		go h.runCanvasJob(job, p, in, sourceSpec, cj)
		c.JSON(http.StatusAccepted, job)
		return
	}
	job := forge.newJob(p)
	go h.runDesignJob(job, p, in, sourceSpec, "", "")
	c.JSON(http.StatusAccepted, job)
}

// prepareCanvasJob validates a bring-your-own-model request and resolves the
// caller's provider. It writes the error response itself.
func (h handlers) prepareCanvasJob(c *gin.Context, p auth.Principal, in forgeDesignRequest) (canvasJob, bool) {
	if p.Type != auth.PrincipalUser || h.deps.forge == nil {
		forbidden(c, "model providers are only available to signed-in users")
		return canvasJob{}, false
	}
	if in.Theme != "" && !design.KnownTheme(in.Theme) {
		badRequest(c, "unknown theme")
		return canvasJob{}, false
	}
	if forge.activeJobs(p.ID) >= maxActiveJobs {
		c.JSON(http.StatusTooManyRequests, errorResponse("too_many_jobs", "You already have designs generating. Wait for one to finish.", nil))
		return canvasJob{}, false
	}
	cfg, err := h.providerConfig(c.Request.Context(), p.ID, in.ProviderID, strings.TrimSpace(in.Model))
	if err != nil {
		if errors.Is(err, forgepkg.ErrNotFound) {
			badRequest(c, "that model provider no longer exists")
		} else {
			forgeErr(c, err)
		}
		return canvasJob{}, false
	}
	if cfg.Model == "" {
		badRequest(c, "choose a model")
		return canvasJob{}, false
	}
	return canvasJob{cfg: cfg, providerID: in.ProviderID, model: cfg.Model}, true
}

type forgeReviseRequest struct {
	Prompt     string   `json:"prompt"`
	CommentIDs []string `json:"comment_ids"`
	// Canvas designs: the model to use, and optionally one screen to revise.
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
	ScreenID   string `json:"screen_id"`
}

// POST /v1/forge/designs/:id/revise: apply feedback (a prompt and/or open
// comments) to the design and publish the result as a new version.
func (h handlers) forgeReviseDesign(c *gin.Context) {
	docID := c.Param("id")
	if !h.needDocRole(c, docID, repo.RoleEditor) {
		return
	}
	var in forgeReviseRequest
	_ = c.ShouldBindJSON(&in)
	ctx := c.Request.Context()
	meta, err := h.loadDesignMeta(ctx, docID)
	if err != nil {
		badRequest(c, "this document is not a Forge design")
		return
	}
	if meta.Engine != forgeEngineCanvas && !forge.client.Configured() {
		c.JSON(http.StatusServiceUnavailable, errorResponse("forge_disabled", "Stitch is not configured on this server", nil))
		return
	}
	feedback := strings.TrimSpace(in.Prompt)
	if fb := h.commentFeedback(ctx, docID, in.CommentIDs); fb != "" {
		if feedback != "" {
			feedback += "\n\n"
		}
		feedback += "Reviewer comments to address:\n" + fb
	}
	if feedback == "" {
		badRequest(c, "nothing to revise: add a prompt or leave open comments")
		return
	}
	p := *current(c)
	ids := h.commentIDs(ctx, docID, in.CommentIDs)
	if meta.Engine == forgeEngineCanvas {
		if in.ProviderID == "" {
			badRequest(c, "choose a model provider to revise this design")
			return
		}
		req := forgeDesignRequest{Prompt: feedback, DeviceType: meta.DeviceType, ProviderID: in.ProviderID, Model: in.Model, resolve: ids}
		cj, ok := h.prepareCanvasJob(c, p, req)
		if !ok {
			return
		}
		cj.reviseDocID, cj.screenID = docID, in.ScreenID
		job := forge.newJob(p)
		go h.runCanvasJob(job, p, req, "", cj)
		c.JSON(http.StatusAccepted, job)
		return
	}
	job := forge.newJob(p)
	go h.runDesignJob(job, p, forgeDesignRequest{Prompt: feedback, DeviceType: meta.DeviceType, resolve: ids}, "", docID, meta.StitchScreen)
	c.JSON(http.StatusAccepted, job)
}

// GET /v1/forge/jobs/:id
func (h handlers) forgeJobStatus(c *gin.Context) {
	j, ok := forge.snapshot(c.Param("id"))
	if !ok || j.ownerID != current(c).ID {
		notFound(c)
		return
	}
	c.JSON(http.StatusOK, j)
}

// GET /v1/forge/designs/:id: design metadata for the document page.
func (h handlers) forgeGetDesign(c *gin.Context) {
	docID := c.Param("id")
	if !h.needDocRole(c, docID, repo.RoleViewer) {
		return
	}
	meta, err := h.loadDesignMeta(c.Request.Context(), docID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"design": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"design": meta})
}

// ---- Job body --------------------------------------------------------------

func (h handlers) runDesignJob(job *forgeJob, p auth.Principal, in forgeDesignRequest, sourceSpec, reviseDocID, reviseScreen string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	fail := func(err error) {
		slog.Warn("forge job failed", slog.String("job", job.ID), slog.Any("error", err))
		forge.update(job, func(j *forgeJob) { j.Status, j.Error, j.FinishedAt = "error", err.Error(), now() })
	}
	stage := func(s string) { forge.update(job, func(j *forgeJob) { j.Status, j.Stage = "running", s }) }

	stage("Preparing Katalyst design system")
	projectID, asset, err := forge.ensureProject(ctx)
	if err != nil {
		fail(err)
		return
	}

	var gen stitch.Generation
	prompt := buildDesignPrompt(in.Prompt, sourceSpec)
	if reviseScreen != "" {
		stage("Revising design in Stitch")
		gen, err = forge.client.EditScreen(ctx, projectID, screenIDOf(reviseScreen), prompt, in.DeviceType)
	} else {
		stage("Generating screens in Stitch")
		gen, err = forge.client.GenerateScreen(ctx, stitch.GenerateInput{
			ProjectID: projectID, Prompt: prompt, DesignSystem: asset,
			DeviceType: in.DeviceType, ModelID: in.ModelID,
		})
	}
	if err != nil {
		fail(err)
		return
	}
	if len(gen.Screens) == 0 {
		forge.update(job, func(j *forgeJob) {
			j.Status, j.Question, j.Suggestions, j.FinishedAt = "needs_input", gen.Text, gen.Suggestions, now()
			if j.Question == "" {
				j.Status, j.Error = "error", "Stitch returned no screens"
			}
		})
		return
	}

	stage("Downloading design")
	screen := gen.Screens[0]
	htmlBytes, _, err := forge.client.Download(ctx, screen.HTMLCode)
	if err != nil {
		fail(err)
		return
	}
	doc := wrapDesignHTML(htmlBytes, screen, in.Prompt)

	meta := forgeDesignMeta{
		Kind: "design", Engine: forgeEngineStitch, StitchProject: projectID, StitchScreen: screen.Name, DesignSystem: asset,
		Prompt: in.Prompt, DeviceType: in.DeviceType, SourceDocID: in.SourceDocID, UpdatedAt: now(),
	}
	if err := h.publishDesign(ctx, job, p, in, doc, screen.Title, reviseDocID, meta, ""); err != nil {
		fail(err)
	}
}

// commentIDs returns the root ids of the open comments a revision addresses.
func (h handlers) commentIDs(ctx context.Context, docID string, want []string) []string {
	comments, err := h.deps.repository.ListComments(ctx, docID, "open", "")
	if err != nil {
		return nil
	}
	sel := map[string]bool{}
	for _, id := range want {
		sel[id] = true
	}
	var out []string
	for _, cm := range comments {
		if cm.ParentCommentID != "" {
			continue
		}
		if len(sel) == 0 || sel[cm.ID] {
			out = append(out, cm.ID)
		}
	}
	return out
}

// closeAddressedComments replies on each addressed thread with a pointer to
// the new version, then resolves it, so the review loop closes visibly.
func (h handlers) closeAddressedComments(ctx context.Context, docID, versionID string, ids []string, p auth.Principal, meta forgeDesignMeta) {
	v, err := h.deps.repository.GetVersion(ctx, versionID)
	label := "a new version"
	if err == nil {
		label = fmt.Sprintf("v%d", v.Number)
	}
	for _, id := range ids {
		how := "Stitch + Katalyst"
		if meta.Engine == forgeEngineCanvas && meta.Model != "" {
			how = meta.Model
		}
		body := "Addressed by Forge in " + label + " (generated with " + how + ")."
		if _, err := h.deps.repository.CreateCommentReply(ctx, docID, id, body, p, nil); err != nil {
			slog.Warn("forge: reply failed", slog.String("comment", id), slog.Any("error", err))
		}
		if _, err := h.deps.repository.UpdateComment(ctx, id, repo.CommentUpdate{Status: "resolved"}); err != nil {
			slog.Warn("forge: resolve failed", slog.String("comment", id), slog.Any("error", err))
		}
	}
	_ = h.deps.repository.AppendDocumentEvent(ctx, docID, repo.DocumentEventComments, nil, p)
}

// ---- Helpers ---------------------------------------------------------------

func normalizeDevice(d string) string {
	switch strings.ToUpper(d) {
	case "MOBILE", "TABLET", "AGNOSTIC":
		return strings.ToUpper(d)
	default:
		return "DESKTOP"
	}
}

func screenIDOf(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

func firstLine(s string, n int) string {
	s = strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
	if len(s) > n {
		s = s[:n] + "…"
	}
	return s
}

func buildDesignPrompt(prompt, spec string) string {
	if spec == "" {
		return prompt
	}
	if len(spec) > 12000 {
		spec = spec[:12000]
	}
	return prompt + "\n\nProduct spec to design from:\n" + spec
}

// wrapDesignHTML tags the Stitch HTML so the document is recognisable as a
// Forge design even outside Forge (and keeps it self-contained).
func wrapDesignHTML(src []byte, s stitch.Screen, prompt string) []byte {
	meta := fmt.Sprintf(`<meta name="forge:kind" content="design"><meta name="forge:stitch-screen" content="%s"><meta name="forge:prompt" content="%s">`,
		html.EscapeString(s.Name), html.EscapeString(firstLine(prompt, 300)))
	if i := bytes.Index(bytes.ToLower(src), []byte("<head>")); i >= 0 {
		out := append([]byte{}, src[:i+6]...)
		out = append(out, meta...)
		return append(out, src[i+6:]...)
	}
	return append([]byte(meta), src...)
}

func (h handlers) loadDesignMeta(ctx context.Context, docID string) (forgeDesignMeta, error) {
	snap, err := h.deps.repository.ListDocumentKV(ctx, docID)
	if err != nil {
		return forgeDesignMeta{}, err
	}
	raw, ok := snap.Items[forgeKVDesign]
	if !ok {
		return forgeDesignMeta{}, errors.New("not a design")
	}
	var m forgeDesignMeta
	if err := json.Unmarshal(raw, &m); err != nil || (m.StitchScreen == "" && m.Engine != forgeEngineCanvas) {
		return forgeDesignMeta{}, errors.New("not a design")
	}
	return m, nil
}

// commentFeedback renders open comments (or the selected ones) as a bullet
// list Stitch can act on.
func (h handlers) commentFeedback(ctx context.Context, docID string, ids []string) string {
	comments, err := h.deps.repository.ListComments(ctx, docID, "open", "")
	if err != nil {
		return ""
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var b strings.Builder
	var walk func(cs []repo.Comment)
	walk = func(cs []repo.Comment) {
		for _, cm := range cs {
			if len(want) > 0 && !want[cm.ID] && !want[cm.RootCommentID] {
				continue
			}
			b.WriteString("- ")
			if q := strings.TrimSpace(cm.SelectedText); q != "" {
				b.WriteString(fmt.Sprintf("On %q: ", firstLine(q, 120)))
			}
			b.WriteString(strings.TrimSpace(cm.Body))
			b.WriteString("\n")
			walk(cm.Replies)
		}
	}
	walk(comments)
	return strings.TrimSpace(b.String())
}

// docPlainText returns a document's current version as rough plain text.
func (h handlers) docPlainText(ctx context.Context, docID string) string {
	d, err := h.deps.repository.GetDocument(ctx, docID)
	if err != nil {
		return ""
	}
	b, err := h.deps.repository.GetVersionHTML(ctx, d.CurrentVersionID)
	if err != nil {
		return ""
	}
	return htmlToText(string(b))
}

func htmlToText(s string) string {
	var out strings.Builder
	skip := ""
	for len(s) > 0 {
		i := strings.IndexByte(s, '<')
		if i < 0 {
			if skip == "" {
				out.WriteString(s)
			}
			break
		}
		if skip == "" {
			out.WriteString(s[:i])
		}
		j := strings.IndexByte(s[i:], '>')
		if j < 0 {
			break
		}
		tag := strings.ToLower(s[i+1 : i+j])
		switch {
		case strings.HasPrefix(tag, "script"), strings.HasPrefix(tag, "style"):
			skip = strings.Fields(tag)[0]
		case skip != "" && strings.HasPrefix(tag, "/"+skip):
			skip = ""
		case strings.HasPrefix(tag, "/p"), strings.HasPrefix(tag, "br"), strings.HasPrefix(tag, "/h"), strings.HasPrefix(tag, "/li"), strings.HasPrefix(tag, "/tr"), strings.HasPrefix(tag, "/div"):
			out.WriteString("\n")
		}
		s = s[i+j+1:]
	}
	text := html.UnescapeString(out.String())
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	for _, l := range lines {
		if l = strings.Join(strings.Fields(l), " "); l != "" {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "\n")
}

// publishDesign stores a generated design as a document (or a new version of
// one), records its metadata, closes addressed comments and settles the job.
// It is shared by the Stitch and canvas engines.
func (h handlers) publishDesign(ctx context.Context, job *forgeJob, p auth.Principal, in forgeDesignRequest, doc []byte, title, reviseDocID string, meta forgeDesignMeta, note string) error {
	forge.update(job, func(j *forgeJob) { j.Status, j.Stage = "running", "Publishing to Forge" })
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Untitled design"
	}
	var docID, versionID string
	if reviseDocID == "" {
		d, v, err := h.deps.repository.CreateDocument(ctx, p, "Design · "+title, doc)
		if err != nil {
			return err
		}
		docID, versionID = d.ID, v.ID
		if h.deps.defaultDocPermission != repo.RoleNone {
			_, _ = h.deps.repository.CreateGrant(ctx, "document", d.ID, auth.Principal{Type: auth.PrincipalAnyone}, h.deps.defaultDocPermission, p)
		}
		_, _ = h.deps.repository.ReplaceDocumentLabels(ctx, d.ID, []string{"design"}, p)
		if in.SolutionID != "" && h.deps.forge != nil {
			stage := in.Stage
			if stage == "" {
				stage = "prd"
			}
			if sol, err := h.deps.forge.GetSolution(ctx, in.SolutionID); err == nil {
				h.placeInSolution(ctx, sol, d.ID, p)
				_, _ = h.deps.repository.ReplaceDocumentLabels(ctx, d.ID, []string{"design", strings.ToLower(sol.Key)}, p)
			}
			if _, err := h.deps.forge.AddArtifact(ctx, forgepkg.AddArtifactInput{
				SolutionID: in.SolutionID, Stage: stage, Kind: "design", DocumentID: d.ID, Title: title, CreatedBy: p.ID,
			}); err != nil {
				slog.Warn("forge: could not attach design to solution", slog.Any("error", err))
			}
		}
	} else {
		d, err := h.deps.repository.GetDocument(ctx, reviseDocID)
		if err != nil {
			return err
		}
		summary := "Forge: " + firstLine(in.Prompt, 140)
		v, err := h.deps.repository.CreateVersion(ctx, reviseDocID, d.CurrentVersionID, summary, doc, p)
		if err != nil {
			return err
		}
		docID, versionID = reviseDocID, v.ID
		_ = h.deps.repository.AppendDocumentEvent(ctx, docID, repo.DocumentEventDocument, nil, p)
	}
	raw, _ := json.Marshal(meta)
	if _, err := h.deps.repository.PatchDocumentKV(ctx, docID, map[string]json.RawMessage{forgeKVDesign: raw}, nil, p); err != nil {
		slog.Warn("forge: could not store design metadata", slog.Any("error", err))
	}
	if len(in.resolve) > 0 {
		h.closeAddressedComments(ctx, docID, versionID, in.resolve, p, meta)
	}
	forge.update(job, func(j *forgeJob) {
		j.Status, j.Stage, j.DocumentID, j.VersionID, j.FinishedAt = "done", "Published"+note, docID, versionID, now()
	})
	return nil
}
