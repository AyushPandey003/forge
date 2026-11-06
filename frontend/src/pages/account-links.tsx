// Public pages opened from invite / reset links.
import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@kalvi-education/katalyst/components/button";
import { Input } from "@kalvi-education/katalyst/components/input";
import { Alert, AlertDescription, AlertTitle } from "@kalvi-education/katalyst/components/alert";
import { Logo } from "@/components/layout/logo";
import { useTitle } from "@/lib/use-title";
import { accountsApi, tokenFromHash } from "@/lib/accounts";

function Shell({ title, subtitle, children }: { title: string; subtitle?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex min-h-full items-center justify-center bg-background px-4 py-16">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex justify-center">
          <Logo className="h-7" />
        </div>
        <div className="rounded-xl border bg-card p-6">
          <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
          {subtitle && <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>}
          <div className="mt-6">{children}</div>
        </div>
      </div>
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5 text-sm">
      <span className="font-medium">{label}</span>
      {children}
      {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
    </label>
  );
}

function useLinkToken() {
  // Read once, then strip the token from the address bar and history.
  const [token] = useState(tokenFromHash);
  useEffect(() => {
    if (window.location.hash) history.replaceState(null, "", window.location.pathname);
  }, []);
  return token;
}

function passwordError(password: string, confirm: string) {
  if (password !== confirm) return "Passwords don't match.";
  const bytes = new TextEncoder().encode(password).length;
  if (bytes < 12 || bytes > 72) return "Password must be 12–72 bytes.";
  return "";
}

function BadLink({ message }: { message: string }) {
  return (
    <Shell title="Link not valid">
      <Alert variant="destructive">
        <AlertTitle>{message}</AlertTitle>
        <AlertDescription>Ask a workspace admin for a new link.</AlertDescription>
      </Alert>
      <Button variant="outline" className="mt-4 w-full" asChild>
        <Link to="/login">Go to sign in</Link>
      </Button>
    </Shell>
  );
}

export function JoinPage() {
  useTitle("Join");
  const token = useLinkToken();
  const qc = useQueryClient();
  const link = useQuery({
    queryKey: ["account-link", token],
    queryFn: () => accountsApi.inspect(token),
    enabled: !!token,
    retry: false,
    staleTime: Infinity,
    refetchOnReconnect: false,
    meta: { suppressGlobalError: true },
  });
  const [name, setName] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  if (!token) return <BadLink message="This invite link is incomplete." />;
  if (link.isLoading) return <Shell title="Checking invite…">{null}</Shell>;
  if (link.error || link.data?.kind !== "invite")
    return <BadLink message="This invite has expired, was already used, or was revoked." />;

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (busy) return;
    const validation = passwordError(password, confirm);
    if (validation) return setError(validation);
    setBusy(true);
    try {
      await qc.cancelQueries({ queryKey: ["account-link", token] });
      await accountsApi.register({ token, name, username, password, confirm_password: confirm });
      // Clean navigation discards the consumed invite and prior identity cache.
      window.location.replace("/app/solutions");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create account");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Shell
      title={`Join ${link.data.workspace}`}
      subtitle={<>You're invited as <b className="text-foreground">{link.data.role}</b>.</>}
    >
      <form className="flex flex-col gap-4" onSubmit={submit}>
        <Field label="Full name">
          <Input value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" required autoFocus />
        </Field>
        <Field label="Username" hint="3–32 characters: letters, numbers, - or _">
          <Input
            value={username}
            onChange={(e) => setUsername(e.target.value.toLowerCase())}
            autoComplete="username"
            required
            pattern="[a-z0-9][a-z0-9_\-]{2,31}"
          />
        </Field>
        <Field label="Password" hint="At least 12 characters">
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" minLength={12} required />
        </Field>
        <Field label="Confirm password">
          <Input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" required />
        </Field>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <Button type="submit" disabled={busy}>
          {busy ? "Creating account…" : "Create account"}
        </Button>
      </form>
    </Shell>
  );
}

export function ResetPasswordPage() {
  useTitle("Reset password");
  const token = useLinkToken();
  const nav = useNavigate();
  const qc = useQueryClient();
  const link = useQuery({
    queryKey: ["account-link", token],
    queryFn: () => accountsApi.inspect(token),
    enabled: !!token,
    retry: false,
    staleTime: Infinity,
    refetchOnReconnect: false,
    meta: { suppressGlobalError: true },
  });
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  if (!token) return <BadLink message="This reset link is incomplete." />;
  if (link.isLoading) return <Shell title="Checking link…">{null}</Shell>;
  if (!done && (link.error || link.data?.kind !== "reset"))
    return <BadLink message="This reset link has expired, was already used, or was revoked." />;
  if (done)
    return (
      <Shell title="Password updated" subtitle="All other sessions were signed out.">
        <Button className="w-full" onClick={() => nav("/login", { replace: true })}>
          Sign in
        </Button>
      </Shell>
    );

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (busy) return;
    const validation = passwordError(password, confirm);
    if (validation) return setError(validation);
    setBusy(true);
    try {
      await qc.cancelQueries({ queryKey: ["account-link", token] });
      await accountsApi.reset(token, password, confirm);
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not reset password");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Shell title="Set a new password" subtitle="This link works once and expires in an hour.">
      <form className="flex flex-col gap-4" onSubmit={submit}>
        <Field label="New password" hint="At least 12 characters">
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" minLength={12} required autoFocus />
        </Field>
        <Field label="Confirm password">
          <Input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" required />
        </Field>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <Button type="submit" disabled={busy}>
          {busy ? "Saving…" : "Update password"}
        </Button>
      </form>
    </Shell>
  );
}
