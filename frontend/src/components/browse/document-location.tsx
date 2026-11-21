import { Link } from "react-router-dom";
import { Folder } from "lucide-react";

/** Uses server-provided location only; never fetch a folder for every search result. */
export function DocumentLocation({
  parentFolder,
  parentFolderId,
}: {
  parentFolder?: { id: string; title: string };
  parentFolderId?: string;
}) {
  const id = parentFolder?.id || parentFolderId;
  return (
    <div className="mt-1 flex min-w-0 items-center gap-1 text-xs text-[var(--muted-foreground)]">
      <Folder aria-hidden="true" className="h-3 w-3 shrink-0" />
      {id ? (
        <Link
          to={`/app/folders/${id}`}
          className="truncate hover:underline"
          title={`Open containing folder: ${parentFolder?.title || "Folder"}`}
        >
          {parentFolder?.title || "Open containing folder"}
        </Link>
      ) : (
        <span>Documents · top level</span>
      )}
    </div>
  );
}
