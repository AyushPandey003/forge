package server

// Forge MCP tools: let agents (Claude Code, Cursor, Codex…) drive solutions
// the same way people do — read the board, write artifacts, request and
// record approvals. Every call runs as the authenticated MCP principal.

import (
	"context"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func registerMCPForgeTools(server *mcpserver.MCPServer, api mcpAPIAdapter) {
	if api.handlers.deps.forge == nil {
		return
	}
	h := api.handlers
	sidParam := mcp.WithString("solution", mcp.Required(), mcp.Description("Solution id or key, e.g. SOL-12."))

	server.AddTool(mcp.NewTool("forge_list_solutions", readOnlyOptions(
		mcp.WithDescription("List Forge solutions you own or are a member of, with their current stage."),
		mcp.WithString("status", mcp.Description("active | shipped | archived (default: all)")),
	)...), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		target := "/v1/forge/solutions"
		if s := r.GetString("status", ""); s != "" {
			target += "?status=" + s
		}
		return api.get(ctx, target, nil, h.forgeListSolutions)
	})

	server.AddTool(mcp.NewTool("forge_get_solution", readOnlyOptions(
		mcp.WithDescription("Get a solution's full board: stages, required artifacts (with document ids), approval gates and their state, members and roles. Use this before writing or reviewing."),
		sidParam,
	)...), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sid, bad := mcpRequired(r, "solution")
		if bad != nil {
			return bad, nil
		}
		return api.get(ctx, "/v1/forge/solutions/"+sid, mcpParams("sid", sid), h.forgeBoard)
	})

	server.AddTool(mcp.NewTool("forge_create_solution", mutationOptions(false, false,
		mcp.WithDescription("Create a solution (starts at the first stage, usually Problem). You become its product owner."),
		mcp.WithString("title", mcp.Required()),
		mcp.WithString("summary"),
	)...), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, bad := mcpRequired(r, "title")
		if bad != nil {
			return bad, nil
		}
		return api.json(ctx, http.MethodPost, "/v1/forge/solutions", map[string]any{"title": title, "summary": r.GetString("summary", "")}, nil, h.forgeCreateSolution)
	})

	server.AddTool(mcp.NewTool("forge_add_artifact", mutationOptions(false, false,
		mcp.WithDescription("Add an artifact to a solution stage. For document kinds pass `markdown` to create a new Forge document (omit it to start from the stage template), or `document_id` to attach an existing document. For link kinds (prototype_link, linear_issue, github_pr) pass `url`."),
		sidParam,
		mcp.WithString("stage", mcp.Required(), mcp.Description("Stage id, e.g. problem, prd, implementation, release.")),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Artifact kind from the board, e.g. problem_statement, prd, impl_doc.")),
		mcp.WithString("title"),
		mcp.WithString("markdown", mcp.Description("Markdown body for a new document.")),
		mcp.WithString("document_id"),
		mcp.WithString("url"),
	)...), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sid, bad := mcpRequired(r, "solution")
		if bad != nil {
			return bad, nil
		}
		body := map[string]any{
			"stage": r.GetString("stage", ""), "kind": r.GetString("kind", ""), "title": r.GetString("title", ""),
			"document_id": r.GetString("document_id", ""), "url": r.GetString("url", ""), "markdown": r.GetString("markdown", ""),
		}
		if body["document_id"] == "" && body["url"] == "" {
			body["create"] = true
		}
		return api.json(ctx, http.MethodPost, "/v1/forge/solutions/"+sid+"/artifacts", body, mcpParams("sid", sid), h.forgeAddArtifact)
	})

	server.AddTool(mcp.NewTool("forge_read_document_markdown", readOnlyOptions(
		mcp.WithDescription("Read the Markdown source of a Forge-authored document (problem statement, PRD, implementation doc…)."),
		mcp.WithString("document_id", mcp.Required()),
	)...), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, bad := mcpRequired(r, "document_id")
		if bad != nil {
			return bad, nil
		}
		return api.get(ctx, "/v1/forge/documents/"+id+"/source", mcpParams("id", id), h.forgeDocSource)
	})

	server.AddTool(mcp.NewTool("forge_update_document_markdown", mutationOptions(false, false,
		mcp.WithDescription("Publish a new version of a Forge-authored document from Markdown. Pass base_version_id from forge_read_document_markdown; a stale base returns a conflict. Approvals pinned to the old version are flagged as changed."),
		mcp.WithString("document_id", mcp.Required()),
		mcp.WithString("markdown", mcp.Required()),
		mcp.WithString("base_version_id", mcp.Required()),
		mcp.WithString("change_summary"),
	)...), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, bad := mcpRequired(r, "document_id")
		if bad != nil {
			return bad, nil
		}
		return api.json(ctx, http.MethodPost, "/v1/forge/documents/"+id+"/source", map[string]any{
			"markdown": r.GetString("markdown", ""), "base_version_id": r.GetString("base_version_id", ""), "change_summary": r.GetString("change_summary", ""),
		}, mcpParams("id", id), h.forgeSaveDocSource)
	})

	server.AddTool(mcp.NewTool("forge_decide_gate", mutationOptions(false, false,
		mcp.WithDescription("Record an approval decision on a stage gate as the authenticated principal. Only principals holding an approver role for the gate can decide. changes_requested and rejected need a comment."),
		sidParam,
		mcp.WithString("stage", mcp.Required()),
		mcp.WithString("gate", mcp.Required()),
		mcp.WithString("decision", mcp.Required(), mcp.Enum("approved", "changes_requested", "rejected")),
		mcp.WithString("comment"),
	)...), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sid, bad := mcpRequired(r, "solution")
		if bad != nil {
			return bad, nil
		}
		stage, gate := r.GetString("stage", ""), r.GetString("gate", "")
		return api.json(ctx, http.MethodPost, "/v1/forge/solutions/"+sid+"/gates/"+stage+"/"+gate,
			map[string]any{"decision": r.GetString("decision", ""), "comment": r.GetString("comment", "")},
			mcpParams("sid", sid, "stage", stage, "gate", gate), h.forgeDecide)
	})

	server.AddTool(mcp.NewTool("forge_inbox", readOnlyOptions(
		mcp.WithDescription("Gates on current stages that are waiting on your decision (or that changed since you decided)."),
	)...), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return api.get(ctx, "/v1/forge/inbox", nil, h.forgeInbox)
	})
}
