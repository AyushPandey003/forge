// Bring-your-own-model settings for the design studio: save provider
// connections (OpenAI-compatible or Anthropic) with your own API key.
// Keys are sent once, stored encrypted, and never shown again.
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { KeyRound, Pencil, Plus, Trash2 } from "lucide-react";
import { Button } from "@kalvi-education/katalyst/components/button";
import { Input } from "@kalvi-education/katalyst/components/input";
import { Badge } from "@kalvi-education/katalyst/components/badge";
import { Alert, AlertDescription } from "@kalvi-education/katalyst/components/alert";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@kalvi-education/katalyst/components/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@kalvi-education/katalyst/components/alert-dialog";
import { errorMessage } from "@/lib/errors";
import { queryKeys } from "@/lib/queryKeys";
import {
  forgeApi,
  PROVIDER_PRESETS,
  type AIProvider,
  type ProviderInput,
  type ProviderKind,
} from "@/lib/forge";

const providersKey = queryKeys.forgeProviders();

export function ProvidersDialog({
  open,
  onOpenChange,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called with the provider the user just created or edited. */
  onSaved?: (p: AIProvider) => void;
}) {
  const qc = useQueryClient();
  const list = useQuery({
    queryKey: providersKey,
    queryFn: forgeApi.providers,
    enabled: open,
  });
  const [editing, setEditing] = useState<AIProvider | "new" | null>(null);
  const [removing, setRemoving] = useState<AIProvider | null>(null);
  const items = list.data?.items || [];

  const remove = useMutation({
    mutationFn: (id: string) => forgeApi.deleteProvider(id),
    onSuccess: () => {
      toast.success("Provider removed.");
      setRemoving(null);
      setEditing(null);
      qc.invalidateQueries({ queryKey: providersKey });
    },
    onError: (e) => toast.error(errorMessage(e, "Could not remove provider")),
  });

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Model providers</DialogTitle>
            <DialogDescription>
              Use your own API keys and models to generate designs. Keys are encrypted
              and only used by the server to call your provider on your behalf.
            </DialogDescription>
          </DialogHeader>

          {list.isError && (
            <Alert variant="destructive">
              <AlertDescription>
                {errorMessage(list.error, "Could not load providers.")}{" "}
                <button className="underline" onClick={() => list.refetch()}>
                  Try again
                </button>
              </AlertDescription>
            </Alert>
          )}

          {editing ? (
            <ProviderForm
              key={editing === "new" ? "new" : editing.id}
              provider={editing === "new" ? null : editing}
              allowPrivate={!!list.data?.allow_private_urls}
              onCancel={() => setEditing(null)}
              onRemove={editing === "new" ? undefined : () => setRemoving(editing)}
              onSaved={(p) => {
                setEditing(null);
                qc.invalidateQueries({ queryKey: providersKey });
                onSaved?.(p);
              }}
            />
          ) : (
            <div className="flex flex-col gap-3">
              {list.isPending ? (
                <p role="status" className="text-sm text-muted-foreground">
                  Loading providers…
                </p>
              ) : items.length === 0 ? (
                <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
                  No providers yet. Add one to start generating with your own model.
                </p>
              ) : (
                <ul className="flex flex-col gap-2" aria-label="Saved providers">
                  {items.map((p) => (
                    <li
                      key={p.id}
                      className="flex items-center justify-between gap-3 rounded-lg border p-3"
                    >
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="truncate text-sm font-medium">
                            {p.label}
                          </span>
                          <Badge variant="outline">
                            {p.kind === "anthropic" ? "Anthropic" : "OpenAI-compatible"}
                          </Badge>
                        </div>
                        <div className="truncate text-xs text-muted-foreground">
                          {p.base_url}
                          {p.default_model ? ` · ${p.default_model}` : ""}
                          {p.has_key ? ` · key ${p.key_hint || "saved"}` : " · no key"}
                        </div>
                      </div>
                      <div className="flex shrink-0 gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          aria-label={`Edit ${p.label}`}
                          onClick={() => setEditing(p)}
                        >
                          <Pencil /> Edit
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-destructive"
                          aria-label={`Remove ${p.label}`}
                          onClick={() => setRemoving(p)}
                        >
                          <Trash2 /> Remove
                        </Button>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
              <DialogFooter>
                <Button
                  onClick={() => setEditing("new")}
                  disabled={items.length >= (list.data?.limit ?? 10)}
                >
                  <Plus /> Add provider
                </Button>
              </DialogFooter>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!removing} onOpenChange={(o) => !o && setRemoving(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove “{removing?.label}”?</AlertDialogTitle>
            <AlertDialogDescription>
              Its saved API key is deleted. Designs already generated are not affected.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              disabled={remove.isPending}
              onClick={(e) => {
                e.preventDefault();
                if (removing) remove.mutate(removing.id);
              }}
            >
              {remove.isPending ? "Removing…" : "Remove"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

function ProviderForm({
  provider,
  allowPrivate,
  onCancel,
  onRemove,
  onSaved,
}: {
  provider: AIProvider | null;
  allowPrivate: boolean;
  onCancel: () => void;
  onRemove?: () => void;
  onSaved: (p: AIProvider) => void;
}) {
  const [preset, setPreset] = useState(provider ? "" : "openrouter");
  const first = PROVIDER_PRESETS[0];
  const [label, setLabel] = useState(provider?.label ?? first.label);
  const [kind, setKind] = useState<ProviderKind>(provider?.kind ?? first.kind);
  const [baseUrl, setBaseUrl] = useState(provider?.base_url ?? first.base_url);
  const [model, setModel] = useState(provider?.default_model ?? first.model);
  const [apiKey, setApiKey] = useState("");

  const needKey = !provider?.has_key && !allowPrivate;
  const valid =
    label.trim() !== "" &&
    /^https?:\/\//i.test(baseUrl.trim()) &&
    (!needKey || apiKey.trim() !== "");

  function applyPreset(id: string) {
    setPreset(id);
    const p = PROVIDER_PRESETS.find((x) => x.id === id);
    if (!p) return;
    setLabel(p.id === "custom" ? "" : p.label);
    setKind(p.kind);
    setBaseUrl(p.base_url);
    setModel(p.model);
  }

  const save = useMutation({
    mutationFn: () => {
      const input: ProviderInput = {
        label: label.trim(),
        kind,
        base_url: baseUrl.trim(),
        default_model: model.trim(),
        ...(apiKey.trim() ? { api_key: apiKey.trim() } : {}),
      };
      return provider
        ? forgeApi.updateProvider(provider.id, input)
        : forgeApi.createProvider(input);
    },
    onSuccess: (p) => {
      toast.success(provider ? "Provider updated." : "Provider added.");
      onSaved(p);
    },
    onError: (e) => toast.error(errorMessage(e, "Could not save provider")),
  });

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (valid && !save.isPending) save.mutate();
      }}
    >
      {!provider && (
        <div
          className="flex flex-wrap gap-2"
          role="group"
          aria-label="Provider presets"
        >
          {PROVIDER_PRESETS.map((p) => (
            <Button
              key={p.id}
              type="button"
              size="sm"
              variant={preset === p.id ? "default" : "outline"}
              aria-pressed={preset === p.id}
              title={p.hint}
              onClick={() => applyPreset(p.id)}
            >
              {p.label}
            </Button>
          ))}
        </div>
      )}
      <div className="grid gap-4 sm:grid-cols-2">
        <label className="flex flex-col gap-1.5 text-sm font-medium">
          Name
          <Input
            value={label}
            maxLength={60}
            onChange={(e) => setLabel(e.target.value)}
          />
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-medium">
          Default model
          <Input
            value={model}
            placeholder="e.g. gpt-4o"
            onChange={(e) => setModel(e.target.value)}
          />
        </label>
      </div>
      <label className="flex flex-col gap-1.5 text-sm font-medium">
        Base URL
        <Input
          value={baseUrl}
          inputMode="url"
          placeholder="https://api.example.com/v1"
          onChange={(e) => setBaseUrl(e.target.value)}
        />
        <span className="text-xs font-normal text-muted-foreground">
          {kind === "anthropic"
            ? "Messages API. Requests go to <base>/v1/messages."
            : "OpenAI-compatible. Requests go to <base>/chat/completions."}
          {allowPrivate
            ? " Local servers such as Ollama are allowed on this server."
            : ""}
        </span>
      </label>
      <label className="flex flex-col gap-1.5 text-sm font-medium">
        <span className="flex items-center gap-1.5">
          <KeyRound className="size-3.5" /> API key
        </span>
        <Input
          type="password"
          autoComplete="off"
          spellCheck={false}
          value={apiKey}
          placeholder={
            provider?.has_key
              ? `Saved ${provider.key_hint} — leave blank to keep`
              : "Paste your API key"
          }
          onChange={(e) => setApiKey(e.target.value)}
        />
      </label>
      <DialogFooter>
        {onRemove && (
          <Button
            type="button"
            variant="outline"
            className="text-destructive sm:mr-auto"
            onClick={onRemove}
            disabled={save.isPending}
          >
            <Trash2 /> Remove provider
          </Button>
        )}
        <Button
          type="button"
          variant="ghost"
          onClick={onCancel}
          disabled={save.isPending}
        >
          Cancel
        </Button>
        <Button type="submit" disabled={!valid || save.isPending}>
          {save.isPending ? "Saving…" : provider ? "Save changes" : "Add provider"}
        </Button>
      </DialogFooter>
    </form>
  );
}
