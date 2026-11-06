package server_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razorpay/slashdocs/api/internal/forge"
	"github.com/razorpay/slashdocs/api/internal/server"
	"github.com/razorpay/slashdocs/api/internal/testdb"
)

func TestForgeLiveInitialUpdateAndReconnect(t *testing.T) {
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
	e := newConfiguredTestEnv(t, server.Config{Environment: "test"}, server.WithForge(store))
	unauth := do(t, e.h, http.MethodGet, "/v1/forge/live", "", nil)
	assertStatus(t, unauth, http.StatusUnauthorized)
	ts := httptest.NewServer(e.h)
	defer ts.Close()
	open := func() (*http.Response, context.CancelFunc) {
		t.Helper()
		ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/forge/live", nil)
		req.Header.Set("X-Test-Principal", principalHeader(fixtureOwner))
		req.Header.Set("Last-Event-ID", "999999") // snapshot reset must still happen
		resp, err := ts.Client().Do(req)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
			t.Fatalf("bad stream: %s", resp.Status)
		}
		return resp, cancel
	}
	read := func(r *bufio.Reader) string {
		t.Helper()
		var event strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			event.WriteString(line)
			if line == "\n" {
				break
			}
		}
		if !strings.Contains(event.String(), "event: invalidate") {
			t.Fatalf("unexpected event %q", event.String())
		}
		return event.String()
	}
	resp, cancel := open()
	reader := bufio.NewReader(resp.Body)
	initial := read(reader)
	_, err = store.CreateSolution(ctx, forge.CreateSolutionInput{Title: "Live board", CreatedBy: fixtureOwner.ID})
	if err != nil {
		t.Fatal(err)
	}
	changed := read(reader)
	if changed == initial {
		t.Fatal("change did not advance cursor")
	}
	resp.Body.Close()
	cancel()
	_, err = store.CreateSolution(ctx, forge.CreateSolutionInput{Title: "While disconnected", CreatedBy: fixtureOwner.ID})
	if err != nil {
		t.Fatal(err)
	}
	resp, cancel = open()
	defer cancel()
	defer resp.Body.Close()
	recovered := read(bufio.NewReader(resp.Body))
	if recovered == changed {
		t.Fatal("reconnect missed disconnected change")
	}
}
