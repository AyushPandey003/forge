import { LayoutGrid, List } from "lucide-react";
import { Button } from "@/components/ui/button";
import { viewModeStore } from "@/lib/view-mode";

const MODES = [
  { mode: "gallery", label: "Gallery view", icon: LayoutGrid },
  { mode: "list", label: "List view", icon: List },
] as const;

// A segmented control, as in Slash: the muted track holds a raised choice.
export function ViewModeToggle() {
  const viewMode = viewModeStore.useStore();
  return (
    <div className="flex shrink-0 gap-0.5 rounded-sm bg-[var(--muted)] p-0.5 ring-1 ring-[var(--border)] dark:ring-transparent">
      {MODES.map(({ mode, label, icon: Icon }) => (
        <Button
          key={mode}
          variant="ghost"
          size="icon"
          className="h-8 w-8 rounded-[6px] text-[var(--muted-foreground)] aria-pressed:bg-[var(--card)] aria-pressed:text-[var(--foreground)] aria-pressed:shadow-[var(--shadow-border)]"
          aria-pressed={viewMode === mode}
          onClick={() => viewModeStore.write(mode)}
          aria-label={label}
        >
          <Icon className="h-4 w-4" />
        </Button>
      ))}
    </div>
  );
}
