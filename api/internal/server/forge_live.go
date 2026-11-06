package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// forgeLive streams invalidations, never document contents. The initial event
// always requests an authorized refetch, so reconnects cannot leave stale data.
// Durable per-user revisions coalesce all changes during a disconnected period.
func (h handlers) forgeLive(c *gin.Context) {
	if !h.forgeEnabled(c) {
		return
	}
	userID := current(c).ID
	ctx := c.Request.Context()
	revision, err := h.deps.forge.LiveRevision(ctx, userID)
	if err != nil {
		internalErr(c, err)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	controller := http.NewResponseController(c.Writer)
	defer controller.SetWriteDeadline(time.Time{}) // best effort on supported servers
	send := func() bool {
		_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_, err := fmt.Fprintf(c.Writer, "id: %d\nevent: invalidate\ndata: {}\n\n", revision)
		c.Writer.Flush()
		return err == nil
	}
	if !send() {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	// Bound stream lifetime: authentication middleware rechecks the session on
	// reconnect, including resets/logout on another tab. No payload is sent here.
	lifetime := time.NewTimer(time.Minute)
	defer lifetime.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-lifetime.C:
			return
		case <-heartbeat.C:
			_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := fmt.Fprint(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case <-ticker.C:
			next, err := h.deps.forge.LiveRevision(ctx, userID)
			if err != nil {
				return
			}
			if next != revision {
				revision = next
				if !send() {
					return
				}
			}
		}
	}
}
