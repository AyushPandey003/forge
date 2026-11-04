package forge

import (
	"testing"
	"time"
)

func TestDefaultWorkflowValid(t *testing.T) {
	if err := DefaultWorkflow().Validate(); err != nil {
		t.Fatalf("default workflow invalid: %v", err)
	}
}

func TestValidateRejectsBadGate(t *testing.T) {
	w := Workflow{Stages: []Stage{{ID: "a", Label: "A", Gates: []Gate{{ID: "g", ApproverRoles: []string{"nope"}, Policy: PolicyAny, OnChange: OnChangeFlag}}}}}
	if err := w.Validate(); err == nil {
		t.Fatal("expected unknown role error")
	}
	w.Stages[0].Gates[0].ApproverRoles = []string{RoleSolutionApprover}
	w.Stages[0].Gates[0].Policy = PolicyN
	if err := w.Validate(); err == nil {
		t.Fatal("expected min_approvals error")
	}
}

func u(id string) User { return User{ID: id, Name: id} }

func approval(user, decision string, pinned map[string]string, at int) Approval {
	return Approval{User: u(user), Stage: "s", Gate: "g", Decision: decision, Pinned: pinned, CreatedAt: time.Unix(int64(at), 0)}
}

func TestGateAnyApproved(t *testing.T) {
	g := Gate{ID: "g", ApproverRoles: []string{RoleSolutionApprover}, Policy: PolicyAny, OnChange: OnChangeFlag}
	members := []Member{{User: u("alice"), Role: RoleSolutionApprover}}
	versions := map[string]string{"art1": "v1"}
	gs := evaluateGate(g, "s", members, []Approval{approval("alice", "approved", map[string]string{"art1": "v1"}, 1)}, versions, map[string]string{"art1": "PRD"}, "alice")
	if gs.State != "approved" || !gs.CanDecide || gs.Approved != 1 {
		t.Fatalf("got %+v", gs)
	}
}

func TestGateStaleFlagVsRevoke(t *testing.T) {
	members := []Member{{User: u("alice"), Role: RoleSolutionApprover}}
	versions := map[string]string{"art1": "v2"} // changed since approval
	apps := []Approval{approval("alice", "approved", map[string]string{"art1": "v1"}, 1)}

	flag := Gate{ID: "g", ApproverRoles: []string{RoleSolutionApprover}, Policy: PolicyAny, OnChange: OnChangeFlag}
	gs := evaluateGate(flag, "s", members, apps, versions, map[string]string{"art1": "PRD"}, "")
	if gs.State != "approved" || !gs.Decisions[0].Stale {
		t.Fatalf("flag: expected approved+stale, got %s stale=%v", gs.State, gs.Decisions[0].Stale)
	}

	revoke := flag
	revoke.OnChange = OnChangeRevoke
	gs = evaluateGate(revoke, "s", members, apps, versions, map[string]string{"art1": "PRD"}, "")
	if gs.State != "pending" {
		t.Fatalf("revoke: expected pending, got %s", gs.State)
	}
}

func TestGateAllAndLatestDecisionWins(t *testing.T) {
	g := Gate{ID: "g", ApproverRoles: []string{RoleUXApprover}, Policy: PolicyAll, OnChange: OnChangeFlag}
	members := []Member{{User: u("a"), Role: RoleUXApprover}, {User: u("b"), Role: RoleUXApprover}}
	// Newest first, as the store returns them.
	apps := []Approval{
		approval("a", "approved", nil, 3),
		approval("b", "approved", nil, 2),
		approval("a", "changes_requested", nil, 1), // superseded
	}
	gs := evaluateGate(g, "s", members, apps, map[string]string{}, nil, "")
	if gs.State != "approved" || gs.Required != 2 || gs.Approved != 2 {
		t.Fatalf("got state=%s required=%d approved=%d", gs.State, gs.Required, gs.Approved)
	}
	apps = append([]Approval{approval("b", "changes_requested", nil, 4)}, apps...)
	gs = evaluateGate(g, "s", members, apps, map[string]string{}, nil, "")
	if gs.State != "changes_requested" {
		t.Fatalf("expected changes_requested, got %s", gs.State)
	}
}

func TestGateOverrideAndNonApproverIgnored(t *testing.T) {
	g := Gate{ID: "g", ApproverRoles: []string{RoleSolutionApprover}, Policy: PolicyAny, OnChange: OnChangeFlag, AllowOverride: true}
	members := []Member{{User: u("approver"), Role: RoleSolutionApprover}, {User: u("owner"), Role: RoleProductOwner}}
	// A non-approver "approval" (without override) must not count.
	gs := evaluateGate(g, "s", members, []Approval{approval("owner", "approved", nil, 1)}, nil, nil, "owner")
	if gs.State != "pending" || gs.CanDecide {
		t.Fatalf("non-approver counted: %+v", gs)
	}
	ov := approval("owner", "approved", nil, 2)
	ov.Override = true
	gs = evaluateGate(g, "s", members, []Approval{ov}, nil, nil, "owner")
	if gs.State != "overridden" {
		t.Fatalf("expected overridden, got %s", gs.State)
	}
}

func TestStageReadyNeedsRequiredArtifacts(t *testing.T) {
	st := DefaultWorkflow().Stages[0] // problem
	members := []Member{{User: u("a"), Role: RoleSolutionApprover}}
	ss := evaluateStage(st, 0, 0, members, nil, []Approval{{User: u("a"), Stage: "problem", Gate: "solution", Decision: "approved"}}, "a")
	if ss.Ready || len(ss.Missing) != 1 {
		t.Fatalf("expected not ready with 1 missing, got ready=%v missing=%v", ss.Ready, ss.Missing)
	}
	arts := []Artifact{{ID: "x", Stage: "problem", Kind: "problem_statement", DocumentID: "d", CurrentVersionID: "v1", Title: "PS"}}
	apps := []Approval{{User: u("a"), Stage: "problem", Gate: "solution", Decision: "approved", Pinned: map[string]string{"x": "v1"}}}
	ss = evaluateStage(st, 0, 0, members, arts, apps, "a")
	if !ss.Ready {
		t.Fatalf("expected ready: %+v", ss.GateViews)
	}
}

func TestMarkdownRoundTrip(t *testing.T) {
	src := "# Title\n\n- [ ] task\n\n<script>x</script>\n"
	doc, err := BuildMarkdownDoc("T", src)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ExtractMarkdownSource(doc)
	if !ok || got != src {
		t.Fatalf("round trip failed: ok=%v got=%q", ok, got)
	}
}
