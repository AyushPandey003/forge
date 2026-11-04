// Forge solutions API: typed client for the delivery workflow.

export type User = { id: string; email: string; name: string; picture_url?: string };

export type ArtifactKind = {
  kind: string;
  label: string;
  description?: string;
  required: boolean;
  multiple: boolean;
  template?: string;
  external?: boolean;
};

export type Gate = {
  id: string;
  label: string;
  question: string;
  approver_roles: string[];
  policy: "any" | "all" | "n";
  min_approvals?: number;
  on_change: "flag" | "revoke";
  allow_override: boolean;
};

export type Stage = {
  id: string;
  label: string;
  description?: string;
  artifacts: ArtifactKind[];
  gates: Gate[];
  ai_checks?: string[];
};

export type Workflow = { stages: Stage[] };

export type WorkflowRecord = {
  id: string;
  name: string;
  description: string;
  definition: Workflow;
  is_default: boolean;
  updated_at: string;
};

export type Solution = {
  id: string;
  key: string;
  title: string;
  summary: string;
  status: "active" | "shipped" | "archived";
  workflow_id: string;
  workflow: Workflow;
  current_stage: string;
  folder_id?: string;
  created_by: User;
  created_at: string;
  updated_at: string;
};

export type SolutionListItem = Solution & {
  stage_label: string;
  stage_index: number;
  stage_count: number;
  roles?: string[];
};

export type Artifact = {
  id: string;
  solution_id: string;
  stage: string;
  kind: string;
  document_id?: string;
  url?: string;
  title: string;
  meta: Record<string, unknown>;
  created_by: User;
  created_at: string;
  current_version_id?: string;
  version_number?: number;
  doc_updated_at?: string;
};

export type Decision = {
  id: string;
  stage: string;
  gate: string;
  user: User;
  decision: "approved" | "changes_requested" | "rejected" | "withdrawn";
  comment: string;
  pinned: Record<string, string>;
  override: boolean;
  created_at: string;
  stale: boolean;
  stale_reasons?: string[];
  counts: boolean;
};

export type GateStatus = Gate & {
  state: "pending" | "approved" | "changes_requested" | "rejected" | "overridden";
  decisions: Decision[] | null;
  approvers: User[] | null;
  approved: number;
  required: number;
  can_decide: boolean;
};

export type Slot = ArtifactKind & { items: Artifact[] | null };

export type StageStatus = Stage & {
  index: number;
  state: "done" | "current" | "upcoming";
  slots: Slot[];
  gate_status: GateStatus[] | null;
  missing: string[] | null;
  ready: boolean;
};

export type Member = { user: User; role: string };

export type Board = {
  solution: Solution;
  members: Member[] | null;
  stages: StageStatus[];
  viewer_roles: string[] | null;
  can_manage: boolean;
  can_advance: boolean;
  progress: number;
};

export type RoleInfo = { id: string; label: string; description: string };

export type ForgeEvent = {
  id: number;
  actor: User;
  kind: string;
  data: Record<string, unknown>;
  created_at: string;
};

export type InboxItem = {
  solution_id: string;
  solution_key: string;
  title: string;
  stage: string;
  stage_label: string;
  gate: string;
  gate_label: string;
  question: string;
  reason: "awaiting" | "stale";
};

export type DocContext = {
  artifact: Artifact | null;
  solution?: { id: string; key: string; title: string };
  stage_label?: string;
};

async function call<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...init,
    headers: { "Content-Type": "application/json", ...(init.headers || {}) },
  });
  if (res.status === 204) return undefined as T;
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body?.error?.message || res.statusText);
  return body as T;
}

const json = (method: string, body?: unknown): RequestInit => ({
  method,
  body: body === undefined ? undefined : JSON.stringify(body),
});
const sid = (id: string) => `/v1/forge/solutions/${encodeURIComponent(id)}`;

export const solutionsApi = {
  meta: () => call<{ roles: RoleInfo[] }>("/v1/forge/meta"),
  workflows: () => call<{ items: WorkflowRecord[] }>("/v1/forge/workflows"),
  saveWorkflow: (w: Partial<WorkflowRecord>) =>
    w.id
      ? call<WorkflowRecord>(`/v1/forge/workflows/${w.id}`, json("PUT", w))
      : call<WorkflowRecord>("/v1/forge/workflows", json("POST", w)),
  list: (status?: string) =>
    call<{ items: SolutionListItem[] }>(
      `/v1/forge/solutions${status ? `?status=${status}` : ""}`,
    ),
  create: (input: { title: string; summary?: string; workflow_id?: string }) =>
    call<Solution>("/v1/forge/solutions", json("POST", input)),
  board: (id: string) => call<Board>(sid(id)),
  patch: (id: string, patch: Partial<Pick<Solution, "title" | "summary" | "status">>) =>
    call<Solution>(sid(id), json("PATCH", patch)),
  events: (id: string) => call<{ items: ForgeEvent[] }>(`${sid(id)}/events`),
  addMember: (id: string, email: string, role: string) =>
    call(`${sid(id)}/members`, json("POST", { email, role })),
  removeMember: (id: string, userId: string, role: string) =>
    call(`${sid(id)}/members/${encodeURIComponent(userId)}/${role}`, json("DELETE")),
  addArtifact: (
    id: string,
    input: {
      stage: string;
      kind: string;
      title?: string;
      create?: boolean;
      document_id?: string;
      url?: string;
      markdown?: string;
    },
  ) => call<Artifact>(`${sid(id)}/artifacts`, json("POST", input)),
  removeArtifact: (id: string, artifactId: string) =>
    call(`${sid(id)}/artifacts/${artifactId}`, json("DELETE")),
  decide: (
    id: string,
    stage: string,
    gate: string,
    input: { decision: string; comment?: string; override?: boolean },
  ) => call<Board>(`${sid(id)}/gates/${stage}/${gate}`, json("POST", input)),
  advance: (id: string) => call<Board>(`${sid(id)}/advance`, json("POST")),
  setStage: (id: string, stage: string, reason: string) =>
    call<Board>(`${sid(id)}/stage`, json("POST", { stage, reason })),
  inbox: () => call<{ items: InboxItem[] }>("/v1/forge/inbox"),
  docSource: (docId: string) =>
    call<{ editable: boolean; markdown: string; version_id: string; title: string }>(
      `/v1/forge/documents/${docId}/source`,
    ),
  saveDocSource: (
    docId: string,
    input: { markdown: string; base_version_id: string; change_summary?: string },
  ) =>
    call<{ id: string; number: number }>(
      `/v1/forge/documents/${docId}/source`,
      json("POST", input),
    ),
  docContext: (docId: string) => call<DocContext>(`/v1/forge/documents/${docId}/context`),
};

export const solutionKeys = {
  all: ["forge", "solutions"] as const,
  list: (status?: string) => ["forge", "solutions", "list", status || "all"] as const,
  board: (id: string) => ["forge", "solutions", "board", id] as const,
  events: (id: string) => ["forge", "solutions", "events", id] as const,
  inbox: ["forge", "inbox"] as const,
  meta: ["forge", "meta"] as const,
  workflows: ["forge", "workflows"] as const,
  docContext: (docId: string) => ["forge", "doc-context", docId] as const,
  docSource: (docId: string) => ["forge", "doc-source", docId] as const,
};

export const ROLE_LABEL: Record<string, string> = {
  product_owner: "Product owner",
  engineering_owner: "Engineering owner",
  ux_owner: "UX owner",
  implementer: "Implementer",
  solution_approver: "Solution approver",
  implementation_approver: "Implementation approver",
  ux_approver: "UX approver",
  consumer: "Consumer",
};

export function userLabel(u?: User) {
  return u?.name || u?.email || "Someone";
}
