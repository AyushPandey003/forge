import { useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, ChevronRight, FileText, Folder, Network } from "lucide-react";
import { api, type FolderBreadcrumb, type FolderEntry } from "@/api";
import { queryKeys } from "@/lib/queryKeys";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

/** Fetch only expanded branches: large libraries never require a full recursive crawl. */
export function RelationshipMap({
  folderId,
  document,
}: {
  folderId?: string;
  document?: { id: string; title: string };
}) {
  const [open, setOpen] = useState(false);
  const folder = useQuery({
    queryKey: queryKeys.folder(folderId || ""),
    queryFn: () => api.getFolder(folderId!),
    enabled: open && !!folderId,
  });
  const ancestors = folder.data?.ancestors || [];
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          aria-label="Document relationship map"
          tooltip="Explore folders and documents"
        >
          <Network className="h-4 w-4" />
          <span className="hidden sm:inline">Map</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Document relationship map</DialogTitle>
          <DialogDescription>
            See where you are. Expand folders to explore their contents, or select a
            name to open it. Lines show folder membership, not links within documents.
          </DialogDescription>
        </DialogHeader>
        {open && (
          <div className="max-h-[60vh] overflow-auto rounded-lg border border-[var(--border)] p-3">
            <MapLink to="/app/documents" title="Documents" folder />
            {folderId ? (
              folder.isPending ? (
                <p role="status" className="p-3 text-sm">
                  Loading location…
                </p>
              ) : folder.isError ? (
                <div role="alert" className="p-3 text-sm">
                  Could not load this location.{" "}
                  <button className="underline" onClick={() => folder.refetch()}>
                    Try again
                  </button>
                </div>
              ) : folder.data ? (
                <AncestorPath ancestors={ancestors}>
                  <MapLink
                    to={`/app/folders/${folderId}`}
                    title={folder.data.title}
                    folder
                    current={!document}
                  />
                  <Branch
                    folderId={folderId}
                    currentDocumentId={document?.id}
                    visited={[...ancestors.map((a) => a.id), folderId]}
                  />
                </AncestorPath>
              ) : null
            ) : (
              <Branch currentDocumentId={document?.id} visited={[]} />
            )}
            {document && (
              <p className="mt-3 text-xs text-[var(--muted-foreground)]">
                Current document: {document.title}
              </p>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function AncestorPath({
  ancestors,
  children,
}: {
  ancestors: FolderBreadcrumb[];
  children: React.ReactNode;
}) {
  if (!ancestors.length)
    return <div className="ml-3 border-l border-[var(--border)] pl-3">{children}</div>;
  const [first, ...rest] = ancestors;
  return (
    <div className="ml-3 border-l border-[var(--border)] pl-3">
      <MapLink to={`/app/folders/${first.id}`} title={first.title} folder />
      <AncestorPath ancestors={rest}>{children}</AncestorPath>
    </div>
  );
}

function MapLink({
  to,
  title,
  folder,
  current,
}: {
  to: string;
  title: string;
  folder?: boolean;
  current?: boolean;
}) {
  const Icon = folder ? Folder : FileText;
  return (
    <Link
      to={to}
      aria-current={current ? "page" : undefined}
      className={`flex min-w-0 items-center gap-2 rounded-md px-2 py-2 text-sm hover:bg-[var(--accent)] focus-visible:outline-2 focus-visible:outline-[var(--primary)] ${current ? "bg-[var(--accent)] font-semibold text-[var(--primary)]" : ""}`}
    >
      <Icon className="h-4 w-4 shrink-0" />
      <span className="break-words">{title}</span>
      {current && <span className="ml-auto shrink-0 text-xs">You are here</span>}
    </Link>
  );
}

function Branch({
  folderId,
  currentDocumentId,
  visited,
}: {
  folderId?: string;
  currentDocumentId?: string;
  visited: string[];
}) {
  const contents = useQuery({
    queryKey: queryKeys.folders(folderId),
    queryFn: () => api.listFolders(folderId),
  });
  if (contents.isPending)
    return (
      <p role="status" className="p-2 text-sm">
        Loading contents…
      </p>
    );
  if (contents.isError)
    return (
      <p role="alert" className="p-2 text-sm">
        Could not load contents.{" "}
        <button className="underline" onClick={() => contents.refetch()}>
          Try again
        </button>
      </p>
    );
  const entries = [...(contents.data?.items || [])].sort((a, b) => {
    if (a.type !== b.type) return a.type === "folder" ? -1 : 1;
    return (a.folder?.title || a.document?.title || "").localeCompare(
      b.folder?.title || b.document?.title || "",
    );
  });
  return (
    <ul
      className="ml-3 border-l border-[var(--border)] pl-3"
      aria-label="Folder contents"
    >
      {!entries.length && (
        <li className="p-2 text-sm text-[var(--muted-foreground)]">
          No items in this folder.
        </li>
      )}
      {contents.data?.has_more && (
        <li className="p-2 text-xs text-[var(--muted-foreground)]">
          Showing a partial view. Open this folder to browse more items.
        </li>
      )}
      {entries.map((entry) =>
        entry.folder ? (
          <FolderNode
            key={entry.folder.id}
            entry={entry}
            currentDocumentId={currentDocumentId}
            visited={visited}
          />
        ) : entry.document ? (
          <li key={entry.document.id}>
            <MapLink
              to={`/app/d/${entry.document.id}`}
              title={entry.document.title}
              current={entry.document.id === currentDocumentId}
            />
          </li>
        ) : null,
      )}
    </ul>
  );
}

function FolderNode({
  entry,
  currentDocumentId,
  visited,
}: {
  entry: FolderEntry;
  currentDocumentId?: string;
  visited: string[];
}) {
  const [expanded, setExpanded] = useState(false);
  const folder = entry.folder!;
  const cyclic = visited.includes(folder.id);
  return (
    <li>
      <div className="flex items-center">
        <button
          type="button"
          disabled={cyclic}
          aria-expanded={expanded}
          aria-label={`${expanded ? "Collapse" : "Expand"} ${folder.title}`}
          onClick={() => setExpanded(!expanded)}
          className="rounded p-2 hover:bg-[var(--accent)]"
        >
          {expanded ? (
            <ChevronDown className="h-4 w-4" />
          ) : (
            <ChevronRight className="h-4 w-4" />
          )}
        </button>
        <MapLink to={`/app/folders/${folder.id}`} title={folder.title} folder />
      </div>
      {expanded && !cyclic && (
        <Branch
          folderId={folder.id}
          currentDocumentId={currentDocumentId}
          visited={[...visited, folder.id]}
        />
      )}
    </li>
  );
}
