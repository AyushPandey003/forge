import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, ChevronRight, Folder, FolderInput } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/api";
import { queryKeys } from "@/lib/queryKeys";
import { errorMessage } from "@/lib/errors";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { AccessWarningDialog } from "./dialogs";

type Location = { id: string; title: string };

export function MoveDocumentDialog({
  documentId,
  title,
  parentFolder,
}: {
  documentId: string;
  title: string;
  parentFolder?: Location;
}) {
  const [open, setOpen] = useState(false);
  const [path, setPath] = useState<Location[]>([]);
  const [filter, setFilter] = useState("");
  const [warning, setWarning] = useState<string | null>(null);
  const target = path.at(-1);
  const qc = useQueryClient();
  const folders = useQuery({
    queryKey: queryKeys.folders(target?.id),
    queryFn: () => api.listFolders(target?.id),
    enabled: open,
  });
  const move = useMutation({
    mutationFn: async () => {
      if (target) return api.moveDocumentToFolder(target.id, documentId);
      if (parentFolder) return api.moveDocumentToRoot(parentFolder.id, documentId);
    },
    onSuccess: (result) => {
      qc.invalidateQueries({ queryKey: queryKeys.foldersAll() });
      qc.invalidateQueries({ queryKey: queryKeys.documents() });
      qc.invalidateQueries({ queryKey: queryKeys.document(documentId) });
      qc.invalidateQueries({ queryKey: queryKeys.documentAccess(documentId) });
      qc.invalidateQueries({ queryKey: queryKeys.pins() });
      setOpen(false);
      if (result?.permission_warning?.message)
        setWarning(result.permission_warning.message);
      else toast.success(`Moved to ${target?.title || "Documents"}.`);
    },
    onError: (error) =>
      toast.error(
        errorMessage(
          error,
          "Could not move document. Try another folder or check your access.",
        ),
      ),
  });
  const unchanged = (target?.id || "") === (parentFolder?.id || "");
  const items = (folders.data?.items || [])
    .flatMap((entry) => (entry.folder ? [entry.folder] : []))
    .filter((folder) =>
      folder.title.toLocaleLowerCase().includes(filter.trim().toLocaleLowerCase()),
    )
    .sort((a, b) => a.title.localeCompare(b.title));
  function changePath(next: Location[]) {
    setPath(next);
    setFilter("");
  }
  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(next) => {
          if (!move.isPending) {
            setOpen(next);
            if (next) {
              changePath([]);
              move.reset();
            }
          }
        }}
      >
        <DialogTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Move document"
            tooltip="Move document"
          >
            <FolderInput className="h-4 w-4" />
          </Button>
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Move document</DialogTitle>
            <DialogDescription>
              Choose a destination for “{title}”. Moving can change who has access
              through folder sharing.
            </DialogDescription>
          </DialogHeader>
          <p className="text-xs text-[var(--muted-foreground)]">
            Current location: {parentFolder?.title || "Documents (top level)"}
          </p>
          <nav
            aria-label="Destination path"
            className="flex flex-wrap items-center gap-1 text-sm"
          >
            <button
              disabled={move.isPending}
              onClick={() => changePath([])}
              className="rounded px-1 py-1 underline"
            >
              Documents
            </button>
            {path.map((folder, index) => (
              <span key={folder.id} className="flex items-center gap-1">
                <ChevronRight className="h-3 w-3" />
                <button
                  disabled={move.isPending}
                  className="rounded px-1 py-1 underline"
                  onClick={() => changePath(path.slice(0, index + 1))}
                >
                  {folder.title}
                </button>
              </span>
            ))}
          </nav>
          <Input
            aria-label="Filter destination folders"
            placeholder="Find a folder here…"
            value={filter}
            disabled={move.isPending}
            onChange={(event) => setFilter(event.target.value)}
          />
          <div className="max-h-64 overflow-auto rounded-lg border border-[var(--border)] p-2">
            {!!path.length && (
              <Button
                variant="ghost"
                size="sm"
                disabled={move.isPending}
                onClick={() => changePath(path.slice(0, -1))}
              >
                <ArrowLeft className="h-4 w-4" />
                Up one level
              </Button>
            )}
            {folders.isPending ? (
              <p role="status" className="p-3 text-sm">
                Loading folders…
              </p>
            ) : folders.isError ? (
              <p role="alert" className="p-3 text-sm">
                Could not load folders.{" "}
                <button className="underline" onClick={() => folders.refetch()}>
                  Try again
                </button>
              </p>
            ) : (
              <>
                {items.map((folder) => (
                  <button
                    key={folder.id}
                    disabled={move.isPending}
                    onClick={() =>
                      changePath([...path, { id: folder.id, title: folder.title }])
                    }
                    className="flex w-full items-center gap-2 rounded-md p-3 text-left text-sm hover:bg-[var(--accent)]"
                  >
                    <Folder className="h-4 w-4 shrink-0" />
                    <span className="min-w-0 flex-1 break-words">{folder.title}</span>
                    <ChevronRight className="h-4 w-4 shrink-0" />
                  </button>
                ))}
                {!items.length && (
                  <p className="p-3 text-sm text-[var(--muted-foreground)]">
                    {filter.trim()
                      ? "No matching folders here."
                      : "No subfolders. You can move the document here."}
                  </p>
                )}
                {folders.data?.has_more && (
                  <p className="p-3 text-xs text-[var(--muted-foreground)]">
                    Only some folders are shown.
                  </p>
                )}
              </>
            )}
          </div>
          <p role="status" className="text-sm">
            {unchanged ? (
              "The document is already in this location."
            ) : (
              <>
                Destination: <strong>{target?.title || "Documents (top level)"}</strong>
              </>
            )}
          </p>
          {move.isError && (
            <p role="alert" className="text-sm text-[var(--destructive)]">
              {errorMessage(
                move.error,
                "Could not move document. Check your access and try again.",
              )}
            </p>
          )}
          <DialogFooter>
            <Button
              variant="ghost"
              disabled={move.isPending}
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button
              disabled={
                unchanged || move.isPending || folders.isPending || folders.isError
              }
              onClick={() => move.mutate()}
            >
              {move.isPending ? "Moving…" : "Move here"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <AccessWarningDialog warning={warning} onClose={() => setWarning(null)} />
    </>
  );
}
