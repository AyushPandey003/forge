package forge_test

import (
	"context"
	"testing"

	"github.com/razorpay/slashdocs/api/internal/forge"
)

func TestLiveRevisionTransactionalAndPrivate(t *testing.T) {
	s, pool, admin := setup(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO users(id,email,name) VALUES ('viewer','viewer@local.invalid','Viewer'),('outsider','outsider@local.invalid','Outsider')`)
	if err != nil {
		t.Fatal(err)
	}
	sol, err := s.CreateSolution(ctx, forge.CreateSolutionInput{Title: "Live", CreatedBy: admin})
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.LiveRevision(ctx, admin)
	if err != nil || before == 0 {
		t.Fatalf("create revision %d: %v", before, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE forge_solutions SET title='Rolled back' WHERE id=$1`, sol.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	after, _ := s.LiveRevision(ctx, admin)
	if after != before {
		t.Fatal("rollback leaked invalidation")
	}
	_, err = pool.Exec(ctx, `INSERT INTO forge_members(solution_id,user_id,role) VALUES ($1,'viewer','consumer')`, sol.ID)
	if err != nil {
		t.Fatal(err)
	}
	joined, _ := s.LiveRevision(ctx, "viewer")
	if joined == 0 {
		t.Fatal("new member not notified")
	}
	_, err = pool.Exec(ctx, `DELETE FROM forge_members WHERE solution_id=$1 AND user_id='viewer'`, sol.ID)
	if err != nil {
		t.Fatal(err)
	}
	removed, _ := s.LiveRevision(ctx, "viewer")
	if removed <= joined {
		t.Fatal("removed member not notified")
	}
	_, err = pool.Exec(ctx, `UPDATE forge_solutions SET title='After removal' WHERE id=$1`, sol.ID)
	if err != nil {
		t.Fatal(err)
	}
	later, _ := s.LiveRevision(ctx, "viewer")
	if later != removed {
		t.Fatal("removed member receives future changes")
	}
	outside, _ := s.LiveRevision(ctx, "outsider")
	if outside != 0 {
		t.Fatal("private solution leaked to outsider")
	}
}
