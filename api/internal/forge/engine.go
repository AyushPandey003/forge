package forge

import (
	"context"
	"fmt"
	"sort"
)

// GateStatus is the evaluated state of one gate.
type GateStatus struct {
	Gate
	State string `json:"state"` // pending | approved | changes_requested | rejected | overridden
	// Decisions: the current (latest) decision per approver.
	Decisions []DecisionView `json:"decisions"`
	// Approvers: members eligible to decide this gate.
	Approvers []User `json:"approvers"`
	Approved  int    `json:"approved"`
	Required  int    `json:"required"`
	// CanDecide: the viewer holds an approver role for this gate.
	CanDecide bool `json:"can_decide"`
}

type DecisionView struct {
	Approval
	// Stale: an artifact changed since this decision.
	Stale        bool     `json:"stale"`
	StaleReasons []string `json:"stale_reasons,omitempty"`
	Counts       bool     `json:"counts"`
}

type ArtifactSlot struct {
	ArtifactKind
	Items []Artifact `json:"items"`
}

type StageStatus struct {
	Stage
	Index     int            `json:"index"`
	State     string         `json:"state"` // done | current | upcoming
	Slots     []ArtifactSlot `json:"slots"`
	GateViews []GateStatus   `json:"gate_status"`
	// Missing: labels of required artifacts not yet provided.
	Missing []string `json:"missing"`
	// Ready: required artifacts present and every gate approved/overridden.
	Ready bool `json:"ready"`
}

type Board struct {
	Solution  Solution      `json:"solution"`
	Members   []Member      `json:"members"`
	Stages    []StageStatus `json:"stages"`
	ViewerRoles []string    `json:"viewer_roles"`
	CanManage bool          `json:"can_manage"`
	CanAdvance bool         `json:"can_advance"`
	// Progress 0..1 across all stages.
	Progress float64 `json:"progress"`
}

// IsManager: owners can edit the solution, its members and override gates.
func IsManager(roles map[string]bool, isCreator bool) bool {
	return isCreator || roles[RoleProductOwner] || roles[RoleEngineeringOwner] || roles[RoleUXOwner]
}

// BuildBoard evaluates the whole solution for one viewer.
func (s *Store) BuildBoard(ctx context.Context, solutionID, viewerID string) (Board, error) {
	sol, err := s.GetSolution(ctx, solutionID)
	if err != nil {
		return Board{}, err
	}
	members, err := s.Members(ctx, sol.ID)
	if err != nil {
		return Board{}, err
	}
	arts, err := s.Artifacts(ctx, sol.ID)
	if err != nil {
		return Board{}, err
	}
	approvals, err := s.Approvals(ctx, sol.ID)
	if err != nil {
		return Board{}, err
	}
	viewerRoles := map[string]bool{}
	for _, m := range members {
		if m.User.ID == viewerID {
			viewerRoles[m.Role] = true
		}
	}
	b := Board{Solution: sol, Members: members}
	for r := range viewerRoles {
		b.ViewerRoles = append(b.ViewerRoles, r)
	}
	sort.Strings(b.ViewerRoles)
	b.CanManage = IsManager(viewerRoles, sol.CreatedBy.ID == viewerID)

	_, curIdx, _ := sol.Workflow.Stage(sol.CurrentStage)
	done := 0
	for i, st := range sol.Workflow.Stages {
		ss := evaluateStage(st, i, curIdx, members, arts, approvals, viewerID)
		if ss.State == "done" {
			done++
		}
		b.Stages = append(b.Stages, ss)
	}
	if curIdx >= 0 && curIdx < len(b.Stages) {
		cur := b.Stages[curIdx]
		b.CanAdvance = cur.Ready && b.CanManage
		if cur.Ready {
			done++ // a ready current stage counts as complete for progress
		}
	}
	if sol.Status == "shipped" {
		done = len(b.Stages)
	}
	if n := len(b.Stages); n > 0 {
		b.Progress = float64(min(done, n)) / float64(n)
	}
	return b, nil
}

func evaluateStage(st Stage, idx, curIdx int, members []Member, arts []Artifact, approvals []Approval, viewerID string) StageStatus {
	ss := StageStatus{Stage: st, Index: idx}
	switch {
	case idx < curIdx:
		ss.State = "done"
	case idx == curIdx:
		ss.State = "current"
	default:
		ss.State = "upcoming"
	}
	// Artifacts by slot; current version per artifact for staleness.
	versions := map[string]string{}
	titles := map[string]string{}
	for _, k := range st.Artifacts {
		slot := ArtifactSlot{ArtifactKind: k}
		for _, a := range arts {
			if a.Stage == st.ID && a.Kind == k.Kind {
				slot.Items = append(slot.Items, a)
				if a.DocumentID != "" {
					versions[a.ID] = a.CurrentVersionID
				}
				titles[a.ID] = a.Title
			}
		}
		if k.Required && len(slot.Items) == 0 {
			ss.Missing = append(ss.Missing, k.Label)
		}
		ss.Slots = append(ss.Slots, slot)
	}
	allGates := true
	for _, g := range st.Gates {
		gs := evaluateGate(g, st.ID, members, approvals, versions, titles, viewerID)
		if gs.State != "approved" && gs.State != "overridden" {
			allGates = false
		}
		ss.GateViews = append(ss.GateViews, gs)
	}
	ss.Ready = len(ss.Missing) == 0 && allGates
	return ss
}

func evaluateGate(g Gate, stage string, members []Member, approvals []Approval, versions, titles map[string]string, viewerID string) GateStatus {
	gs := GateStatus{Gate: g, State: "pending"}
	eligible := map[string]User{}
	for _, m := range members {
		for _, r := range g.ApproverRoles {
			if m.Role == r {
				eligible[m.User.ID] = m.User
			}
		}
	}
	for _, u := range eligible {
		gs.Approvers = append(gs.Approvers, u)
	}
	sort.Slice(gs.Approvers, func(i, j int) bool { return gs.Approvers[i].Name+gs.Approvers[i].Email < gs.Approvers[j].Name+gs.Approvers[j].Email })
	_, gs.CanDecide = eligible[viewerID]

	// Latest decision per user (approvals are newest-first). An override is
	// recorded by a manager who may not be an approver; keep it regardless.
	seen := map[string]bool{}
	var overridden bool
	for _, a := range approvals {
		if a.Stage != stage || a.Gate != g.ID || seen[a.User.ID] {
			continue
		}
		seen[a.User.ID] = true
		if a.Decision == "withdrawn" {
			continue
		}
		dv := DecisionView{Approval: a}
		for artID, ver := range versions {
			if pinned, ok := a.Pinned[artID]; !ok || pinned != ver {
				dv.Stale = true
				if ok {
					dv.StaleReasons = append(dv.StaleReasons, fmt.Sprintf("%s changed", titles[artID]))
				} else {
					dv.StaleReasons = append(dv.StaleReasons, fmt.Sprintf("%s added", titles[artID]))
				}
			}
		}
		sort.Strings(dv.StaleReasons)
		_, isApprover := eligible[a.User.ID]
		dv.Counts = (isApprover || a.Override) && !(dv.Stale && g.OnChange == OnChangeRevoke)
		if a.Override && a.Decision == "approved" && dv.Counts {
			overridden = true
		}
		gs.Decisions = append(gs.Decisions, dv)
	}

	approved, blocked := 0, ""
	for _, d := range gs.Decisions {
		if !d.Counts {
			continue
		}
		switch d.Decision {
		case "approved":
			approved++
		case "rejected":
			blocked = "rejected"
		case "changes_requested":
			if blocked == "" {
				blocked = "changes_requested"
			}
		}
	}
	gs.Approved = approved
	switch g.Policy {
	case PolicyAll:
		gs.Required = max(len(eligible), 1)
	case PolicyN:
		gs.Required = g.MinApprovals
	default:
		gs.Required = 1
	}
	switch {
	case overridden:
		gs.State = "overridden"
	case blocked != "":
		gs.State = blocked
	case approved >= gs.Required:
		gs.State = "approved"
	}
	return gs
}

// PinnedVersions snapshots the current version of every artifact in a stage.
func (s *Store) PinnedVersions(ctx context.Context, solutionID, stage string) (map[string]string, error) {
	arts, err := s.Artifacts(ctx, solutionID)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, a := range arts {
		if a.Stage == stage && a.DocumentID != "" {
			out[a.ID] = a.CurrentVersionID
		}
	}
	return out, nil
}
