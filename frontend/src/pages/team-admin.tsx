// Workspace admin: members, teams, invite links and password-reset links.
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Copy, KeyRound, Link2, Plus, Trash2 } from "lucide-react";
import { Button } from "@kalvi-education/katalyst/components/button";
import { Badge } from "@kalvi-education/katalyst/components/badge";
import { Input } from "@kalvi-education/katalyst/components/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@kalvi-education/katalyst/components/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@kalvi-education/katalyst/components/select";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@kalvi-education/katalyst/components/dialog";
import { Page } from "@/components/layout/page";
import { useTitle } from "@/lib/use-title";
import { formatTimestamp } from "@/lib/format-date";
import { accountsApi } from "@/lib/accounts";

const NO_TEAM = "__none__";

export function TeamAdminPage() {
  useTitle("Team");
  const qc = useQueryClient();
  const account = useQuery({ queryKey: ["forge", "account"], queryFn: accountsApi.account });
  const isAdmin = account.data?.role === "owner" || account.data?.role === "admin";
  const data = useQuery({ queryKey: ["forge", "admin"], queryFn: accountsApi.admin, enabled: isAdmin });
  const [role, setRole] = useState("member");
  const [team, setTeam] = useState(NO_TEAM);
  const [teamName, setTeamName] = useState("");
  const [issued, setIssued] = useState<{ url: string; title: string } | null>(null);
  const refresh = () => qc.invalidateQueries({ queryKey: ["forge", "admin"] });
  const fail = (e: unknown) => toast.error(e instanceof Error ? e.message : "Failed");

  const invite = useMutation({
    mutationFn: () => accountsApi.createInvite(role, team === NO_TEAM ? undefined : team),
    onSuccess: (r) => { setIssued({ url: r.url, title: "Invite link" }); refresh(); },
    onError: fail,
  });
  const reset = useMutation({
    mutationFn: (uid: string) => accountsApi.createReset(uid),
    onSuccess: (r) => { setIssued({ url: r.url, title: "Password reset link" }); refresh(); },
    onError: fail,
  });
  const addTeam = useMutation({
    mutationFn: () => accountsApi.createTeam(teamName),
    onSuccess: () => { setTeamName(""); refresh(); },
    onError: fail,
  });
  const revoke = useMutation({ mutationFn: accountsApi.revoke, onSuccess: refresh, onError: fail });

  if (account.isLoading) return <Page><div className="h-40 animate-pulse rounded-xl bg-muted" /></Page>;
  if (!account.data?.enabled || !isAdmin)
    return (
      <Page lane="focus">
        <h1 className="text-2xl font-semibold">Team</h1>
        <p className="mt-2 text-sm text-muted-foreground">Only workspace admins can manage members and links.</p>
      </Page>
    );

  const d = data.data;
  const teamName_ = (id: string) => d?.teams.find((t) => t.id === id)?.name;
  const memberName = (id: string) => {
    const m = d?.members.find((x) => x.id === id);
    return m ? m.name || m.username : id;
  };
  const activeLinks = (d?.links || []).filter((l) => !l.used && !l.revoked && new Date(l.expires_at) > new Date());

  return (
    <Page>
      <header className="mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">Team</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {account.data.workspace} · you are <b className="text-foreground">{account.data.role}</b>. Invite people with a link; they set their own name, username and password.
        </p>
      </header>

      <div className="grid gap-6 lg:grid-cols-[1fr_340px]">
        <div className="flex flex-col gap-6">
          <Card>
            <CardHeader>
              <CardTitle>Invite people</CardTitle>
              <CardDescription>Single-use link, expires in 7 days. Open it in an incognito window to test.</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-wrap items-end gap-3">
              <label className="flex flex-col gap-1.5 text-sm">
                <span className="font-medium">Workspace role</span>
                <Select value={role} onValueChange={setRole}>
                  <SelectTrigger className="w-40"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="member">Member</SelectItem>
                    <SelectItem value="guest">Guest</SelectItem>
                    {account.data.role === "owner" && <SelectItem value="admin">Admin</SelectItem>}
                  </SelectContent>
                </Select>
              </label>
              <label className="flex flex-col gap-1.5 text-sm">
                <span className="font-medium">Team</span>
                <Select value={team} onValueChange={setTeam}>
                  <SelectTrigger className="w-48"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NO_TEAM}>No team</SelectItem>
                    {(d?.teams || []).map((t) => <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>)}
                  </SelectContent>
                </Select>
              </label>
              <Button disabled={invite.isPending} onClick={() => invite.mutate()}>
                <Link2 /> Generate invite link
              </Button>
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle>Members</CardTitle></CardHeader>
            <CardContent>
              <ul className="divide-y">
                {(d?.members || []).map((m) => (
                  <li key={m.id} className="flex items-center justify-between gap-3 py-2.5 text-sm">
                    <div className="min-w-0">
                      <div className="font-medium">{m.name || m.email}</div>
                      <div className="text-xs text-muted-foreground">{m.username ? `@${m.username}` : `${m.email} · SSO`}</div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Badge variant={m.role === "owner" || m.role === "admin" ? "default" : "outline"}>{m.role}</Badge>
                      {m.username && (
                        <Button variant="ghost" size="sm" disabled={reset.isPending} onClick={() => reset.mutate(m.id)} title="Generate a one-hour password reset link">
                          <KeyRound /> Reset link
                        </Button>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        </div>

        <aside className="flex flex-col gap-6">
          <Card size="sm">
            <CardHeader><CardTitle className="text-sm">Teams</CardTitle></CardHeader>
            <CardContent className="flex flex-col gap-3">
              <form className="flex gap-2" onSubmit={(e) => { e.preventDefault(); if (teamName.trim()) addTeam.mutate(); }}>
                <Input value={teamName} onChange={(e) => setTeamName(e.target.value)} placeholder="e.g. Platform" />
                <Button type="submit" size="icon" variant="outline" title="Add team"><Plus /></Button>
              </form>
              <ul className="flex flex-wrap gap-1.5">
                {(d?.teams || []).map((t) => <Badge key={t.id} variant="secondary">{t.name}</Badge>)}
              </ul>
            </CardContent>
          </Card>
          <Card size="sm">
            <CardHeader><CardTitle className="text-sm">Active links</CardTitle></CardHeader>
            <CardContent>
              {activeLinks.length === 0 ? (
                <p className="text-xs text-muted-foreground">No active links.</p>
              ) : (
                <ul className="flex flex-col gap-2">
                  {activeLinks.map((l) => (
                    <li key={l.id} className="flex items-center justify-between gap-2 text-xs">
                      <div>
                        <div className="font-medium">
                          {l.kind === "invite" ? `Invite · ${l.role}` : `Reset · ${memberName(l.user_id)}`}
                          {l.team_id && ` · ${teamName_(l.team_id)}`}
                        </div>
                        <div className="text-muted-foreground">expires {formatTimestamp(l.expires_at)}</div>
                      </div>
                      <Button variant="ghost" size="icon-xs" title="Revoke" onClick={() => revoke.mutate(l.id)}><Trash2 /></Button>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>
        </aside>
      </div>

      <Dialog open={!!issued} onOpenChange={(o) => !o && setIssued(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{issued?.title}</DialogTitle>
            <DialogDescription>Copy it now — it is shown only once. Share it privately.</DialogDescription>
          </DialogHeader>
          <div className="flex gap-2">
            <Input readOnly value={issued?.url || ""} onFocus={(e) => e.currentTarget.select()} className="font-mono text-xs" />
            <Button
              variant="outline"
              size="icon"
              title="Copy"
              onClick={() => { navigator.clipboard.writeText(issued?.url || ""); toast.success("Copied"); }}
            >
              <Copy />
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </Page>
  );
}
