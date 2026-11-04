import { BRAND } from "@/lib/brand";
import { cn } from "@/lib/cn";

// Forge wordmark: a monochrome mark plus the product name, set in the KDS face.
export function Logo({ className }: { className?: string }) {
  return (
    <span
      role="img"
      aria-label={BRAND.name}
      className={cn("inline-flex h-6 items-center gap-2 text-foreground", className)}
    >
      <svg viewBox="0 0 24 24" className="h-full w-auto" aria-hidden>
        <rect x="1" y="1" width="22" height="22" rx="6" className="fill-primary" />
        <path
          d="M8 17V7h8M8 12h6"
          className="stroke-primary-foreground"
          strokeWidth="2.25"
          strokeLinecap="round"
          strokeLinejoin="round"
          fill="none"
        />
      </svg>
      <span className="text-[1.05em] font-semibold leading-none tracking-tight">
        {BRAND.name}
      </span>
    </span>
  );
}
