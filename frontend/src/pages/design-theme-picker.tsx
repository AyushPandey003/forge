// Theme swatches for canvas designs. Colors come from the server so the
// picker always matches what the model is given.
import { useQuery } from "@tanstack/react-query";
import { Check } from "lucide-react";
import { forgeApi } from "@/lib/forge";

export function DesignThemePicker({
  value,
  onChange,
  disabled,
}: {
  /** Empty string means "let the model choose". */
  value: string;
  onChange: (id: string) => void;
  disabled?: boolean;
}) {
  const themes = useQuery({
    queryKey: ["forge", "design", "themes"],
    queryFn: forgeApi.themes,
    staleTime: Infinity,
  });
  const items = themes.data?.items || [];
  if (themes.isError) return null; // optional enhancement: generation works without it
  return (
    <div className="flex flex-col gap-2">
      <span id="theme-label" className="text-sm font-medium">
        Theme <span className="font-normal text-muted-foreground">(optional)</span>
      </span>
      <div
        role="radiogroup"
        aria-labelledby="theme-label"
        className="flex flex-wrap gap-2"
      >
        <Chip
          selected={value === ""}
          disabled={disabled}
          onClick={() => onChange("")}
          label="Auto"
        >
          <span className="text-xs font-medium">Auto</span>
        </Chip>
        {items.map((t) => (
          <Chip
            key={t.id}
            selected={value === t.id}
            disabled={disabled}
            onClick={() => onChange(t.id)}
            label={t.name}
          >
            <span className="flex -space-x-1" aria-hidden="true">
              {["background", "primary", "accent"].map((k) => (
                <span
                  key={k}
                  className="size-4 rounded-full border border-black/10"
                  style={{ background: t.colors[k] || "#ccc" }}
                />
              ))}
            </span>
            <span className="text-xs font-medium">{t.name}</span>
          </Chip>
        ))}
      </div>
    </div>
  );
}

function Chip({
  selected,
  disabled,
  onClick,
  label,
  children,
}: {
  selected: boolean;
  disabled?: boolean;
  onClick: () => void;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
      className={`flex items-center gap-2 rounded-full border px-3 py-1.5 transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 disabled:opacity-50 ${
        selected ? "border-primary bg-primary/10" : "hover:bg-muted"
      }`}
    >
      {children}
      {selected && <Check className="size-3.5" aria-hidden="true" />}
    </button>
  );
}
