package forge_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/razorpay/slashdocs/api/internal/forge"
	"github.com/razorpay/slashdocs/api/internal/testdb"
)

func setup(t *testing.T) (*forge.Store, *pgxpool.Pool, string) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,name) VALUES ('usr_admin','admin@example.com','Admin')`); err != nil {
		t.Fatal(err)
	}
	s := forge.NewStore(pool)
	if err := s.BootstrapLocalAdmin(ctx, "usr_admin", "admin@example.com", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	return s, pool, "usr_admin"
}

func TestBootstrapOnlyConfiguredEmail(t *testing.T) {
	s, pool, _ := setup(t)
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `INSERT INTO users(id,email,name) VALUES ('usr_x','x@example.com','X')`)
	if err := s.BootstrapLocalAdmin(ctx, "usr_x", "x@example.com", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	if role, _ := s.WorkspaceRole(ctx, "usr_x"); role != "" {
		t.Fatalf("unconfigured user got role %q", role)
	}
	if role, _ := s.WorkspaceRole(ctx, "usr_admin"); role != "owner" {
		t.Fatalf("admin role = %q", role)
	}
}

func TestInviteRegisterLoginAndSingleUse(t *testing.T) {
	s, _, admin := setup(t)
	ctx := context.Background()
	team, err := s.CreateTeam(ctx, "Platform")
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := s.NewAccountLink(ctx, admin, "invite", "member", team.ID, "", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RegisterLocal(ctx, token, "Asha", "asha", "short"); !errors.Is(err, forge.ErrInvalid) {
		t.Fatalf("weak password accepted: %v", err)
	}
	uid, err := s.RegisterLocal(ctx, token, "Asha Rao", "Asha", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if role, _ := s.WorkspaceRole(ctx, uid); role != "member" {
		t.Fatalf("role = %q", role)
	}
	if _, err := s.RegisterLocal(ctx, token, "Again", "again", "correct horse battery"); !errors.Is(err, forge.ErrBadCredential) {
		t.Fatalf("invite reused: %v", err)
	}
	if got, err := s.LoginLocal(ctx, "ASHA", "correct horse battery"); err != nil || got != uid {
		t.Fatalf("login = %q %v", got, err)
	}
	if _, err := s.LoginLocal(ctx, "asha", "wrong password!!"); !errors.Is(err, forge.ErrBadCredential) {
		t.Fatalf("wrong password accepted: %v", err)
	}
	if _, err := s.LoginLocal(ctx, "nobody", "whatever password"); !errors.Is(err, forge.ErrBadCredential) {
		t.Fatalf("unknown user: %v", err)
	}
}

func TestExpiredAndRevokedInvites(t *testing.T) {
	s, _, admin := setup(t)
	ctx := context.Background()
	_, expired, _ := s.NewAccountLink(ctx, admin, "invite", "member", "", "", -time.Minute)
	if _, err := s.RegisterLocal(ctx, expired, "A", "aaa", "correct horse battery"); !errors.Is(err, forge.ErrBadCredential) {
		t.Fatalf("expired invite accepted: %v", err)
	}
	l, revoked, _ := s.NewAccountLink(ctx, admin, "invite", "member", "", "", time.Hour)
	if err := s.RevokeAccountLink(ctx, l.ID, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RegisterLocal(ctx, revoked, "B", "bbb", "correct horse battery"); !errors.Is(err, forge.ErrBadCredential) {
		t.Fatalf("revoked invite accepted: %v", err)
	}
}

func TestResetPasswordRevokesSessionsAndOldLinks(t *testing.T) {
	s, _, admin := setup(t)
	ctx := context.Background()
	_, inv, _ := s.NewAccountLink(ctx, admin, "invite", "member", "", "", time.Hour)
	uid, err := s.RegisterLocal(ctx, inv, "Ravi", "ravi", "first password 123")
	if err != nil {
		t.Fatal(err)
	}
	session, _ := s.NewLocalSession(ctx, uid)
	_, oldReset, _ := s.NewAccountLink(ctx, admin, "reset", "member", "", uid, time.Hour)
	_, reset, _ := s.NewAccountLink(ctx, admin, "reset", "member", "", uid, time.Hour)
	if err := s.ResetLocalPassword(ctx, oldReset, "another password 1"); !errors.Is(err, forge.ErrBadCredential) {
		t.Fatalf("superseded reset link worked: %v", err)
	}
	if err := s.ResetLocalPassword(ctx, reset, "second password 456"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LocalSessionUser(ctx, session); !errors.Is(err, forge.ErrBadCredential) {
		t.Fatalf("old session survived reset: %v", err)
	}
	if _, err := s.LoginLocal(ctx, "ravi", "first password 123"); err == nil {
		t.Fatal("old password still works")
	}
	if _, err := s.LoginLocal(ctx, "ravi", "second password 456"); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetLocalPassword(ctx, reset, "third password 789"); !errors.Is(err, forge.ErrBadCredential) {
		t.Fatalf("reset link reused: %v", err)
	}
}

func TestAdminCannotResetOwner(t *testing.T) {
	s, _, owner := setup(t)
	ctx := context.Background()
	_, inv, _ := s.NewAccountLink(ctx, owner, "invite", "admin", "", "", time.Hour)
	adminID, err := s.RegisterLocal(ctx, inv, "Admin Two", "admin2", "admin password 12")
	if err != nil {
		t.Fatal(err)
	}
	_, inv2, _ := s.NewAccountLink(ctx, owner, "invite", "admin", "", "", time.Hour)
	otherAdmin, _ := s.RegisterLocal(ctx, inv2, "Admin Three", "admin3", "admin password 34")
	if _, _, err := s.NewAccountLink(ctx, adminID, "reset", "member", "", otherAdmin, time.Hour); !errors.Is(err, forge.ErrForbidden) {
		t.Fatalf("admin reset another admin: %v", err)
	}
	if _, _, err := s.NewAccountLink(ctx, owner, "reset", "member", "", otherAdmin, time.Hour); err != nil {
		t.Fatalf("owner could not reset admin: %v", err)
	}
}
