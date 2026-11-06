package forge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const LocalWorkspace = "local"

var ErrBadCredential = errors.New("invalid or expired credentials")
var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,31}$`)

func randomToken() string       { b := make([]byte, 32); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func TokenHash(t string) string { h := sha256.Sum256([]byte(t)); return hex.EncodeToString(h[:]) }
func PasswordHash(password string) (string, error) {
	if len(password) < 12 || len(password) > 72 {
		return "", fmt.Errorf("%w: password must be 12–72 bytes", ErrInvalid)
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(b), err
}

type AccountMember struct {
	User
	Username string `json:"username"`
	Role     string `json:"role"`
}
type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type AccountLink struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Role      string    `json:"role"`
	TeamID    string    `json:"team_id"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	Revoked   bool      `json:"revoked"`
}

// Bootstrap only accepts the explicitly configured development administrator.
// Public signup and first-visitor-is-admin are intentionally not supported.
func (s *Store) BootstrapLocalAdmin(ctx context.Context, userID, email, allowed string) error {
	if allowed == "" || !strings.EqualFold(email, allowed) {
		return nil
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO forge_workspaces(id,name) VALUES ('local','Local workspace') ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge_workspace_members(workspace_id,user_id,role) VALUES ('local',$1,'owner') ON CONFLICT DO NOTHING`, userID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) WorkspaceRole(ctx context.Context, userID string) (string, error) {
	var role string
	err := s.db.QueryRow(ctx, `SELECT role FROM forge_workspace_members WHERE workspace_id='local' AND user_id=$1`, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, err
}
func (s *Store) AccountMembers(ctx context.Context) ([]AccountMember, error) {
	rows, err := s.db.Query(ctx, `SELECT u.id,u.email,u.name,u.picture_url,COALESCE(a.username,''),m.role FROM forge_workspace_members m JOIN users u ON u.id=m.user_id LEFT JOIN forge_local_accounts a ON a.user_id=u.id WHERE m.workspace_id='local' ORDER BY u.name,u.email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AccountMember{}
	for rows.Next() {
		var m AccountMember
		if err = rows.Scan(&m.ID, &m.Email, &m.Name, &m.PictureURL, &m.Username, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) Teams(ctx context.Context) ([]Team, error) {
	rows, err := s.db.Query(ctx, `SELECT id,name FROM forge_teams WHERE workspace_id='local' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Team{}
	for rows.Next() {
		var t Team
		if err = rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Store) CreateTeam(ctx context.Context, name string) (Team, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return Team{}, fmt.Errorf("%w: team name is required (max 100 characters)", ErrInvalid)
	}
	t := Team{ID: newID("team"), Name: name}
	_, err := s.db.Exec(ctx, `INSERT INTO forge_teams(id,workspace_id,name) VALUES ($1,'local',$2)`, t.ID, t.Name)
	return t, err
}
func (s *Store) AccountLinks(ctx context.Context) ([]AccountLink, error) {
	rows, err := s.db.Query(ctx, `SELECT id,kind,role,COALESCE(team_id,''),COALESCE(user_id,''),expires_at,used_at IS NOT NULL,revoked_at IS NOT NULL FROM forge_account_links WHERE workspace_id='local' ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AccountLink{}
	for rows.Next() {
		var l AccountLink
		if err = rows.Scan(&l.ID, &l.Kind, &l.Role, &l.TeamID, &l.UserID, &l.ExpiresAt, &l.Used, &l.Revoked); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// NewAccountLink stores only a hash; the raw token is returned once to the admin.
func (s *Store) NewAccountLink(ctx context.Context, actor, kind, role, team, userID string, ttl time.Duration) (AccountLink, string, error) {
	if role != "admin" && role != "member" && role != "guest" {
		return AccountLink{}, "", fmt.Errorf("%w: unknown workspace role", ErrInvalid)
	}
	if kind != "invite" && kind != "reset" {
		return AccountLink{}, "", ErrInvalid
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return AccountLink{}, "", err
	}
	defer tx.Rollback(ctx)
	if team != "" {
		var n int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM forge_teams WHERE id=$1 AND workspace_id='local'`, team).Scan(&n)
		if err != nil {
			return AccountLink{}, "", err
		}
		if n != 1 {
			return AccountLink{}, "", ErrNotFound
		}
	}
	if kind == "reset" {
		var targetRole string
		err = tx.QueryRow(ctx, `SELECT m.role FROM forge_workspace_members m JOIN forge_local_accounts a ON a.user_id=m.user_id WHERE m.workspace_id='local' AND m.user_id=$1`, userID).Scan(&targetRole)
		if errors.Is(err, pgx.ErrNoRows) {
			return AccountLink{}, "", ErrNotFound
		}
		if err != nil {
			return AccountLink{}, "", err
		}
		var actorRole string
		err = tx.QueryRow(ctx, `SELECT role FROM forge_workspace_members WHERE workspace_id='local' AND user_id=$1`, actor).Scan(&actorRole)
		if err != nil {
			return AccountLink{}, "", err
		}
		if actorRole != "owner" && (targetRole == "owner" || targetRole == "admin") {
			return AccountLink{}, "", ErrForbidden
		}
		// A new reset supersedes previously issued reset links.
		if _, err = tx.Exec(ctx, `UPDATE forge_account_links SET revoked_at=now() WHERE kind='reset' AND user_id=$1 AND used_at IS NULL AND revoked_at IS NULL`, userID); err != nil {
			return AccountLink{}, "", err
		}
	}
	token := randomToken()
	l := AccountLink{ID: newID("link"), Kind: kind, Role: role, TeamID: team, UserID: userID, ExpiresAt: time.Now().Add(ttl)}
	_, err = tx.Exec(ctx, `INSERT INTO forge_account_links(id,token_hash,kind,workspace_id,role,team_id,user_id,created_by,expires_at) VALUES ($1,$2,$3,'local',$4,NULLIF($5,''),NULLIF($6,''),$7,$8)`, l.ID, TokenHash(token), kind, role, team, userID, actor, l.ExpiresAt)
	if err != nil {
		return l, "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge_account_events(workspace_id,actor_id,kind,target_id) VALUES ('local',$1,$2,$3)`, actor, kind+".created", l.ID)
	if err != nil {
		return l, "", err
	}
	return l, token, tx.Commit(ctx)
}
func (s *Store) RevokeAccountLink(ctx context.Context, id, actor string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE forge_account_links SET revoked_at=now() WHERE id=$1 AND workspace_id='local' AND used_at IS NULL AND revoked_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge_account_events(workspace_id,actor_id,kind,target_id) VALUES ('local',$1,'link.revoked',$2)`, actor, id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) InspectAccountLink(ctx context.Context, token string) (AccountLink, error) {
	var l AccountLink
	err := s.db.QueryRow(ctx, `SELECT id,kind,role,COALESCE(team_id,''),COALESCE(user_id,''),expires_at FROM forge_account_links WHERE token_hash=$1 AND used_at IS NULL AND revoked_at IS NULL AND expires_at>now()`, TokenHash(token)).Scan(&l.ID, &l.Kind, &l.Role, &l.TeamID, &l.UserID, &l.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, ErrBadCredential
	}
	return l, err
}
func (s *Store) RegisterLocal(ctx context.Context, token, name, username, password string) (string, error) {
	name = strings.TrimSpace(name)
	username = strings.ToLower(strings.TrimSpace(username))
	if name == "" || len(name) > 100 || !usernamePattern.MatchString(username) {
		return "", fmt.Errorf("%w: name is required; username must be 3–32 letters, digits, underscores or hyphens", ErrInvalid)
	}
	hash, err := PasswordHash(password)
	if err != nil {
		return "", err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var id, role, team string
	err = tx.QueryRow(ctx, `SELECT id,role,COALESCE(team_id,'') FROM forge_account_links WHERE token_hash=$1 AND kind='invite' AND used_at IS NULL AND revoked_at IS NULL AND expires_at>now() FOR UPDATE`, TokenHash(token)).Scan(&id, &role, &team)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrBadCredential
	}
	if err != nil {
		return "", err
	}
	uid := newID("usr")
	// Local identities deliberately use a non-routable email domain; not verified email.
	_, err = tx.Exec(ctx, `INSERT INTO users(id,email,name) VALUES ($1,$2,$3)`, uid, username+"@local.invalid", name)
	if err != nil {
		return "", fmt.Errorf("%w: username unavailable", ErrInvalid)
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge_local_accounts(user_id,username,password_hash) VALUES ($1,$2,$3)`, uid, username, hash)
	if err != nil {
		return "", fmt.Errorf("%w: username unavailable", ErrInvalid)
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge_workspace_members(workspace_id,user_id,role) VALUES ('local',$1,$2)`, uid, role)
	if err != nil {
		return "", err
	}
	if team != "" {
		if _, err = tx.Exec(ctx, `INSERT INTO forge_team_members(team_id,user_id) VALUES ($1,$2)`, team, uid); err != nil {
			return "", err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE forge_account_links SET used_at=now() WHERE id=$1`, id); err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge_account_events(workspace_id,actor_id,kind,target_id) VALUES ('local',$1,'account.registered',$2)`, uid, id)
	if err != nil {
		return "", err
	}
	return uid, tx.Commit(ctx)
}

// Dummy hash makes nonexistent-user login perform the same bcrypt work.
var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-not-a-login"), 12)

func (s *Store) LoginLocal(ctx context.Context, username, password string) (string, error) {
	var uid, hash string
	err := s.db.QueryRow(ctx, `SELECT user_id,password_hash FROM forge_local_accounts WHERE username=$1`, strings.ToLower(strings.TrimSpace(username))).Scan(&uid, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if hash == "" {
		hash = string(dummyPasswordHash)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil || uid == "" {
		return "", ErrBadCredential
	}
	return uid, nil
}
func (s *Store) NewLocalSession(ctx context.Context, userID string) (string, error) {
	token := randomToken()
	_, err := s.db.Exec(ctx, `INSERT INTO forge_local_sessions(token_hash,user_id,expires_at) VALUES ($1,$2,now()+interval '30 days')`, TokenHash(token), userID)
	return token, err
}
func (s *Store) LocalSessionUser(ctx context.Context, token string) (string, error) {
	var uid string
	err := s.db.QueryRow(ctx, `SELECT user_id FROM forge_local_sessions WHERE token_hash=$1 AND expires_at>now()`, TokenHash(token)).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrBadCredential
	}
	return uid, err
}
func (s *Store) DeleteLocalSession(ctx context.Context, token string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM forge_local_sessions WHERE token_hash=$1`, TokenHash(token))
	return err
}
func (s *Store) ResetLocalPassword(ctx context.Context, token, password string) error {
	hash, err := PasswordHash(password)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id, uid string
	err = tx.QueryRow(ctx, `SELECT id,user_id FROM forge_account_links WHERE token_hash=$1 AND kind='reset' AND used_at IS NULL AND revoked_at IS NULL AND expires_at>now() FOR UPDATE`, TokenHash(token)).Scan(&id, &uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBadCredential
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE forge_local_accounts SET password_hash=$2 WHERE user_id=$1`, uid, hash); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM forge_local_sessions WHERE user_id=$1`, uid); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE forge_account_links SET used_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge_account_events(workspace_id,actor_id,kind,target_id) VALUES ('local',$1,'password.reset',$2)`, uid, id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
