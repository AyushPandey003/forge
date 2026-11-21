// Legacy general-design comments use a nonempty quote because the comment API
// requires one. It is a label, not text to highlight inside an iframe.
export function isDesignFeedback(quote?: string) {
  return quote === "Design feedback" || !!quote?.startsWith("[Screen: ");
}
