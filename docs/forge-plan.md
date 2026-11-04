# Forge plan

Forge = SlashDocs (documents, versions, anchored comments, MCP) + Katalyst
(design system) + Google Stitch (design generation), organised around a
**Solution** that moves through a configurable delivery workflow:

```
Problem ──✅ "this is the problem"──▶ PRD ──✅ solution · implementation · UX──▶
Implementation doc ──✅ "Yes"──▶ Release (Linear, GitHub, notes, CSAT)
```

## Principles

1. **Solution is the unit of work.** Artifacts are ordinary documents, so
   versions, comments, sharing and agent access work everywhere.
2. **Approvals are pinned to versions.** A decision records the artifact
   versions it saw. When an artifact changes, the decision is flagged (or
   revoked — configurable per gate).
3. **Workflow is data.** Stages, artifact slots, approver roles, policy
   (any / all / N), on-change behaviour and overrides are editable at
   `/app/settings/workflows`. Solutions snapshot the workflow at creation.
4. **AI comments like a reviewer.** Checks post anchored comments; nothing is
   rewritten silently.
5. **Agents are first-class.** Every action is also an MCP tool.

## Status

### ✅ Phase 0 — Foundations
- Migration `00034_forge_solutions`: workflows, solutions, members/roles,
  artifacts, version-pinned approvals, audit events, durable jobs table.
- `internal/forge`: workflow model + validation, store, gate engine
  (unit-tested), Markdown templates and rendering.
- Markdown authoring with Katalyst `MarkdownEditor` (`/app/edit/:id`); each
  save is an immutable version; source embedded for round-trip.
- Katalyst theme everywhere; Forge branding; dev runs in Docker (Smart App
  Control safe) — `scripts/restart.sh`.

### ✅ Phase 1 — Solution pipeline
- Solutions list with "Waiting on you" inbox.
- Board: stage stepper, artifact slots (write / attach / link / generate),
  gate cards (approve / request changes / reject / override with reason,
  stale flags), members & roles, timeline, advance / move-to-stage.
- Workflow editor.
- Documents show their solution + stage; Markdown docs get **Edit**.
- Design studio attaches generated designs to a solution (uses its PRD as
  the spec when no prompt is given).
- MCP tools: `forge_list_solutions`, `forge_get_solution`,
  `forge_create_solution`, `forge_add_artifact`,
  `forge_read_document_markdown`, `forge_update_document_markdown`,
  `forge_decide_gate`, `forge_inbox`.

### ⏭ Phase 2 — Problem & PRD depth
- Structured user stories / acceptance criteria (parsed from the PRD
  Markdown into KV, shown as a checklist; coverage tracking).
- Interview scheduling + transcript → insights extraction.
- Stitch variants (generate 3, vote), side-by-side compare, desktop+mobile.
- Prototype (code): Stitch HTML → React with Katalyst components using
  Katalyst contracts.

### ⏭ Phase 3 — AI checks (needs an LLM key decision)
- Conflict check at gates (PRD vs problem, impl vs PRD, design vs ACs),
  posted as anchored bot comments.
- Coverage report, Katalyst design review, draft-next-stage, thread summaries.

### ⏭ Phase 4 — Implementation doc
- HLD/LLD/NFR template (done), Mermaid rendering (done), conflict check.

### ⏭ Phase 5 — Release (live calls only with explicit permission)
- Linear: create issues from stories / import; status sync.
- GitHub: link PRs to stories.
- Release + launch notes drafted from PRD + issues + PRs; learnings; CSAT.

### ⏭ Phase 6 — Control & insight
- Per-role permissions matrix, notifications (in-app + Slack), metrics
  (cycle time per stage, approval latency, rework), cross-solution search,
  audit export.

## Open decisions
- LLM provider for AI checks (Gemini on personal project / Claude / OpenAI).
- Default gate policy for new workflows (currently: any approver, flag on change).
- Linear workspace/team and GitHub OAuth app — **no live calls without permission**.

## Dev notes
- `scripts/restart.sh` rebuilds the frontend and restarts the API container;
  `scripts/restart.sh server` restarts the API only.
- Go tests run in Docker (Windows Smart App Control blocks test binaries):
  ```
  MSYS_NO_PATHCONV=1 docker run --rm --network app_default -v "D:/prod-tool/app:/src" \
    -v "C:/Users/Ayush/go/pkg/mod:/go/pkg/mod:ro" -v app_gocache:/root/.cache/go-build \
    -e GOFLAGS=-mod=mod -e GOPROXY=off -e GOTOOLCHAIN=local \
    -e SLASHDOCS_TEST_DATABASE_URL="postgres://slashdocs:slashdocs@postgres:5432/slashdocs?sslmode=disable" \
    -w /src golang:1.27 go test ./api/... ./internal/...
  ```
- Stitch auth in the container: `.secrets/stitch-personal.json` (personal
  gcloud refresh token; gitignored), quota project `forge-stitch-261104`.
