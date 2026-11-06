// Local accounts: invite links, username/password sign-in, admin resets.
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
const post = (body: unknown): RequestInit => ({ method: "POST", body: JSON.stringify(body) });

export type AccountMember = {
  id: string;
  email: string;
  name: string;
  username: string;
  role: "owner" | "admin" | "member" | "guest";
};
export type Team = { id: string; name: string };
export type AccountLink = {
  id: string;
  kind: "invite" | "reset";
  role: string;
  team_id: string;
  user_id: string;
  expires_at: string;
  used: boolean;
  revoked: boolean;
};

export const accountsApi = {
  config: () => call<{ enabled: boolean }>("/v1/auth/local/config"),
  login: (username: string, password: string) =>
    call("/v1/auth/local/login", post({ username, password })),
  inspect: (token: string) =>
    call<{ kind: "invite" | "reset"; role: string; team_id: string; expires_at: string; workspace: string }>(
      "/v1/auth/local/link",
      post({ token }),
    ),
  register: (input: { token: string; name: string; username: string; password: string; confirm_password: string }) =>
    call("/v1/auth/local/register", post(input)),
  reset: (token: string, password: string, confirm_password: string) => call("/v1/auth/local/reset", post({ token, password, confirm_password })),
  account: () => call<{ enabled: boolean; role?: string; workspace?: string }>("/v1/forge/account"),
  admin: () =>
    call<{ members: AccountMember[]; teams: Team[]; links: AccountLink[] }>("/v1/forge/admin/accounts"),
  createTeam: (name: string) => call<Team>("/v1/forge/admin/teams", post({ name })),
  createInvite: (role: string, team_id?: string) =>
    call<{ url: string; link: AccountLink }>("/v1/forge/admin/invites", post({ role, team_id })),
  createReset: (user_id: string) =>
    call<{ url: string; link: AccountLink }>("/v1/forge/admin/resets", post({ user_id })),
  revoke: (id: string) => call(`/v1/forge/admin/links/${id}`, { method: "DELETE" }),
};

// Tokens live in the URL fragment so they never reach server logs.
export function tokenFromHash() {
  return new URLSearchParams(window.location.hash.replace(/^#/, "")).get("token") || "";
}
