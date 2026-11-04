import * as React from "react";
import { cn } from "@/lib/cn";

export function Card({ className, ...p }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "rounded-lg border border-[var(--border)] bg-[var(--card)] shadow-[var(--shadow-card)] dark:border-transparent",
        className,
      )}
      {...p}
    />
  );
}
export function CardHeader({ className, ...p }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("p-5 pb-3", className)} {...p} />;
}
export function CardTitle({
  className,
  ...p
}: React.HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3
      className={cn("text-balance text-base font-semibold tracking-tight", className)}
      {...p}
    />
  );
}
export function CardDescription({
  className,
  ...p
}: React.HTMLAttributes<HTMLParagraphElement>) {
  return (
    <p
      className={cn("text-pretty text-sm text-[var(--muted-foreground)]", className)}
      {...p}
    />
  );
}
export function CardContent({ className, ...p }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("p-5 pt-2", className)} {...p} />;
}
export function CardFooter({ className, ...p }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "flex items-center gap-2 p-5 pt-0 border-t border-[var(--border)] mt-3",
        className,
      )}
      {...p}
    />
  );
}
