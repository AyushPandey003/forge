package server

// Forge solutions: HTTP surface for the delivery workflow.

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/razorpay/slashdocs/api/internal/auth"
	forgepkg "github.com/razorpay/slashdocs/api/internal/forge"
	"github.com/razorpay/slashdocs/api/internal/repo"
)

func (h handlers) registerForgeRoutes(r *gin.RouterGroup) {
	r.GET("/forge/meta", h.forgeMeta)
	r.GET("/forge/workflows", h.forgeListWorkflows)
	r.POST("/forge/workflows", h.forgeSaveWorkflow)
	r.PUT("/forge/workflows/:id", h.forgeSaveWorkflow)
	r.GET("/forge/solutions", h.forgeListSolutions)
	r.POST("/forge/solutions", h.forgeCreateSolution)
	r.GET("/forge/solutions/:sid", h.forgeBoard)
	r.PATCH("/forge/solutions/:sid", h.forgePatchSolution)
	r.GET("/forge/solutions/:sid/events", h.forgeEvents)
	r.POST("/forge/solutions/:sid/members", h.forgeAddMember)
	r.DELETE("/forge/solutions/:sid/members/:uid/:role", h.forgeRemoveMember)
	r.POST("/forge/solutions/:sid/artifacts", h.forgeAddArtifact)
	r.DELETE("/forge/solutions/:sid/artifacts/:aid", h.forgeRemoveArtifact)
	r.POST("/forge/solutions/:sid/gates/:stage/:gate", h.forgeDecide)
	r.POST("/forge/solutions/:sid/advance", h.forgeAdvance)
	r.POST("/forge/solutions/:sid/stage", h.forgeSetStage)
	r.GET("/forge/inbox", h.forgeInbox)
	r.GET("/forge/documents/:id/source", h.forgeDocSource)
	r.POST("/forge/documents/:id/source", h.forgeSaveDocSource)
	r.GET("/forge/documents/:id/context", h.forgeDocContext)
}

func (h handlers) forgeEnabled(c *gin.Context) bool {
	if h.deps.forge == nil {
		featureDisabled(c, "Forge is not enabled on this server")
		return false
	}
	return true
}

func forgeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, forgepkg.ErrNotFound):
		notFound(c)
	case errors.Is(err, forgepkg.ErrForbidden):
		forbidden(c, "not allowed")
	case errors.Is(err, forgepkg.ErrInvalid):
		badRequest(c, strings.TrimPrefix(err.Error(), "invalid: "))
	default:
		internalErr(c, err)
	}
}

// solutionAccess loads a solution and the viewer's roles. Visible to its
// creator and members.
func (h handlers) solutionAccess(c *gin.Context) (forgepkg.Solution, map[string]bool, bool) {
	if !h.forgeEnabled(c) {
		return forgepkg.Solution{}, nil, false
	}
	ctx := c.Request.Context()
	sol, err := h.deps.forge.GetSolution(ctx, c.Param("sid"))
	if err != nil {
		forgeErr(c, err)
		return sol, nil, false
	}
	p := current(c)
	roles, err := h.deps.forge.RolesOf(ctx, sol.ID, p.ID)
	if err != nil {
		internalErr(c, err)
		return sol, nil, false
	}
	if len(roles) == 0 && sol.CreatedBy.ID != p.ID {
		notFound(c)
		return sol, nil, false
	}
	return sol, roles, true
}

func (h handlers) requireManager(c *gin.Context, sol forgepkg.Solution, roles map[string]bool) bool {
	if !forgepkg.IsManager(roles, sol.CreatedBy.ID == current(c).ID) {
		forbidden(c, "only solution owners can do this")
		return false
	}
	return true
}

// ---- Meta / workflows -------------------------------------------------------

func (h handlers) forgeMeta(c *gin.Context) {
	if !h.forgeEnabled(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"roles": forgepkg.AllRoles})
}

func (h handlers) forgeListWorkflows(c *gin.Context) {
	if !h.forgeEnabled(c) {
		return
	}
	ctx := c.Request.Context()
	if _, err := h.deps.forge.EnsureDefaultWorkflow(ctx); err != nil {
		internalErr(c, err)
		return
	}
	list, err := h.deps.forge.ListWorkflows(ctx)
	if err != nil {
		internalErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func (h handlers) forgeSaveWorkflow(c *gin.Context) {
	if !h.forgeEnabled(c) {
		return
	}
	var in forgepkg.WorkflowRecord
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid workflow")
		return
	}
	in.ID = c.Param("id")
	w, err := h.deps.forge.SaveWorkflow(c.Request.Context(), in, current(c).ID)
	if err != nil {
		forgeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, w)
}

// ---- Solutions --------------------------------------------------------------

type solutionListItem struct {
	forgepkg.Solution
	StageLabel string   `json:"stage_label"`
	StageIndex int      `json:"stage_index"`
	StageCount int      `json:"stage_count"`
	Roles      []string `json:"roles"`
}

func (h handlers) forgeListSolutions(c *gin.Context) {
	if !h.forgeEnabled(c) {
		return
	}
	ctx := c.Request.Context()
	p := current(c)
	list, err := h.deps.forge.ListSolutions(ctx, p.ID, c.Query("status"))
	if err != nil {
		internalErr(c, err)
		return
	}
	out := make([]solutionListItem, 0, len(list))
	for _, s := range list {
		st, idx, _ := s.Workflow.Stage(s.CurrentStage)
		roles, _ := h.deps.forge.RolesOf(ctx, s.ID, p.ID)
		item := solutionListItem{Solution: s, StageLabel: st.Label, StageIndex: idx, StageCount: len(s.Workflow.Stages)}
		for r := range roles {
			item.Roles = append(item.Roles, r)
		}
		item.Workflow = forgepkg.Workflow{} // keep list payloads small
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h handlers) forgeCreateSolution(c *gin.Context) {
	if !h.forgeEnabled(c) {
		return
	}
	var in struct {
		Title      string `json:"title"`
		Summary    string `json:"summary"`
		WorkflowID string `json:"workflow_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid solution")
		return
	}
	ctx := c.Request.Context()
	p := current(c)
	// Every solution gets a folder so its documents stay together.
	var folderID string
	if f, err := h.deps.repository.CreateFolder(ctx, *p, strings.TrimSpace(in.Title), ""); err == nil {
		folderID = f.ID
	}
	sol, err := h.deps.forge.CreateSolution(ctx, forgepkg.CreateSolutionInput{
		Title: in.Title, Summary: in.Summary, WorkflowID: in.WorkflowID, FolderID: folderID, CreatedBy: p.ID,
	})
	if err != nil {
		forgeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, sol)
}

func (h handlers) forgeBoard(c *gin.Context) {
	sol, _, ok := h.solutionAccess(c)
	if !ok {
		return
	}
	b, err := h.deps.forge.BuildBoard(c.Request.Context(), sol.ID, current(c).ID)
	if err != nil {
		forgeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h handlers) forgePatchSolution(c *gin.Context) {
	sol, roles, ok := h.solutionAccess(c)
	if !ok || !h.requireManager(c, sol, roles) {
		return
	}
	var p forgepkg.SolutionPatch
	if err := c.ShouldBindJSON(&p); err != nil {
		badRequest(c, "invalid patch")
		return
	}
	out, err := h.deps.forge.UpdateSolution(c.Request.Context(), sol.ID, p, current(c).ID)
	if err != nil {
		forgeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h handlers) forgeEvents(c *gin.Context) {
	sol, _, ok := h.solutionAccess(c)
	if !ok {
		return
	}
	ev, err := h.deps.forge.Events(c.Request.Context(), sol.ID, 300)
	if err != nil {
		internalErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": ev})
}

// ---- Members ----------------------------------------------------------------

func (h handlers) forgeAddMember(c *gin.Context) {
	sol, roles, ok := h.solutionAccess(c)
	if !ok || !h.requireManager(c, sol, roles) {
		return
	}
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Email == "" {
		badRequest(c, "email and role are required")
		return
	}
	ctx := c.Request.Context()
	u, err := h.deps.forge.UserByEmail(ctx, in.Email)
	if errors.Is(err, forgepkg.ErrNotFound) {
		// Invite: create a placeholder user who completes sign-in later.
		pr, perr := h.deps.repository.EnsureUserByEmail(ctx, strings.TrimSpace(in.Email))
		if perr != nil {
			internalErr(c, perr)
			return
		}
		u = forgepkg.User{ID: pr.ID, Email: pr.Email}
	} else if err != nil {
		internalErr(c, err)
		return
	}
	if err := h.deps.forge.AddMember(ctx, sol.ID, u.ID, in.Role, current(c).ID); err != nil {
		forgeErr(c, err)
		return
	}
	// Members can comment on every artifact; owners edit.
	h.grantSolutionDocs(ctx, sol, u.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h handlers) forgeRemoveMember(c *gin.Context) {
	sol, roles, ok := h.solutionAccess(c)
	if !ok || !h.requireManager(c, sol, roles) {
		return
	}
	if err := h.deps.forge.RemoveMember(c.Request.Context(), sol.ID, c.Param("uid"), c.Param("role"), current(c).ID); err != nil {
		forgeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// grantSolutionDocs shares the solution folder (and so every artifact in it)
// with a member. Managers get editor; everyone else commenter.
func (h handlers) grantSolutionDocs(ctx context.Context, sol forgepkg.Solution, userID string) {
	if sol.FolderID == "" {
		return
	}
	roles, _ := h.deps.forge.RolesOf(ctx, sol.ID, userID)
	role := repo.RoleCommenter
	if forgepkg.IsManager(roles, false) || roles[forgepkg.RoleImplementer] {
		role = repo.RoleEditor
	}
	_, _ = h.deps.repository.CreateGrant(ctx, "folder", sol.FolderID,
		auth.Principal{Type: auth.PrincipalUser, ID: userID}, role, auth.Principal{Type: auth.PrincipalUser, ID: sol.CreatedBy.ID})
}

// ---- Artifacts --------------------------------------------------------------

type addArtifactRequest struct {
	Stage string `json:"stage"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	// One of: DocumentID (attach existing), URL (external), or Create=true
	// (new document from the slot's template).
	DocumentID string         `json:"document_id"`
	URL        string         `json:"url"`
	Create     bool           `json:"create"`
	Markdown   string         `json:"markdown"`
	Meta       map[string]any `json:"meta"`
}

func (h handlers) forgeAddArtifact(c *gin.Context) {
	sol, roles, ok := h.solutionAccess(c)
	if !ok {
		return
	}
	if len(roles) == 0 && sol.CreatedBy.ID != current(c).ID {
		forbidden(c, "members only")
		return
	}
	var in addArtifactRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid artifact")
		return
	}
	ctx := c.Request.Context()
	p := current(c)
	st, _, found := sol.Workflow.Stage(in.Stage)
	if !found {
		badRequest(c, "unknown stage")
		return
	}
	var kind forgepkg.ArtifactKind
	for _, k := range st.Artifacts {
		if k.Kind == in.Kind {
			kind = k
		}
	}
	if kind.Kind == "" {
		badRequest(c, "unknown artifact kind for this stage")
		return
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = kind.Label
	}
	switch {
	case in.Create && !kind.External:
		src := in.Markdown
		if strings.TrimSpace(src) == "" {
			src = forgepkg.Template(kind.Template, sol.Title)
		}
		docTitle := sol.Key + " · " + title
		html, err := forgepkg.BuildMarkdownDoc(docTitle, src)
		if err != nil {
			internalErr(c, err)
			return
		}
		d, _, err := h.deps.repository.CreateDocument(ctx, *p, docTitle, html)
		if err != nil {
			internalErr(c, err)
			return
		}
		h.placeInSolution(ctx, sol, d.ID, *p)
		in.DocumentID = d.ID
	case in.DocumentID != "":
		if !h.needDocRole(c, in.DocumentID, repo.RoleViewer) {
			return
		}
		h.placeInSolution(ctx, sol, in.DocumentID, *p)
	}
	art, err := h.deps.forge.AddArtifact(ctx, forgepkg.AddArtifactInput{
		SolutionID: sol.ID, Stage: in.Stage, Kind: in.Kind, DocumentID: in.DocumentID,
		URL: in.URL, Title: title, Meta: in.Meta, CreatedBy: p.ID,
	})
	if err != nil {
		forgeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, art)
}

// placeInSolution moves a document into the solution folder (best effort).
func (h handlers) placeInSolution(ctx context.Context, sol forgepkg.Solution, docID string, p auth.Principal) {
	if sol.FolderID == "" {
		return
	}
	_, _ = h.deps.repository.MoveDocumentToFolder(ctx, p, sol.FolderID, docID)
	_, _ = h.deps.repository.ReplaceDocumentLabels(ctx, docID, []string{strings.ToLower(sol.Key)}, p)
}

func (h handlers) forgeRemoveArtifact(c *gin.Context) {
	sol, roles, ok := h.solutionAccess(c)
	if !ok || !h.requireManager(c, sol, roles) {
		return
	}
	a, err := h.deps.forge.GetArtifact(c.Request.Context(), c.Param("aid"))
	if err != nil || a.SolutionID != sol.ID {
		notFound(c)
		return
	}
	if err := h.deps.forge.RemoveArtifact(c.Request.Context(), a.ID, current(c).ID); err != nil {
		forgeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- Gates ------------------------------------------------------------------

func (h handlers) forgeDecide(c *gin.Context) {
	sol, roles, ok := h.solutionAccess(c)
	if !ok {
		return
	}
	var in struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
		Override bool   `json:"override"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid decision")
		return
	}
	stageID, gateID := c.Param("stage"), c.Param("gate")
	st, _, found := sol.Workflow.Stage(stageID)
	if !found {
		notFound(c)
		return
	}
	var gate *forgepkg.Gate
	for i := range st.Gates {
		if st.Gates[i].ID == gateID {
			gate = &st.Gates[i]
		}
	}
	if gate == nil {
		notFound(c)
		return
	}
	isApprover := false
	for _, r := range gate.ApproverRoles {
		if roles[r] {
			isApprover = true
		}
	}
	if in.Override {
		if !gate.AllowOverride {
			forbidden(c, "this gate cannot be overridden")
			return
		}
		if !h.requireManager(c, sol, roles) {
			return
		}
		if strings.TrimSpace(in.Comment) == "" {
			badRequest(c, "an override needs a reason")
			return
		}
		in.Decision = "approved"
	} else if !isApprover {
		forbidden(c, "you are not an approver for this gate")
		return
	}
	if (in.Decision == "changes_requested" || in.Decision == "rejected") && strings.TrimSpace(in.Comment) == "" {
		badRequest(c, "explain what needs to change")
		return
	}
	ctx := c.Request.Context()
	pinned, err := h.deps.forge.PinnedVersions(ctx, sol.ID, stageID)
	if err != nil {
		internalErr(c, err)
		return
	}
	if _, err := h.deps.forge.RecordDecision(ctx, sol.ID, stageID, gateID, current(c).ID, in.Decision, strings.TrimSpace(in.Comment), pinned, in.Override); err != nil {
		forgeErr(c, err)
		return
	}
	b, err := h.deps.forge.BuildBoard(ctx, sol.ID, current(c).ID)
	if err != nil {
		forgeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

// forgeAdvance moves to the next stage when the current one is ready.
func (h handlers) forgeAdvance(c *gin.Context) {
	sol, roles, ok := h.solutionAccess(c)
	if !ok || !h.requireManager(c, sol, roles) {
		return
	}
	ctx := c.Request.Context()
	p := current(c)
	b, err := h.deps.forge.BuildBoard(ctx, sol.ID, p.ID)
	if err != nil {
		forgeErr(c, err)
		return
	}
	_, idx, _ := sol.Workflow.Stage(sol.CurrentStage)
	cur := b.Stages[idx]
	if !cur.Ready {
		c.JSON(http.StatusConflict, errorResponse("stage_not_ready", "this stage still has missing artifacts or open gates", gin.H{"missing": cur.Missing}))
		return
	}
	if idx == len(sol.Workflow.Stages)-1 {
		shipped := "shipped"
		if _, err := h.deps.forge.UpdateSolution(ctx, sol.ID, forgepkg.SolutionPatch{Status: &shipped}, p.ID); err != nil {
			forgeErr(c, err)
			return
		}
	} else if err := h.deps.forge.SetStage(ctx, sol.ID, sol.Workflow.Stages[idx+1].ID, p.ID, map[string]any{"from": cur.ID}); err != nil {
		internalErr(c, err)
		return
	}
	b, err = h.deps.forge.BuildBoard(ctx, sol.ID, p.ID)
	if err != nil {
		forgeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

// forgeSetStage lets a manager move the solution to any stage (e.g. send it
// back to PRD). Always audited.
func (h handlers) forgeSetStage(c *gin.Context) {
	sol, roles, ok := h.solutionAccess(c)
	if !ok || !h.requireManager(c, sol, roles) {
		return
	}
	var in struct {
		Stage  string `json:"stage"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid request")
		return
	}
	if _, _, found := sol.Workflow.Stage(in.Stage); !found {
		badRequest(c, "unknown stage")
		return
	}
	ctx := c.Request.Context()
	if err := h.deps.forge.SetStage(ctx, sol.ID, in.Stage, current(c).ID, map[string]any{"from": sol.CurrentStage, "reason": in.Reason, "manual": true}); err != nil {
		internalErr(c, err)
		return
	}
	if sol.Status == "shipped" {
		active := "active"
		_, _ = h.deps.forge.UpdateSolution(ctx, sol.ID, forgepkg.SolutionPatch{Status: &active}, current(c).ID)
	}
	b, err := h.deps.forge.BuildBoard(ctx, sol.ID, current(c).ID)
	if err != nil {
		forgeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

// ---- Inbox ------------------------------------------------------------------

type inboxItem struct {
	SolutionID  string `json:"solution_id"`
	SolutionKey string `json:"solution_key"`
	Title       string `json:"title"`
	Stage       string `json:"stage"`
	StageLabel  string `json:"stage_label"`
	Gate        string `json:"gate"`
	GateLabel   string `json:"gate_label"`
	Question    string `json:"question"`
	Reason      string `json:"reason"` // awaiting | stale
}

// forgeInbox lists gates on current stages waiting on the viewer.
func (h handlers) forgeInbox(c *gin.Context) {
	if !h.forgeEnabled(c) {
		return
	}
	ctx := c.Request.Context()
	p := current(c)
	list, err := h.deps.forge.ListSolutions(ctx, p.ID, "active")
	if err != nil {
		internalErr(c, err)
		return
	}
	items := []inboxItem{}
	for _, s := range list {
		b, err := h.deps.forge.BuildBoard(ctx, s.ID, p.ID)
		if err != nil {
			continue
		}
		for _, st := range b.Stages {
			if st.State != "current" {
				continue
			}
			for _, g := range st.GateViews {
				if !g.CanDecide || g.State == "overridden" {
					continue
				}
				// Awaiting: you haven't decided and the gate still needs approvals.
				// Stale: you decided, but an artifact changed since.
				reason := ""
				if g.State != "approved" {
					reason = "awaiting"
				}
				for _, d := range g.Decisions {
					if d.User.ID == p.ID {
						reason = ""
						if d.Stale {
							reason = "stale"
						}
					}
				}
				if reason == "" {
					continue
				}
				items = append(items, inboxItem{
					SolutionID: s.ID, SolutionKey: s.Key, Title: s.Title, Stage: st.ID, StageLabel: st.Label,
					Gate: g.ID, GateLabel: g.Label, Question: g.Question, Reason: reason,
				})
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// ---- Markdown authoring -----------------------------------------------------

func (h handlers) forgeDocSource(c *gin.Context) {
	id := c.Param("id")
	if !h.needDocRole(c, id, repo.RoleViewer) {
		return
	}
	ctx := c.Request.Context()
	d, err := h.deps.repository.GetDocument(ctx, id)
	if err != nil {
		notFound(c)
		return
	}
	html, err := h.deps.repository.GetVersionHTML(ctx, d.CurrentVersionID)
	if err != nil {
		internalErr(c, err)
		return
	}
	src, ok := forgepkg.ExtractMarkdownSource(html)
	c.JSON(http.StatusOK, gin.H{"editable": ok, "markdown": src, "version_id": d.CurrentVersionID, "title": d.Title})
}

func (h handlers) forgeSaveDocSource(c *gin.Context) {
	id := c.Param("id")
	if !h.needDocRole(c, id, repo.RoleEditor) {
		return
	}
	var in struct {
		Markdown      string `json:"markdown"`
		BaseVersionID string `json:"base_version_id"`
		ChangeSummary string `json:"change_summary"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid request")
		return
	}
	ctx := c.Request.Context()
	d, err := h.deps.repository.GetDocument(ctx, id)
	if err != nil {
		notFound(c)
		return
	}
	doc, err := forgepkg.BuildMarkdownDoc(d.Title, in.Markdown)
	if err != nil {
		internalErr(c, err)
		return
	}
	summary := strings.TrimSpace(in.ChangeSummary)
	if summary == "" {
		summary = "Edited in Forge"
	}
	v, err := h.deps.repository.CreateVersion(ctx, id, in.BaseVersionID, summary, doc, *current(c))
	if err != nil {
		if errors.Is(err, repo.ErrVersionConflict) {
			c.JSON(http.StatusConflict, errorResponse("version_conflict", "someone published a newer version; reload to merge", nil))
			return
		}
		internalErr(c, err)
		return
	}
	_ = h.deps.repository.AppendDocumentEvent(ctx, id, repo.DocumentEventDocument, nil, *current(c))
	if h.deps.forge != nil {
		if a, err := h.deps.forge.ArtifactForDocument(ctx, id); err == nil {
			_ = h.deps.forge.LogEvent(ctx, a.SolutionID, current(c).ID, "artifact.versioned",
				map[string]any{"artifact_id": a.ID, "title": a.Title, "version": v.Number})
		}
	}
	c.JSON(http.StatusCreated, gin.H{"id": v.ID, "number": v.Number})
}

// forgeDocContext tells the document page which solution/stage a doc belongs to.
func (h handlers) forgeDocContext(c *gin.Context) {
	id := c.Param("id")
	if !h.needDocRole(c, id, repo.RoleViewer) {
		return
	}
	if h.deps.forge == nil {
		c.JSON(http.StatusOK, gin.H{"artifact": nil})
		return
	}
	ctx := c.Request.Context()
	a, err := h.deps.forge.ArtifactForDocument(ctx, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"artifact": nil})
		return
	}
	sol, err := h.deps.forge.GetSolution(ctx, a.SolutionID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"artifact": nil})
		return
	}
	st, _, _ := sol.Workflow.Stage(a.Stage)
	c.JSON(http.StatusOK, gin.H{"artifact": a, "solution": gin.H{"id": sol.ID, "key": sol.Key, "title": sol.Title}, "stage_label": st.Label})
}
