import { cn } from "@/lib/cn";

// A quiet, monochrome page backdrop: a hairline dot grid that fades out
// downward. KDS keeps chrome achromatic, so there is no hued wash here.
export function RazorSenseBackdrop({
  framing = "page",
  className,
}: {
  framing?: "halo" | "page";
  className?: string;
}) {
  return (
    <div
      aria-hidden
      className={cn(
        "pointer-events-none absolute inset-x-0 top-0 -z-10 overflow-hidden",
        framing === "halo" ? "h-[640px]" : "h-[320px]",
        className,
      )}
      style={{
        backgroundImage:
          "radial-gradient(color-mix(in oklch, var(--foreground) 14%, transparent) 1px, transparent 1px)",
        backgroundSize: "22px 22px",
        maskImage: "linear-gradient(to bottom, black, transparent)",
        WebkitMaskImage: "linear-gradient(to bottom, black, transparent)",
      }}
    />
  );
}
