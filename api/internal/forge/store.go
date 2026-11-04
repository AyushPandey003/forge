package forge

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrInvalid   = errors.New("invalid")
)

// Store is Forge's persistence. It talks to the same Postgres as slashdocs.
type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

func newID(prefix string) string {
	var b [10]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]))
}

// ---- Types -----------------------------------------------------------------

type User struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	PictureURL string `json:"picture_url,omitempty"`
}

type WorkflowRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Definition  Workflow  `json:"definition"`
	IsDefault   bool      `json:"is_default"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Solution struct {
	ID           string    `json:"id"`
	Key          string    `json:"key"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	Status       string    `json:"status"`
	WorkflowID   string    `json:"workflow_id"`
	Workflow     Workflow  `json:"workflow"`
	CurrentStage string    `json:"current_stage"`
	FolderID     string    `json:"folder_id,omitempty"`
	CreatedBy    User      `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Member struct {
	User User   `json:"user"`
	Role string `json:"role"`
}

type Artifact struct {
	ID         string          `json:"id"`
	SolutionID string          `json:"solution_id"`
	Stage      string          `json:"stage"`
	Kind       string          `json:"kind"`
	DocumentID string          `json:"document_id,omitempty"`
	URL        string          `json:"url,omitempty"`
	Title      string          `json:"title"`
	Meta       json.RawMessage `json:"meta"`
	CreatedBy  User            `json:"created_by"`
	CreatedAt  time.Time       `json:"created_at"`
	// Joined from documents/versions when DocumentID is set.
	CurrentVersionID string `json:"current_version_id,omitempty"`
	VersionNumber    int    `json:"version_number,omitempty"`
	DocUpdatedAt     string `json:"doc_updated_at,omitempty"`
}

type Approval struct {
	ID        string            `json:"id"`
	Stage     string            `json:"stage"`
	Gate      string            `json:"gate"`
	User      User              `json:"user"`
	Decision  string            `json:"decision"`
	Comment   string            `json:"comment"`
	Pinned    map[string]string `json:"pinned"`
	Override  bool              `json:"override"`
	CreatedAt time.Time         `json:"created_at"`
}

type Event struct {
	ID        int64           `json:"id"`
	Actor     User            `json:"actor"`
	Kind      string          `json:"kind"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

// ---- Workflows -------------------------------------------------------------

// EnsureDefaultWorkflow seeds the built-in template once.
func (s *Store) EnsureDefaultWorkflow(ctx context.Context) (WorkflowRecord, error) {
	w, err := s.DefaultWorkflow(ctx)
	if err == nil {
		return w, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return WorkflowRecord{}, err
	}
	def, _ := json.Marshal(DefaultWorkflow())
	id := newID("wf")
	_, err = s.db.Exec(ctx, `INSERT INTO forge_workflows (id, name, description, definition, is_default)
		VALUES ($1, 'Product delivery', 'Problem → PRD → Implementation → Release, with approval gates.', $2, true)
		ON CONFLICT DO NOTHING`, id, def)
	if err != nil {
		return WorkflowRecord{}, err
	}
	return s.DefaultWorkflow(ctx)
}

const workflowCols = `id, name, description, definition, is_default, updated_at`

func scanWorkflow(row pgx.Row) (WorkflowRecord, error) {
	var w WorkflowRecord
	var def []byte
	if err := row.Scan(&w.ID, &w.Name, &w.Description, &def, &w.IsDefault, &w.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return w, ErrNotFound
		}
		return w, err
	}
	return w, json.Unmarshal(def, &w.Definition)
}

func (s *Store) DefaultWorkflow(ctx context.Context) (WorkflowRecord, error) {
	return scanWorkflow(s.db.QueryRow(ctx, `SELECT `+workflowCols+` FROM forge_workflows WHERE is_default`))
}

func (s *Store) GetWorkflow(ctx context.Context, id string) (WorkflowRecord, error) {
	return scanWorkflow(s.db.QueryRow(ctx, `SELECT `+workflowCols+` FROM forge_workflows WHERE id=$1`, id))
}

func (s *Store) ListWorkflows(ctx context.Context) ([]WorkflowRecord, error) {
	rows, err := s.db.Query(ctx, `SELECT `+workflowCols+` FROM forge_workflows ORDER BY is_default DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkflowRecord
	for rows.Next() {
		w, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) SaveWorkflow(ctx context.Context, w WorkflowRecord, actor string) (WorkflowRecord, error) {
	if err := w.Definition.Validate(); err != nil {
		return w, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if strings.TrimSpace(w.Name) == "" {
		return w, fmt.Errorf("%w: name is required", ErrInvalid)
	}
	def, _ := json.Marshal(w.Definition)
	if w.ID == "" {
		w.ID = newID("wf")
		_, err := s.db.Exec(ctx, `INSERT INTO forge_workflows (id, name, description, definition, created_by) VALUES ($1,$2,$3,$4,$5)`,
			w.ID, w.Name, w.Description, def, actor)
		if err != nil {
			return w, err
		}
	} else {
		tag, err := s.db.Exec(ctx, `UPDATE forge_workflows SET name=$2, description=$3, definition=$4, updated_at=now() WHERE id=$1`,
			w.ID, w.Name, w.Description, def)
		if err != nil {
			return w, err
		}
		if tag.RowsAffected() == 0 {
			return w, ErrNotFound
		}
	}
	return s.GetWorkflow(ctx, w.ID)
}

// ---- Users -----------------------------------------------------------------

func (s *Store) userMap(ctx context.Context, ids []string) map[string]User {
	out := map[string]User{}
	if len(ids) == 0 {
		return out
	}
	rows, err := s.db.Query(ctx, `SELECT id, email, name, picture_url FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var u User
		if rows.Scan(&u.ID, &u.Email, &u.Name, &u.PictureURL) == nil {
			out[u.ID] = u
		}
	}
	return out
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `SELECT id, email, name, picture_url FROM users WHERE lower(email)=lower($1)`, strings.TrimSpace(email)).
		Scan(&u.ID, &u.Email, &u.Name, &u.PictureURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// ---- Solutions -------------------------------------------------------------

const solutionCols = `id, key, title, summary, status, workflow_id, workflow, current_stage, COALESCE(folder_id,''), created_by, created_at, updated_at`

func (s *Store) scanSolutions(ctx context.Context, rows pgx.Rows) ([]Solution, error) {
	defer rows.Close()
	var out []Solution
	var ids []string
	for rows.Next() {
		var sol Solution
		var wf []byte
		if err := rows.Scan(&sol.ID, &sol.Key, &sol.Title, &sol.Summary, &sol.Status, &sol.WorkflowID, &wf,
			&sol.CurrentStage, &sol.FolderID, &sol.CreatedBy.ID, &sol.CreatedAt, &sol.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(wf, &sol.Workflow)
		out = append(out, sol)
		ids = append(ids, sol.CreatedBy.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	users := s.userMap(ctx, ids)
	for i := range out {
		if u, ok := users[out[i].CreatedBy.ID]; ok {
			out[i].CreatedBy = u
		}
	}
	return out, nil
}

func (s *Store) GetSolution(ctx context.Context, id string) (Solution, error) {
	rows, err := s.db.Query(ctx, `SELECT `+solutionCols+` FROM forge_solutions WHERE id=$1 OR key=upper($1)`, id)
	if err != nil {
		return Solution{}, err
	}
	list, err := s.scanSolutions(ctx, rows)
	if err != nil {
		return Solution{}, err
	}
	if len(list) == 0 {
		return Solution{}, ErrNotFound
	}
	return list[0], nil
}

// ListSolutions returns solutions the user created or is a member of.
func (s *Store) ListSolutions(ctx context.Context, userID, status string) ([]Solution, error) {
	q := `SELECT ` + solutionCols + ` FROM forge_solutions s
		WHERE (s.created_by=$1 OR EXISTS (SELECT 1 FROM forge_members m WHERE m.solution_id=s.id AND m.user_id=$1))`
	args := []any{userID}
	if status != "" {
		q += ` AND s.status=$2`
		args = append(args, status)
	}
	q += ` ORDER BY s.updated_at DESC`
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return s.scanSolutions(ctx, rows)
}

type CreateSolutionInput struct {
	Title      string
	Summary    string
	WorkflowID string
	FolderID   string
	CreatedBy  string
}

func (s *Store) CreateSolution(ctx context.Context, in CreateSolutionInput) (Solution, error) {
	if strings.TrimSpace(in.Title) == "" {
		return Solution{}, fmt.Errorf("%w: title is required", ErrInvalid)
	}
	var wf WorkflowRecord
	var err error
	if in.WorkflowID != "" {
		wf, err = s.GetWorkflow(ctx, in.WorkflowID)
	} else {
		wf, err = s.EnsureDefaultWorkflow(ctx)
	}
	if err != nil {
		return Solution{}, err
	}
	def, _ := json.Marshal(wf.Definition)
	id := newID("sol")
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Solution{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var seq int64
	if err := tx.QueryRow(ctx, `SELECT nextval('forge_solution_key_seq')`).Scan(&seq); err != nil {
		return Solution{}, err
	}
	var folder any
	if in.FolderID != "" {
		folder = in.FolderID
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge_solutions (id, key, title, summary, workflow_id, workflow, current_stage, folder_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, fmt.Sprintf("SOL-%d", seq), strings.TrimSpace(in.Title), strings.TrimSpace(in.Summary), wf.ID, def, wf.Definition.Stages[0].ID, folder, in.CreatedBy)
	if err != nil {
		return Solution{}, err
	}
	// The creator owns the product side by default.
	if _, err := tx.Exec(ctx, `INSERT INTO forge_members (solution_id, user_id, role, added_by) VALUES ($1,$2,$3,$2)`, id, in.CreatedBy, RoleProductOwner); err != nil {
		return Solution{}, err
	}
	if err := logEvent(ctx, tx, id, in.CreatedBy, "solution.created", map[string]any{"title": in.Title, "workflow": wf.Name}); err != nil {
		return Solution{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Solution{}, err
	}
	return s.GetSolution(ctx, id)
}

type SolutionPatch struct {
	Title   *string `json:"title"`
	Summary *string `json:"summary"`
	Status  *string `json:"status"`
}

func (s *Store) UpdateSolution(ctx context.Context, id string, p SolutionPatch, actor string) (Solution, error) {
	sol, err := s.GetSolution(ctx, id)
	if err != nil {
		return sol, err
	}
	changes := map[string]any{}
	if p.Title != nil && strings.TrimSpace(*p.Title) != "" && *p.Title != sol.Title {
		sol.Title, changes["title"] = strings.TrimSpace(*p.Title), strings.TrimSpace(*p.Title)
	}
	if p.Summary != nil && *p.Summary != sol.Summary {
		sol.Summary, changes["summary"] = *p.Summary, true
	}
	if p.Status != nil && *p.Status != sol.Status {
		switch *p.Status {
		case "active", "shipped", "archived":
			sol.Status, changes["status"] = *p.Status, *p.Status
		default:
			return sol, fmt.Errorf("%w: unknown status", ErrInvalid)
		}
	}
	if len(changes) == 0 {
		return sol, nil
	}
	if _, err := s.db.Exec(ctx, `UPDATE forge_solutions SET title=$2, summary=$3, status=$4, updated_at=now() WHERE id=$1`,
		sol.ID, sol.Title, sol.Summary, sol.Status); err != nil {
		return sol, err
	}
	_ = logEvent(ctx, s.db, sol.ID, actor, "solution.updated", changes)
	return s.GetSolution(ctx, sol.ID)
}

func (s *Store) SetStage(ctx context.Context, id, stage, actor string, data map[string]any) error {
	if _, err := s.db.Exec(ctx, `UPDATE forge_solutions SET current_stage=$2, updated_at=now() WHERE id=$1`, id, stage); err != nil {
		return err
	}
	if data == nil {
		data = map[string]any{}
	}
	data["stage"] = stage
	return logEvent(ctx, s.db, id, actor, "stage.changed", data)
}

func (s *Store) touch(ctx context.Context, id string) {
	_, _ = s.db.Exec(ctx, `UPDATE forge_solutions SET updated_at=now() WHERE id=$1`, id)
}

// ---- Members ---------------------------------------------------------------

func (s *Store) Members(ctx context.Context, solutionID string) ([]Member, error) {
	rows, err := s.db.Query(ctx, `SELECT m.role, u.id, u.email, u.name, u.picture_url
		FROM forge_members m JOIN users u ON u.id=m.user_id WHERE m.solution_id=$1 ORDER BY m.created_at`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.Role, &m.User.ID, &m.User.Email, &m.User.Name, &m.User.PictureURL); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) AddMember(ctx context.Context, solutionID, userID, role, actor string) error {
	if !validRole(role) {
		return fmt.Errorf("%w: unknown role %q", ErrInvalid, role)
	}
	tag, err := s.db.Exec(ctx, `INSERT INTO forge_members (solution_id, user_id, role, added_by) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
		solutionID, userID, role, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		s.touch(ctx, solutionID)
		return logEvent(ctx, s.db, solutionID, actor, "member.added", map[string]any{"user_id": userID, "role": role})
	}
	return nil
}

func (s *Store) RemoveMember(ctx context.Context, solutionID, userID, role, actor string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM forge_members WHERE solution_id=$1 AND user_id=$2 AND role=$3`, solutionID, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	s.touch(ctx, solutionID)
	return logEvent(ctx, s.db, solutionID, actor, "member.removed", map[string]any{"user_id": userID, "role": role})
}

// RolesOf returns the roles a user holds on a solution.
func (s *Store) RolesOf(ctx context.Context, solutionID, userID string) (map[string]bool, error) {
	rows, err := s.db.Query(ctx, `SELECT role FROM forge_members WHERE solution_id=$1 AND user_id=$2`, solutionID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out[r] = true
	}
	return out, rows.Err()
}

// ---- Artifacts -------------------------------------------------------------

func (s *Store) Artifacts(ctx context.Context, solutionID string) ([]Artifact, error) {
	rows, err := s.db.Query(ctx, `SELECT a.id, a.solution_id, a.stage, a.kind, COALESCE(a.document_id,''), a.url, a.title, a.meta, a.created_by, a.created_at,
			COALESCE(d.current_version_id,''), COALESCE(v.number,0), COALESCE(to_char(d.updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''), COALESCE(d.title,'')
		FROM forge_artifacts a
		LEFT JOIN documents d ON d.id=a.document_id
		LEFT JOIN versions v ON v.id=d.current_version_id
		WHERE a.solution_id=$1 ORDER BY a.created_at`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Artifact
	var ids []string
	for rows.Next() {
		var a Artifact
		var docTitle string
		if err := rows.Scan(&a.ID, &a.SolutionID, &a.Stage, &a.Kind, &a.DocumentID, &a.URL, &a.Title, &a.Meta, &a.CreatedBy.ID, &a.CreatedAt,
			&a.CurrentVersionID, &a.VersionNumber, &a.DocUpdatedAt, &docTitle); err != nil {
			return nil, err
		}
		if a.Title == "" {
			a.Title = docTitle
		}
		out = append(out, a)
		ids = append(ids, a.CreatedBy.ID)
	}
	users := s.userMap(ctx, ids)
	for i := range out {
		if u, ok := users[out[i].CreatedBy.ID]; ok {
			out[i].CreatedBy = u
		}
	}
	return out, rows.Err()
}

func (s *Store) GetArtifact(ctx context.Context, id string) (Artifact, error) {
	var a Artifact
	err := s.db.QueryRow(ctx, `SELECT id, solution_id, stage, kind, COALESCE(document_id,''), url, title, meta, created_by, created_at FROM forge_artifacts WHERE id=$1`, id).
		Scan(&a.ID, &a.SolutionID, &a.Stage, &a.Kind, &a.DocumentID, &a.URL, &a.Title, &a.Meta, &a.CreatedBy.ID, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// ArtifactForDocument finds the solution artifact backed by a document.
func (s *Store) ArtifactForDocument(ctx context.Context, documentID string) (Artifact, error) {
	var id string
	err := s.db.QueryRow(ctx, `SELECT id FROM forge_artifacts WHERE document_id=$1 ORDER BY created_at LIMIT 1`, documentID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Artifact{}, ErrNotFound
	}
	if err != nil {
		return Artifact{}, err
	}
	return s.GetArtifact(ctx, id)
}

type AddArtifactInput struct {
	SolutionID string
	Stage      string
	Kind       string
	DocumentID string
	URL        string
	Title      string
	Meta       map[string]any
	CreatedBy  string
}

func (s *Store) AddArtifact(ctx context.Context, in AddArtifactInput) (Artifact, error) {
	sol, err := s.GetSolution(ctx, in.SolutionID)
	if err != nil {
		return Artifact{}, err
	}
	st, _, ok := sol.Workflow.Stage(in.Stage)
	if !ok {
		return Artifact{}, fmt.Errorf("%w: unknown stage %q", ErrInvalid, in.Stage)
	}
	var kind *ArtifactKind
	for i := range st.Artifacts {
		if st.Artifacts[i].Kind == in.Kind {
			kind = &st.Artifacts[i]
		}
	}
	if kind == nil {
		return Artifact{}, fmt.Errorf("%w: stage %q has no artifact kind %q", ErrInvalid, in.Stage, in.Kind)
	}
	if kind.External && strings.TrimSpace(in.URL) == "" {
		return Artifact{}, fmt.Errorf("%w: %s needs a URL", ErrInvalid, kind.Label)
	}
	if !kind.External && in.DocumentID == "" {
		return Artifact{}, fmt.Errorf("%w: %s needs a document", ErrInvalid, kind.Label)
	}
	if !kind.Multiple {
		var n int
		_ = s.db.QueryRow(ctx, `SELECT count(*) FROM forge_artifacts WHERE solution_id=$1 AND stage=$2 AND kind=$3`, in.SolutionID, in.Stage, in.Kind).Scan(&n)
		if n > 0 {
			return Artifact{}, fmt.Errorf("%w: %s already exists for this stage", ErrInvalid, kind.Label)
		}
	}
	if in.Meta == nil {
		in.Meta = map[string]any{}
	}
	meta, _ := json.Marshal(in.Meta)
	id := newID("art")
	var doc any
	if in.DocumentID != "" {
		doc = in.DocumentID
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO forge_artifacts (id, solution_id, stage, kind, document_id, url, title, meta, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, in.SolutionID, in.Stage, in.Kind, doc, strings.TrimSpace(in.URL), strings.TrimSpace(in.Title), meta, in.CreatedBy); err != nil {
		return Artifact{}, err
	}
	s.touch(ctx, in.SolutionID)
	_ = logEvent(ctx, s.db, in.SolutionID, in.CreatedBy, "artifact.added", map[string]any{"artifact_id": id, "stage": in.Stage, "kind": in.Kind, "title": in.Title})
	return s.GetArtifact(ctx, id)
}

func (s *Store) RemoveArtifact(ctx context.Context, id, actor string) error {
	a, err := s.GetArtifact(ctx, id)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx, `DELETE FROM forge_artifacts WHERE id=$1`, id); err != nil {
		return err
	}
	s.touch(ctx, a.SolutionID)
	return logEvent(ctx, s.db, a.SolutionID, actor, "artifact.removed", map[string]any{"artifact_id": id, "kind": a.Kind, "title": a.Title})
}

// ---- Approvals -------------------------------------------------------------

func (s *Store) Approvals(ctx context.Context, solutionID string) ([]Approval, error) {
	rows, err := s.db.Query(ctx, `SELECT a.id, a.stage, a.gate, a.decision, a.comment, a.pinned, a.override, a.created_at,
			u.id, u.email, u.name, u.picture_url
		FROM forge_approvals a JOIN users u ON u.id=a.user_id
		WHERE a.solution_id=$1 ORDER BY a.created_at DESC`, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Approval
	for rows.Next() {
		var a Approval
		var pinned []byte
		if err := rows.Scan(&a.ID, &a.Stage, &a.Gate, &a.Decision, &a.Comment, &pinned, &a.Override, &a.CreatedAt,
			&a.User.ID, &a.User.Email, &a.User.Name, &a.User.PictureURL); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(pinned, &a.Pinned)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) RecordDecision(ctx context.Context, solutionID, stage, gate, userID, decision, comment string, pinned map[string]string, override bool) (Approval, error) {
	switch decision {
	case "approved", "changes_requested", "rejected", "withdrawn":
	default:
		return Approval{}, fmt.Errorf("%w: unknown decision", ErrInvalid)
	}
	p, _ := json.Marshal(pinned)
	id := newID("apr")
	if _, err := s.db.Exec(ctx, `INSERT INTO forge_approvals (id, solution_id, stage, gate, user_id, decision, comment, pinned, override)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, solutionID, stage, gate, userID, decision, comment, p, override); err != nil {
		return Approval{}, err
	}
	s.touch(ctx, solutionID)
	kind := "gate." + decision
	if override {
		kind = "gate.overridden"
	}
	_ = logEvent(ctx, s.db, solutionID, userID, kind, map[string]any{"stage": stage, "gate": gate, "comment": comment, "approval_id": id})
	return Approval{ID: id, Stage: stage, Gate: gate, Decision: decision, Comment: comment, Pinned: pinned, Override: override, CreatedAt: time.Now()}, nil
}

// ---- Events ----------------------------------------------------------------

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func logEvent(ctx context.Context, db execer, solutionID, actor, kind string, data any) error {
	b, _ := json.Marshal(data)
	_, err := db.Exec(ctx, `INSERT INTO forge_events (solution_id, actor_id, kind, data) VALUES ($1,$2,$3,$4)`, solutionID, actor, kind, b)
	return err
}

func (s *Store) LogEvent(ctx context.Context, solutionID, actor, kind string, data any) error {
	return logEvent(ctx, s.db, solutionID, actor, kind, data)
}

func (s *Store) Events(ctx context.Context, solutionID string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.Query(ctx, `SELECT id, actor_id, kind, data, created_at FROM forge_events WHERE solution_id=$1 ORDER BY id DESC LIMIT $2`, solutionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	var ids []string
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Actor.ID, &e.Kind, &e.Data, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
		ids = append(ids, e.Actor.ID)
	}
	users := s.userMap(ctx, ids)
	for i := range out {
		if u, ok := users[out[i].Actor.ID]; ok {
			out[i].Actor = u
		}
	}
	return out, rows.Err()
}
