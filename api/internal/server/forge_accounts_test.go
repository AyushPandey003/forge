package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razorpay/slashdocs/api/internal/forge"
	"github.com/razorpay/slashdocs/api/internal/server"
	"github.com/razorpay/slashdocs/api/internal/testdb"
)

func TestLocalRegistrationValidationAndLogin(t *testing.T) {
	t.Setenv("FORGE_LOCAL_AUTH", "true")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `INSERT INTO users(id,email,name) VALUES ($1,$2,'Owner')`, fixtureOwner.ID, fixtureOwner.Email)
	if err != nil {
		t.Fatal(err)
	}
	store := forge.NewStore(pool)
	if err := store.BootstrapLocalAdmin(ctx, fixtureOwner.ID, fixtureOwner.Email, fixtureOwner.Email); err != nil {
		t.Fatal(err)
	}
	_, token, err := store.NewAccountLink(ctx, fixtureOwner.ID, "invite", "member", "", "", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	e := newConfiguredTestEnv(t, server.Config{Environment: "test"}, server.WithForge(store))
	post := func(path string, body map[string]string, status int) {
		t.Helper()
		raw, _ := json.Marshal(body)
		rr := do(t, e.h, http.MethodPost, path, string(raw), map[string]string{"Content-Type": "application/json"})
		assertStatus(t, rr, status)
	}
	registration := map[string]string{"token": token, "name": "Validation User", "username": "validation", "password": "long-password-123", "confirm_password": "different-password"}
	post("/v1/auth/local/register", registration, 400)
	registration["confirm_password"] = ""
	post("/v1/auth/local/register", registration, 400)
	registration["password"] = "short"
	registration["confirm_password"] = "short"
	post("/v1/auth/local/register", registration, 400)
	// All failed submissions must leave the invitation usable.
	if _, err := store.InspectAccountLink(ctx, token); err != nil {
		t.Fatal(err)
	}
	registration["password"] = "long-password-123"
	registration["confirm_password"] = "long-password-123"
	post("/v1/auth/local/register", registration, 200)
	post("/v1/auth/local/login", map[string]string{"username": "validation", "password": "wrong-password-123"}, 400)
	post("/v1/auth/local/login", map[string]string{"username": "validation", "password": "long-password-123"}, 200)
	uid, err := store.LoginLocal(ctx, "validation", "long-password-123")
	if err != nil {
		t.Fatal(err)
	}
	_, reset, err := store.NewAccountLink(ctx, fixtureOwner.ID, "reset", "member", "", uid, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	post("/v1/auth/local/reset", map[string]string{"token": reset, "password": "new-password-123", "confirm_password": "mismatch"}, 400)
	if _, err := store.InspectAccountLink(ctx, reset); err != nil {
		t.Fatal(err)
	}
	post("/v1/auth/local/reset", map[string]string{"token": reset, "password": "new-password-123", "confirm_password": "new-password-123"}, 200)
	post("/v1/auth/local/login", map[string]string{"username": "validation", "password": "long-password-123"}, 400)
	post("/v1/auth/local/login", map[string]string{"username": "validation", "password": "new-password-123"}, 200)
}
