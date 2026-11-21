// Canvas screens are opaque sandbox frames: inject a selection action into
// each srcdoc, then validate its WindowProxy before relaying only bounded text.
export function installCanvasComments(
  d: Document,
  onComment: (quote: string) => void,
  onActivate?: (id: string) => void,
) {
  if (!d.getElementById("forge-canvas-data")) return;
  const installed = new WeakSet<HTMLIFrameElement>();
  function screenSelection() {
    let button: HTMLButtonElement | null = null;
    let overlays: HTMLElement[] = [];
    let latest: {
      items: { id: string; quote: string; author: string }[];
      active?: string;
    } = { items: [] };
    function paint() {
      overlays.forEach((el) => el.remove());
      overlays = [];
      const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
      const nodes: Text[] = [];
      let text = "";
      while (walker.nextNode()) {
        const node = walker.currentNode as Text;
        if (node.parentElement?.closest("script,style,button[data-forge-label]"))
          continue;
        nodes.push(node);
        text += node.textContent || "";
      }
      latest.items.forEach((item) => {
        const start = text.indexOf(item.quote);
        if (start < 0 || !item.quote) return;
        let offset = 0;
        const range = document.createRange();
        let found = false;
        for (const node of nodes) {
          const len = node.length;
          if (!found && start < offset + len) {
            range.setStart(node, start - offset);
            found = true;
          }
          if (found && start + item.quote.length <= offset + len) {
            range.setEnd(node, start + item.quote.length - offset);
            break;
          }
          offset += len;
        }
        if (!found) return;
        const rects = [...range.getClientRects()];
        rects.forEach((rect) => {
          const el = document.createElement("div");
          el.dataset.forgeHighlight = item.id;
          el.style.cssText = `position:absolute;pointer-events:none;z-index:2147483645;background:${latest.active === item.id ? "#a78bfa70" : "#facc1545"};border-bottom:2px solid #a78bfa;left:${rect.left + scrollX}px;top:${rect.top + scrollY}px;width:${rect.width}px;height:${rect.height}px`;
          document.body.appendChild(el);
          overlays.push(el);
        });
        const rect = rects[0];
        if (!rect) return;
        const label = document.createElement("button");
        label.type = "button";
        label.dataset.forgeLabel = "true";
        label.textContent = item.author || "Comment";
        label.setAttribute("aria-label", `Comment by ${item.author}`);
        label.style.cssText = `position:absolute;z-index:2147483646;left:${Math.max(0, rect.left + scrollX)}px;top:${Math.max(0, rect.top + scrollY - 23)}px;background:#6d28d9;color:white;font:600 11px system-ui;padding:3px 7px;border-radius:5px;border:0;cursor:pointer`;
        label.onclick = () =>
          parent.postMessage({ type: "forge:activate-comment", id: item.id }, "*");
        document.body.appendChild(label);
        overlays.push(label);
      });
    }
    window.addEventListener("message", (e) => {
      if (
        (e.source !== parent && e.source !== parent.parent) ||
        e.data?.type !== "forge:paint-comments" ||
        !Array.isArray(e.data.items)
      )
        return;
      latest = e.data;
      paint();
    });
    window.addEventListener("resize", paint);
    document.addEventListener("scroll", paint, { capture: true, passive: true });
    document.fonts?.ready.then(paint);
    document.addEventListener("mouseup", () => {
      setTimeout(() => {
        const selection = getSelection();
        const text = selection?.toString().trim();
        if (!text || !selection?.rangeCount) {
          button?.remove();
          return;
        }
        const rect = selection.getRangeAt(0).getBoundingClientRect();
        button?.remove();
        button = document.createElement("button");
        button.textContent = "💬 Comment";
        button.type = "button";
        button.setAttribute("aria-label", "Comment on selected text");
        button.style.cssText = `position:fixed;z-index:2147483647;left:${Math.max(8, Math.min(innerWidth - 150, rect.left))}px;top:${Math.max(8, rect.top - 42)}px;background:#18181b;color:white;border:1px solid #71717a;border-radius:8px;padding:9px 14px;font:600 13px system-ui;cursor:pointer;box-shadow:0 4px 20px #0004`;
        button.onmousedown = (e) => e.preventDefault();
        button.onclick = () => {
          parent.postMessage(
            { type: "forge:screen-comment", quote: text.slice(0, 4000) },
            "*",
          );
          button?.remove();
        };
        document.body.appendChild(button);
      }, 0);
    });
  }
  function scan() {
    d.querySelectorAll<HTMLIFrameElement>("#world iframe").forEach((frame) => {
      if (installed.has(frame) || !frame.srcdoc) return;
      installed.add(frame);
      frame.addEventListener("load", broadcast);
      const script = `<script>(${screenSelection.toString()})();<` + `/script>`;
      frame.srcdoc += script;
    });
  }
  d.defaultView?.addEventListener("message", (e) => {
    if (!["forge:screen-comment", "forge:activate-comment"].includes(e.data?.type))
      return;
    const frame = [...d.querySelectorAll<HTMLIFrameElement>("#world iframe")].find(
      (f) => f.contentWindow === e.source,
    );
    if (!frame || !installed.has(frame)) return;
    if (e.data.type === "forge:activate-comment") {
      if (typeof e.data.id === "string") onActivate?.(e.data.id.slice(0, 200));
      return;
    }
    if (typeof e.data.quote !== "string") return;
    const text = e.data.quote.trim().slice(0, 4000);
    if (text) onComment(`[Screen: ${frame.title.slice(0, 120)}] ${text}`);
  });
  let latest: {
    items: { id: string; quote?: string; author?: string }[];
    active?: string;
  } = { items: [] };
  function broadcast() {
    d.querySelectorAll<HTMLIFrameElement>("#world iframe").forEach((frame) => {
      const prefix = `[Screen: ${frame.title.slice(0, 120)}] `;
      const items = latest.items
        .filter((i) => i.quote?.startsWith(prefix))
        .map((i) => ({ ...i, quote: i.quote!.slice(prefix.length) }));
      frame.contentWindow?.postMessage(
        { type: "forge:paint-comments", items, active: latest.active },
        "*",
      );
    });
  }
  scan();

  const observer = new MutationObserver(scan);
  observer.observe(d.body, { childList: true, subtree: true });
  d.defaultView?.addEventListener("pagehide", () => observer.disconnect(), {
    once: true,
  });
  return (items: typeof latest.items, active?: string) => {
    latest = { items, active };
    broadcast();
  };
}
