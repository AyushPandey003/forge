# Forge — Product feature guide

> Status: local prototype. This guide distinguishes implemented features from planned capabilities. Deployment is out of scope for the current build.

## 1. What is Forge?

Forge is a shared workspace for product, design and engineering teams to take a problem through discovery, specification, implementation planning and release.

People and AI agents work on the same versioned artifacts. Review happens through anchored comments and explicit approval gates, rather than scattered chat messages and untraceable sign-offs.

**Default delivery flow**

```text
Problem
  └─ Problem approval
       ↓
PRD + designs + prototypes
  └─ Solution approval + Implementation approval + UX approval
       ↓
Implementation doc: HLD + LLD + non-functional requirements
  └─ Implementation approval
       ↓
Release: issues, PR links, notes, learnings and feedback
  └─ Release sign-off
```

The flow is a configurable template, not a fixed process imposed on every team.

## 2. Main concepts

| Concept | Meaning |
|---|---|
| Solution | A unit of product work containing its workflow, people and artifacts. Gets a key such as `SOL-12`. |
| Stage | A step in a solution's workflow, such as Problem or PRD. |
| Artifact | A document or external link attached to a stage. |
| Artifact slot | A configured kind of artifact a stage expects, optionally required or repeatable. |
| Gate | A sign-off that must pass before a stage is ready to advance. |
| Approval | A person's decision, pinned to the document versions present when they reviewed. |
| Workflow template | A reusable definition of stages, artifacts and gate policies. |
| Timeline | The solution's activity history: artifacts, membership, decisions and stage changes. |

## 3. Features available now

### 3.1 Solutions workspace

**Purpose:** Give every piece of work a single shared home.

- Create a solution with a title and optional summary.
- Generate a human-readable solution key automatically.
- Browse Active, Shipped, Archived or All solutions.
- See current stage and stage-based progress.
- Edit a solution's title and summary.
- Archive and restore solutions.
- Keep solution documents together in a dedicated folder.

**Where:** `/app/solutions`

### 3.2 Solution board

**Purpose:** Make the delivery state and next action visible.

Each board includes:

- A clickable stage stepper.
- Artifact slots for the selected stage.
- Required-artifact warnings.
- Approval gate cards and reviewer decisions.
- Members and their roles.
- An activity timeline with All and Decisions views.
- An Advance action, enabled when required artifacts exist and all gates pass.
- A Mark shipped action at the end of the workflow.
- A manual Move solution here action for owners, with a reason recorded in the timeline.

The progress indicator reflects workflow stages, not issue completion or engineering effort.

**Where:** `/app/solutions/:key`

### 3.3 Team roles

A person can hold multiple roles within one solution.

| Role | Intended responsibility |
|---|---|
| Product owner | Own the problem and PRD. |
| Engineering owner | Own implementation planning and delivery. |
| UX owner | Own design and prototype artifacts. |
| Implementer | Build the solution. |
| Solution approver | Review the problem and proposed solution. |
| Implementation approver | Review feasibility and the implementation plan. |
| UX approver | Review the user experience. |
| Solution consumer | Follow progress and provide feedback. |

- The solution creator becomes its product owner.
- Owners can add members by email and assign roles.
- Owners can remove role assignments.
- Owners and implementers receive editor access through the solution folder; other members receive commenter access.
- A member can be added before signing in; an email-based placeholder is created. This does not send an email invitation.

**Current permissions caveat:** Role removal does not yet reconcile previously granted document/folder access. This needs hardening before access-sensitive use.

### 3.4 Artifact management

**Purpose:** Keep evidence, specifications and outputs attached to the work they describe.

For document slots:

- Write a new document from a starter template.
- Attach an existing document using its URL or document ID.
- Open the document for review.
- Open the Markdown editor where applicable.
- See the current document version.

For link slots:

- Add a title and external URL.
- Open the link in a new tab.

Owners can remove artifact associations. Removing an artifact from a solution is not the same as deleting its underlying document.

**Important:** Linear and GitHub slots currently store links only. Adding one does not create issues, fetch statuses, sync data or call either service.

### 3.5 Document authoring

**Purpose:** Let people write directly in Forge, not just review agent output.

- Rich-text authoring using Katalyst's Markdown editor.
- Formatting controls for headings, emphasis, lists, tables, links and more.
- Markdown source retained inside the published HTML for editing round-trips.
- Publish changes as a new immutable document version.
- Optional change summary with each publication.
- Unsaved-change indicator and browser unload warning.
- Ctrl/Cmd+S publication shortcut.
- Stale-base protection: publication conflicts if another version was published first.
- Solution and stage context displayed in the editor.

**Where:** `/app/edit/:documentId`

**Limits:** This is not simultaneous Google Docs-style editing. Concurrent edits are handled with version-conflict protection, not live text merging. The editor applies to Forge-authored Markdown documents; arbitrary HTML and Stitch designs are not editable through this page.

### 3.6 Templates

Starter documents are available for:

- Problem statements.
- PRDs, including user stories and acceptance criteria.
- Implementation docs, including HLD, LLD and non-functional requirements.
- User interview notes.
- User interview plans.
- Incidents / IRCA.
- Learnings.
- Release notes.
- Launch notes.
- CSAT plans/results.

Templates are starter content, not automatic validators. Stories, acceptance criteria, interview plans and CSAT results are currently document content, not separate structured records or automated surveys.

Mermaid code blocks in published Markdown documents can render diagrams.

### 3.7 Document review and collaboration

Inherited from the SlashDocs foundation:

- Rich HTML documents rendered in a separate-origin sandbox.
- Comments anchored to selected document text.
- Threaded replies and comment reactions.
- Open and resolved review threads.
- Document version history.
- Share controls and document access roles.
- Live collaboration signals supported by the underlying document viewer.

Forge adds solution/stage context and Edit actions to document pages.

The application interface follows Katalyst's light/dark semantic theme. Uploaded and generated document content can have its own styling.

### 3.8 Version-pinned approval gates

**Purpose:** Record exactly what was approved and by whom.

An eligible approver can:

- Approve.
- Request changes, with an explanation.
- Reject, with an explanation.
- Re-approve after an artifact changes.

A decision records the document versions available in that stage at review time. The latest decision from each person is used to evaluate the gate.

**Gate policies**

| Policy | Behaviour |
|---|---|
| Any | One eligible approval is enough. |
| All | Every eligible approver must approve. |
| N | At least the configured number of approvers must approve. |

A current counted rejection or request for changes blocks the gate, even if enough other people approved. An allowed owner override can pass the gate despite that block.

**When a document changes after approval**

| Setting | Behaviour |
|---|---|
| Flag | Keep counting the approval, but mark it as changed since review. |
| Revoke | Stop counting the stale decision until reviewed again. |

**Owner overrides**

- Available only when the gate permits them.
- Require a reason.
- Appear explicitly as overrides on the gate and in the timeline.

**Current limits:** Staleness tracks document version changes and newly added document artifacts. External link changes and removed artifacts are not yet comprehensively tracked. Approval checks artifact presence, not whether its content is complete. Owners can manually move a solution to another stage; that bypass is recorded but not currently disabled by a workflow setting.

### 3.9 Approval inbox

**Purpose:** Surface decisions that need your attention.

The Solutions page shows a Waiting on you section containing:

- Current-stage gates waiting for your decision.
- Your previous decisions where artifacts changed since review.
- Links directly into the relevant solution and stage.

This is an in-app view, not an email notification system.

### 3.10 Configurable workflows

**Purpose:** Let teams shape the process rather than adapt to a rigid tool.

The workflow editor supports:

- Workflow names and descriptions.
- Duplicating a workflow.
- Adding, removing and reordering stages.
- Editing stage labels, IDs and descriptions.
- Adding/removing artifact slots.
- Required, repeatable and external-link slot settings.
- Adding/removing approval gates.
- Editing approval questions and eligible roles.
- Any, All and N approval policies.
- Flag or revoke behaviour after document changes.
- Allowing/disallowing owner overrides.

Each solution snapshots its workflow when created. Editing a template does not silently change existing solutions.

**Where:** `/app/settings/workflows`

**Current limits:** Workflow editing is available to signed-in users; a workspace-level admin permission model is not yet implemented. The API supports selecting a workflow when creating a solution, but the creation dialog currently uses the default template. Existing solution workflow snapshots are not editable through the UI.

### 3.11 Design studio: Stitch + Katalyst

**Purpose:** Turn a spec or prompt into a reviewable UI design.

- Describe a screen in natural language.
- Select a document as the source spec.
- Choose Desktop, Tablet or Mobile.
- Supply an optional title.
- Use starter prompts.
- Generate through Google Stitch using the configured Katalyst design system.
- Track generation progress.
- Publish the generated screen as a normal versioned document.
- Generate from a solution board and attach the result as a design artifact.
- Use the solution's PRD as context when generating from a solution.

**Where:** `/app/design`

Katalyst guides generation; it does not guarantee the output uses actual Katalyst React components. Stitch output is currently HTML, and it may rely on external fonts or scripts.

**Current limit:** If Stitch returns several screens, Forge currently publishes only the first. Screen collections and variant comparison are planned.

### 3.12 Comment-to-design revision loop

On a Forge design document:

1. Reviewers leave comments on the design.
2. An editor opens Revise with Stitch.
3. Open comments and an optional instruction become revision feedback.
4. Stitch edits the screen.
5. Forge publishes the result as a new document version.
6. Forge replies to the addressed threads and marks them resolved.

The previous design remains in version history.

**Control caveat:** Automatic resolution currently means the feedback was submitted in a completed revision, not that Forge independently verified the design satisfied every request. Reviewers should check the new version and reopen threads where necessary.

### 3.13 Agent access through MCP

In addition to SlashDocs' document tools, Forge exposes:

| Tool | Purpose |
|---|---|
| `forge_list_solutions` | List solutions accessible to the caller. |
| `forge_get_solution` | Read the board, artifacts, roles and gate states. |
| `forge_create_solution` | Create a solution. |
| `forge_add_artifact` | Create a document, attach an existing one or add a link. |
| `forge_read_document_markdown` | Read editable Markdown source. |
| `forge_update_document_markdown` | Publish a new Markdown document version. |
| `forge_decide_gate` | Record an eligible approval decision. |
| `forge_inbox` | Read outstanding review work. |

Calls use the authenticated principal. The human approval-role model currently assumes user identities; dedicated bot membership and approval delegation still need work.

### 3.14 Live workflow updates

Solutions, boards, timelines and approval inboxes automatically refresh after relevant changes, including linked document publications. This uses authenticated SSE and transactionally stored per-user revisions, with a two-second polling interval and snapshot refresh on reconnect. It does not replace unsaved editor content or provide simultaneous text editing.

See [Realtime architecture and limits](realtime.md). Stitch job durability/progress and a workspace notification center remain separate work.

### 3.15 Accounts, invites and password resets (local)

**Purpose:** Let real teammates join with their own identity, controlled by admins.

**Hierarchy:** Workspace → Teams → Solutions. Workspace roles (`owner`, `admin`, `member`, `guest`) control access; solution roles (approvers, owners) control sign-off. They are deliberately separate.

**Joining**
1. An admin opens **Account menu → Team & invites** (`/app/settings/team`).
2. Picks a workspace role and optional team, then **Generate invite link**.
3. The link is shown once. Share it privately (e.g. open it in an incognito window to test).
4. The invitee opens it and sets **full name, username and password** (min 12 characters).
5. They are signed in and land on Solutions.

**Signing in:** `/login` → username + password. The Dex SSO button remains for the bootstrap admin.

**Password reset (admin-issued)**
1. On the Team page, an admin clicks **Reset link** next to a member.
2. The member opens the link and sets a new password.
3. Their existing sessions are signed out; the old password stops working.

**Rules**
| | Invite link | Reset link |
|---|---|---|
| Lifetime | 7 days | 1 hour |
| Uses | Single use | Single use |
| Revocable | Yes | Yes; a new reset supersedes older ones |
| Who can issue | Owners/admins; only owners can invite admins | Owners/admins; admins can't reset owners or other admins |

**Security notes**
- Tokens live in the URL `#fragment`, so they never reach server logs or `Referer` headers, and are stripped from the address bar on open.
- Only token hashes are stored. Passwords use bcrypt.
- Login/register/reset are rate-limited per IP (in-memory, single node).
- Local accounts use a non-routable `username@local.invalid` email; there is no email verification or delivery.
- The first owner is the email in `FORGE_LOCAL_ADMIN_EMAIL` (dev: `admin@example.com`). There is no public signup.
- Enabled with `FORGE_LOCAL_AUTH=true` (on in the dev compose file).

**Not yet:** self-service "forgot password", email delivery, team-scoped solution visibility, removing members/changing workspace roles from the UI.

## 4. Default stage requirements

### Problem

**Artifacts**
- Problem statement — required.
- User interviews — optional, repeatable.
- IRCA / incidents — optional, repeatable.

**Gate**
- Solution approver: “Yes, this is the problem.”

### PRD

**Artifacts**
- PRD — required.
- UX designs — optional, repeatable.
- User interview plan — optional.
- Prototype document — optional, repeatable.
- Prototype link — optional, repeatable.

**Gates**
- Solution approver: “This solution makes sense.”
- Implementation approver: “This is buildable as specified.”
- UX approver: “The experience is right.”

### Implementation

**Artifact**
- Implementation doc — required, with HLD/LLD/NFR starter sections.

**Gate**
- Implementation approver: “Yes.”

### Release

**Artifacts**
- Linear issue links.
- GitHub PR links.
- Learnings.
- Release notes.
- Launch notes.
- Post-launch interviews.
- CSAT document.

These slots are optional in the default template.

**Gate**
- Product or engineering owner: “Shipped and documented.”

All default gates use Any approval, Flag on change, and allow an owner override.

## 5. Example walkthrough

1. Create **“Onboarding new engineers takes 3 weeks.”**
2. Write the problem statement and attach interview evidence.
3. Add the product, engineering and UX owners and required approvers.
4. Obtain problem approval and advance to PRD.
5. Write stories and acceptance criteria.
6. Generate UX designs from the PRD and collect review comments.
7. Revise the designs with Stitch.
8. Obtain solution, implementation and UX approvals.
9. Advance to Implementation and write HLD, LLD and NFRs.
10. Obtain implementation approval and advance to Release.
11. Add issue and PR links manually, capture learnings and publish notes.
12. Obtain release sign-off and mark the solution shipped.

If a reviewed document changes, the board flags or revokes the affected decision according to its gate settings.

## 6. Planned features — not implemented yet

### Discovery and requirements
- Structured user stories and acceptance-criteria records.
- Interview scheduling and transcript ingestion.
- AI extraction of insights and supporting quotes.
- Acceptance-criteria coverage across designs and implementation.

### AI review
- PRD-versus-problem conflict checks.
- Implementation-versus-PRD checks.
- Design-versus-acceptance-criteria checks.
- Anchored findings from an AI reviewer.
- Drafting the next artifact from approved upstream content.
- Review-thread summaries.
- Katalyst compliance review and scoring.

These need an LLM provider configuration separate from Stitch.

### Design and prototyping
- Multiple generated screens per request.
- Variants, voting and side-by-side comparisons.
- Desktop/mobile design sets.
- Targeted region editing.
- HTML-to-React conversion using real Katalyst components.
- Runnable React prototype workspaces.

### Release integrations
- Linear issue creation/import and status sync.
- GitHub PR metadata and story linkage.
- AI-generated release notes from linked delivery records.
- Automated CSAT surveys and analysis.

**Permission boundary:** No actual Linear or GitHub integration tests, issue creation, PR creation or live sync calls without explicit user permission. Storing a manually supplied link is not integration testing.

### Reliability and governance
- Durable design-job execution and restart recovery.
- Stronger workspace/admin permissions.
- Reconciliation of document access after member changes.
- Transactional gate transitions and approval snapshots.
- Complete artifact/link-change invalidation.
- Notification delivery for approval requests and mentions.
- Delivery metrics and audit export.

The database contains a `forge_jobs` table, but the current Stitch job runner still uses in-memory jobs. Restarting the server can lose tracking of an unfinished job; published documents remain stored.

## 7. Local access

- **Workspace:** http://localhost:8080/app/solutions
- **Workflow editor:** http://localhost:8080/app/settings/workflows
- **Design studio:** http://localhost:8080/app/design
- **Team & invites:** http://localhost:8080/app/settings/team
- **Bootstrap admin login (Dex):** `admin@example.com` / `password`
- **Other users:** join through an admin-generated invite link with their own username and password.

This is a local setup, not a production authentication configuration.

From the repository root:

```bash
scripts/restart.sh          # build frontend and restart the API
scripts/restart.sh server   # rebuild/restart only the API
```

Local services: Go API, Postgres, S3-compatible blob storage and Dex. Redis is not required. Local development currently uses Docker Postgres, not the Neon URL in `.env.local`.

## 8. Implementation references

- [Development roadmap](forge-plan.md)
- [Underlying document notification support](comment-notifications.md)
- [Underlying GitHub document integration](github-integration.md) — distinct from the planned Forge release integration.
- Domain: `api/internal/forge/`
- Solution API: `api/internal/server/forge_solutions.go`
- Design API: `api/internal/server/forge_design.go`
- MCP tools: `api/internal/server/mcp_forge.go`
- UI: `frontend/src/pages/solutions.tsx`, `solution-board.tsx`, `doc-editor.tsx`, `workflows.tsx`, `design-studio.tsx`
