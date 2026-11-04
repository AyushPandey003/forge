// Package forge implements Forge's product-delivery domain: Solutions that
// move through a configurable workflow of stages, collecting artifacts and
// closing at version-pinned approval gates.
package forge

import (
	"errors"
	"fmt"
)

// Roles a member can hold on a solution. A user may hold several.
const (
	RoleProductOwner           = "product_owner"
	RoleEngineeringOwner       = "engineering_owner"
	RoleUXOwner                = "ux_owner"
	RoleImplementer            = "implementer"
	RoleSolutionApprover       = "solution_approver"
	RoleImplementationApprover = "implementation_approver"
	RoleUXApprover             = "ux_approver"
	RoleConsumer               = "consumer"
)

// AllRoles is the ordered role catalogue (UI order).
var AllRoles = []RoleInfo{
	{RoleProductOwner, "Product owner", "Owns the problem and the PRD."},
	{RoleEngineeringOwner, "Engineering owner", "Owns the implementation doc and delivery."},
	{RoleUXOwner, "UX owner", "Owns designs and prototypes."},
	{RoleImplementer, "Implementer", "Builds the solution."},
	{RoleSolutionApprover, "Solution approver", "Signs off on the problem and the solution."},
	{RoleImplementationApprover, "Implementation approver", "Signs off on the implementation plan."},
	{RoleUXApprover, "UX approver", "Signs off on the user experience."},
	{RoleConsumer, "Solution consumer", "Uses the solution; follows progress and gives feedback."},
}

type RoleInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

func validRole(r string) bool {
	for _, x := range AllRoles {
		if x.ID == r {
			return true
		}
	}
	return false
}

// Approval policies for a gate.
const (
	PolicyAny = "any" // one approval from any approver role holder
	PolicyAll = "all" // every member holding an approver role
	PolicyN   = "n"   // at least MinApprovals approvals
)

// Behaviour when an artifact gets a new version after approval.
const (
	OnChangeFlag   = "flag"   // keep approval, mark it stale
	OnChangeRevoke = "revoke" // approval no longer counts
)

// ArtifactKind describes a slot a stage expects to be filled.
type ArtifactKind struct {
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Multiple    bool   `json:"multiple"`
	// Template is the starter content key (see templates.go); "" = none.
	Template string `json:"template,omitempty"`
	// External means this slot holds a link, not a document.
	External bool `json:"external,omitempty"`
}

// Gate is one sign-off within a stage. A stage passes when all its gates pass.
type Gate struct {
	ID            string   `json:"id"`
	Label         string   `json:"label"`
	Question      string   `json:"question"` // e.g. "Yes, this is the problem"
	ApproverRoles []string `json:"approver_roles"`
	Policy        string   `json:"policy"`
	MinApprovals  int      `json:"min_approvals,omitempty"`
	OnChange      string   `json:"on_change"`
	// AllowOverride lets owners force the gate open (always audited).
	AllowOverride bool `json:"allow_override"`
}

type Stage struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Description string         `json:"description,omitempty"`
	Artifacts   []ArtifactKind `json:"artifacts"`
	Gates       []Gate         `json:"gates"`
	// AIChecks lists checks offered at this stage (e.g. "conflict").
	AIChecks []string `json:"ai_checks,omitempty"`
}

type Workflow struct {
	Stages []Stage `json:"stages"`
}

func (w Workflow) Stage(id string) (Stage, int, bool) {
	for i, s := range w.Stages {
		if s.ID == id {
			return s, i, true
		}
	}
	return Stage{}, -1, false
}

// Validate checks a workflow definition before it is saved.
func (w Workflow) Validate() error {
	if len(w.Stages) == 0 {
		return errors.New("workflow needs at least one stage")
	}
	seen := map[string]bool{}
	for _, s := range w.Stages {
		if s.ID == "" || s.Label == "" {
			return errors.New("every stage needs an id and a label")
		}
		if seen[s.ID] {
			return fmt.Errorf("duplicate stage id %q", s.ID)
		}
		seen[s.ID] = true
		kinds := map[string]bool{}
		for _, a := range s.Artifacts {
			if a.Kind == "" {
				return fmt.Errorf("stage %q: artifact without kind", s.ID)
			}
			if kinds[a.Kind] {
				return fmt.Errorf("stage %q: duplicate artifact kind %q", s.ID, a.Kind)
			}
			kinds[a.Kind] = true
		}
		gates := map[string]bool{}
		for _, g := range s.Gates {
			if g.ID == "" || gates[g.ID] {
				return fmt.Errorf("stage %q: gate ids must be unique and non-empty", s.ID)
			}
			gates[g.ID] = true
			if len(g.ApproverRoles) == 0 {
				return fmt.Errorf("stage %q gate %q: needs approver roles", s.ID, g.ID)
			}
			for _, r := range g.ApproverRoles {
				if !validRole(r) {
					return fmt.Errorf("stage %q gate %q: unknown role %q", s.ID, g.ID, r)
				}
			}
			switch g.Policy {
			case PolicyAny, PolicyAll:
			case PolicyN:
				if g.MinApprovals < 1 {
					return fmt.Errorf("stage %q gate %q: min_approvals must be >= 1", s.ID, g.ID)
				}
			default:
				return fmt.Errorf("stage %q gate %q: unknown policy %q", s.ID, g.ID, g.Policy)
			}
			if g.OnChange != OnChangeFlag && g.OnChange != OnChangeRevoke {
				return fmt.Errorf("stage %q gate %q: on_change must be flag or revoke", s.ID, g.ID)
			}
		}
	}
	return nil
}

// DefaultWorkflow mirrors the team's delivery flow:
// Problem -> PRD -> Implementation -> Release, each closed by a gate.
func DefaultWorkflow() Workflow {
	return Workflow{Stages: []Stage{
		{
			ID: "problem", Label: "Problem",
			Description: "Agree on the problem before anyone designs a solution.",
			Artifacts: []ArtifactKind{
				{Kind: "problem_statement", Label: "Problem statement", Description: "Text and images describing the problem.", Required: true, Template: "problem_statement"},
				{Kind: "user_interview", Label: "User interviews", Description: "Notes or transcripts from user interviews.", Multiple: true, Template: "user_interview"},
				{Kind: "incident", Label: "IRCA / incidents", Description: "Incident root-cause analyses that motivate the work.", Multiple: true, Template: "incident"},
			},
			Gates: []Gate{{
				ID: "solution", Label: "Problem approval", Question: "Yes, this is the problem.",
				ApproverRoles: []string{RoleSolutionApprover}, Policy: PolicyAny, OnChange: OnChangeFlag, AllowOverride: true,
			}},
		},
		{
			ID: "prd", Label: "PRD",
			Description: "Define the solution: stories, acceptance criteria, UX and prototypes.",
			Artifacts: []ArtifactKind{
				{Kind: "prd", Label: "PRD", Description: "User stories and acceptance criteria.", Required: true, Template: "prd"},
				{Kind: "design", Label: "UX designs", Description: "Designs generated with Stitch + Katalyst, or uploaded.", Multiple: true},
				{Kind: "interview_schedule", Label: "User interview plan", Description: "Who to interview and when.", Template: "interview_plan"},
				{Kind: "prototype", Label: "Prototype (code)", Description: "A runnable prototype document.", Multiple: true},
				{Kind: "prototype_link", Label: "Prototype (link)", Description: "Link to a hosted prototype.", Multiple: true, External: true},
			},
			Gates: []Gate{
				{ID: "solution", Label: "Solution approval", Question: "This solution makes sense.", ApproverRoles: []string{RoleSolutionApprover}, Policy: PolicyAny, OnChange: OnChangeFlag, AllowOverride: true},
				{ID: "implementation", Label: "Implementation approval", Question: "This is buildable as specified.", ApproverRoles: []string{RoleImplementationApprover}, Policy: PolicyAny, OnChange: OnChangeFlag, AllowOverride: true},
				{ID: "ux", Label: "UX approval", Question: "The experience is right.", ApproverRoles: []string{RoleUXApprover}, Policy: PolicyAny, OnChange: OnChangeFlag, AllowOverride: true},
			},
			AIChecks: []string{"conflict"},
		},
		{
			ID: "implementation", Label: "Implementation",
			Description: "Plan the build: HLD, LLD and non-functional requirements.",
			Artifacts: []ArtifactKind{
				{Kind: "impl_doc", Label: "Implementation doc", Description: "HLD, LLD and non-functional requirements.", Required: true, Template: "impl_doc"},
			},
			Gates: []Gate{{
				ID: "implementation", Label: "Implementation approval", Question: "Yes.",
				ApproverRoles: []string{RoleImplementationApprover}, Policy: PolicyAny, OnChange: OnChangeFlag, AllowOverride: true,
			}},
			AIChecks: []string{"conflict"},
		},
		{
			ID: "release", Label: "Release",
			Description: "Ship it, and learn from it.",
			Artifacts: []ArtifactKind{
				{Kind: "linear_issue", Label: "Linear issues", Description: "Linked or imported Linear issues.", Multiple: true, External: true},
				{Kind: "github_pr", Label: "GitHub PRs", Description: "Pull requests implementing the solution.", Multiple: true, External: true},
				{Kind: "learnings", Label: "Learnings", Description: "What we learned while implementing.", Template: "learnings"},
				{Kind: "release_notes", Label: "Release notes", Template: "release_notes"},
				{Kind: "launch_notes", Label: "Launch notes", Template: "launch_notes"},
				{Kind: "user_interview", Label: "Post-launch interviews", Multiple: true, Template: "user_interview"},
				{Kind: "csat", Label: "CSAT", Description: "Customer satisfaction survey and results.", Template: "csat"},
			},
			Gates: []Gate{{
				ID: "release", Label: "Release sign-off", Question: "Shipped and documented.",
				ApproverRoles: []string{RoleProductOwner, RoleEngineeringOwner}, Policy: PolicyAny, OnChange: OnChangeFlag, AllowOverride: true,
			}},
		},
	}}
}
