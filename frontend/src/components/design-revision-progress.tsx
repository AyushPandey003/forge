import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { ChevronDown, Sparkles } from "lucide-react";

/** An honest activity indicator, not a simulated completion percentage. */
export function DesignRevisionProgress({
  stage,
  startedAt,
}: {
  stage?: string;
  startedAt?: string;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const [elapsed, setElapsed] = useState(0);
  useEffect(() => {
    const start = startedAt ? Date.parse(startedAt) : Date.now();
    const tick = () => setElapsed(Math.max(0, Math.floor((Date.now() - start) / 1000)));
    tick();
    const timer = window.setInterval(tick, 1000);
    return () => window.clearInterval(timer);
  }, [startedAt]);
  return createPortal(
    <aside className="revision-activity" aria-label="Design revision progress">
      <div className="revision-activity__rim" aria-hidden="true" />
      <div className="revision-activity__body">
        <button
          className="revision-activity__toggle"
          aria-expanded={!collapsed}
          onClick={() => setCollapsed(!collapsed)}
        >
          <span className="revision-activity__spark">
            <Sparkles size={17} />
          </span>
          <span className="revision-activity__heading">Reimagining your design</span>
          <span className="revision-activity__time">
            {Math.floor(elapsed / 60)}:{String(elapsed % 60).padStart(2, "0")}
          </span>
          <ChevronDown
            size={16}
            style={{ transform: collapsed ? "rotate(180deg)" : undefined }}
          />
        </button>
        {!collapsed && (
          <>
            <div className="revision-activity__preview" aria-hidden="true">
              <div className="revision-activity__mini">
                <div className="revision-activity__sidebar">
                  <i />
                  <i />
                  <i />
                  <i />
                </div>
                <div className="revision-activity__content">
                  <i className="revision-activity__title" />
                  <div className="revision-activity__cards">
                    <i />
                    <i />
                    <i />
                  </div>
                  <div className="revision-activity__chart">
                    <i />
                    <i />
                    <i />
                    <i />
                    <i />
                    <i />
                    <i />
                  </div>
                  <i className="revision-activity__line" />
                  <i className="revision-activity__line" />
                </div>
              </div>
              <div className="revision-activity__scan" />
            </div>
            <div className="revision-activity__status" role="status" aria-live="polite">
              <span className="revision-activity__dot" />
              {stage || "Preparing your revision…"}
            </div>
            <p className="revision-activity__note">
              Working in the background. Your current design stays available.
            </p>
          </>
        )}
      </div>
    </aside>,
    document.body,
  );
}
