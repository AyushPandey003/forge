package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/razorpay/slashdocs/api/internal/auth"
	fp "github.com/razorpay/slashdocs/api/internal/forge"
)

const localCookie = "forge_local_session"

// Local sessions are opaque database tokens, so a password reset invalidates
// all prior sessions immediately. Existing OIDC authentication remains intact.
type localAuthenticator struct {
	base    auth.Authenticator
	store   *fp.Store
	resolve auth.UserResolver
}

func (a localAuthenticator) Authenticate(r *http.Request) (*auth.Principal, error) {
	if auth.BearerFromHeader(r.Header.Get("Authorization")) != "" {
		return a.base.Authenticate(r)
	}
	if ck, err := r.Cookie(localCookie); err == nil && ck.Value != "" {
		uid, err := a.store.LocalSessionUser(r.Context(), ck.Value)
		if errors.Is(err, fp.ErrBadCredential) {
			return nil, auth.ErrUnauthorized
		}
		if err != nil {
			return nil, err
		}
		p, err := a.resolve.ResolveUser(r.Context(), uid)
		if err != nil {
			return nil, err
		}
		return &p, nil
	}
	return a.base.Authenticate(r)
}
func (h handlers) localAuthEnabled() bool {
	return h.deps.forge != nil && os.Getenv("FORGE_LOCAL_AUTH") == "true"
}
func (h handlers) registerLocalAccountPublic(r *gin.RouterGroup) {
	r.GET("/auth/local/config", func(c *gin.Context) { c.JSON(200, gin.H{"enabled": h.localAuthEnabled()}) })
	guard := func(c *gin.Context) {
		if !h.localAuthEnabled() {
			c.AbortWithStatus(404)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	}
	// Per-IP budgets are a prototype safeguard, not a distributed limiter.
	r.POST("/auth/local/login", guard, ipRateLimit(12), h.localLogin)
	r.POST("/auth/local/link", guard, ipRateLimit(30), h.localLink)
	r.POST("/auth/local/register", guard, ipRateLimit(8), h.localRegister)
	r.POST("/auth/local/reset", guard, ipRateLimit(8), h.localReset)
}
func (h handlers) registerLocalAccountAdmin(r *gin.RouterGroup) {
	r.GET("/forge/account", h.localAccount)
	admin := r.Group("/forge/admin", h.requireLocalAdmin)
	admin.GET("/accounts", h.localAdminAccounts)
	admin.POST("/teams", h.localCreateTeam)
	admin.POST("/invites", h.localCreateInvite)
	admin.POST("/resets", h.localCreateReset)
	admin.DELETE("/links/:lid", h.localRevokeLink)
}
func (h handlers) ensureLocalAdmin(ctx context.Context, p *auth.Principal) error {
	return h.deps.forge.BootstrapLocalAdmin(ctx, p.ID, p.Email, os.Getenv("FORGE_LOCAL_ADMIN_EMAIL"))
}
func (h handlers) requireLocalAdmin(c *gin.Context) {
	if !h.localAuthEnabled() {
		c.AbortWithStatus(404)
		return
	}
	if err := h.ensureLocalAdmin(c.Request.Context(), current(c)); err != nil {
		internalErr(c, err)
		c.Abort()
		return
	}
	role, err := h.deps.forge.WorkspaceRole(c.Request.Context(), current(c).ID)
	if err != nil {
		internalErr(c, err)
		c.Abort()
		return
	}
	if role != "owner" && role != "admin" {
		forbidden(c, "workspace administrator required")
		c.Abort()
		return
	}
	c.Set("workspace_role", role)
}
func accountError(c *gin.Context, err error) {
	if errors.Is(err, fp.ErrBadCredential) {
		c.JSON(400, errorResponse("invalid_credentials", "Invalid credentials or expired/used link", nil))
		return
	}
	forgeErr(c, err)
}
func (h handlers) localSession(c *gin.Context, uid string) {
	tok, err := h.deps.forge.NewLocalSession(c.Request.Context(), uid)
	if err != nil {
		internalErr(c, err)
		return
	}
	// Remove an old OIDC cookie when switching identities.
	h.writeSessionCookie(c, "", -1)
	http.SetCookie(c.Writer, &http.Cookie{Name: localCookie, Value: tok, Path: "/", HttpOnly: true, Secure: secureCookiesForOrigin(h.deps.appOrigin), SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600})
	c.JSON(200, gin.H{"ok": true})
}
func (h handlers) localLogin(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&in) != nil {
		badRequest(c, "username and password are required")
		return
	}
	uid, err := h.deps.forge.LoginLocal(c.Request.Context(), in.Username, in.Password)
	if err != nil {
		accountError(c, err)
		return
	}
	h.localSession(c, uid)
}
func (h handlers) localLink(c *gin.Context) {
	var in struct {
		Token string `json:"token"`
	}
	if c.ShouldBindJSON(&in) != nil {
		badRequest(c, "invalid link")
		return
	}
	l, err := h.deps.forge.InspectAccountLink(c.Request.Context(), in.Token)
	if err != nil {
		accountError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"kind": l.Kind, "role": l.Role, "team_id": l.TeamID, "expires_at": l.ExpiresAt, "workspace": "Local workspace"})
}
func (h handlers) localRegister(c *gin.Context) {
	var in struct {
		Token           string `json:"token"`
		Name            string `json:"name"`
		Username        string `json:"username"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if c.ShouldBindJSON(&in) != nil {
		badRequest(c, "invalid registration")
		return
	}
	if in.Password != in.ConfirmPassword {
		badRequest(c, "Passwords don't match.")
		return
	}
	uid, err := h.deps.forge.RegisterLocal(c.Request.Context(), in.Token, in.Name, in.Username, in.Password)
	if err != nil {
		accountError(c, err)
		return
	}
	h.localSession(c, uid)
}
func (h handlers) localReset(c *gin.Context) {
	var in struct {
		Token           string `json:"token"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if c.ShouldBindJSON(&in) != nil {
		badRequest(c, "invalid reset")
		return
	}
	if in.Password != in.ConfirmPassword {
		badRequest(c, "Passwords don't match.")
		return
	}
	if err := h.deps.forge.ResetLocalPassword(c.Request.Context(), in.Token, in.Password); err != nil {
		accountError(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (h handlers) localAccount(c *gin.Context) {
	if !h.localAuthEnabled() {
		c.JSON(200, gin.H{"enabled": false})
		return
	}
	if err := h.ensureLocalAdmin(c.Request.Context(), current(c)); err != nil {
		internalErr(c, err)
		return
	}
	role, err := h.deps.forge.WorkspaceRole(c.Request.Context(), current(c).ID)
	if err != nil {
		internalErr(c, err)
		return
	}
	c.JSON(200, gin.H{"enabled": true, "role": role, "workspace": "Local workspace"})
}
func (h handlers) localAdminAccounts(c *gin.Context) {
	ctx := c.Request.Context()
	members, err := h.deps.forge.AccountMembers(ctx)
	if err != nil {
		internalErr(c, err)
		return
	}
	teams, err := h.deps.forge.Teams(ctx)
	if err != nil {
		internalErr(c, err)
		return
	}
	links, err := h.deps.forge.AccountLinks(ctx)
	if err != nil {
		internalErr(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"members": members, "teams": teams, "links": links})
}
func (h handlers) localCreateTeam(c *gin.Context) {
	var in struct {
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&in) != nil {
		badRequest(c, "invalid team")
		return
	}
	t, err := h.deps.forge.CreateTeam(c.Request.Context(), in.Name)
	if err != nil {
		accountError(c, err)
		return
	}
	c.JSON(201, t)
}
func (h handlers) localCreateInvite(c *gin.Context) {
	var in struct {
		Role   string `json:"role"`
		TeamID string `json:"team_id"`
	}
	if c.ShouldBindJSON(&in) != nil {
		badRequest(c, "invalid invite")
		return
	}
	if in.Role == "" {
		in.Role = "member"
	}
	if in.Role == "admin" && c.GetString("workspace_role") != "owner" {
		forbidden(c, "only workspace owners can invite admins")
		return
	}
	h.issueLocalLink(c, "invite", in.Role, in.TeamID, "", 7*24*time.Hour)
}
func (h handlers) localCreateReset(c *gin.Context) {
	var in struct {
		UserID string `json:"user_id"`
	}
	if c.ShouldBindJSON(&in) != nil {
		badRequest(c, "invalid reset request")
		return
	}
	h.issueLocalLink(c, "reset", "member", "", in.UserID, time.Hour)
}
func (h handlers) issueLocalLink(c *gin.Context, kind, role, team, uid string, ttl time.Duration) {
	l, tok, err := h.deps.forge.NewAccountLink(c.Request.Context(), current(c).ID, kind, role, team, uid, ttl)
	if err != nil {
		accountError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	// Token in fragment: never sent in HTTP URLs, request logs, or Referer.
	path := "/join"
	if kind == "reset" {
		path = "/reset-password"
	}
	c.JSON(201, gin.H{"link": l, "url": strings.TrimRight(h.deps.appOrigin, "/") + path + "#token=" + tok})
}
func (h handlers) localRevokeLink(c *gin.Context) {
	if err := h.deps.forge.RevokeAccountLink(c.Request.Context(), c.Param("lid"), current(c).ID); err != nil {
		accountError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ipRateLimit is a per-client-IP budget for unauthenticated endpoints
// (login, register, reset). In-memory, single node: a prototype safeguard.
func ipRateLimit(perMinute int) gin.HandlerFunc {
	type window struct {
		start time.Time
		count int
	}
	var mu sync.Mutex
	windows := map[string]window{}
	return func(c *gin.Context) {
		key, now := c.ClientIP(), time.Now()
		mu.Lock()
		if len(windows) > 10000 {
			for k, v := range windows {
				if now.Sub(v.start) >= time.Minute {
					delete(windows, k)
				}
			}
		}
		w := windows[key]
		if now.Sub(w.start) >= time.Minute {
			w = window{start: now}
		}
		allowed := w.count < perMinute
		if allowed {
			w.count++
			windows[key] = w
		}
		mu.Unlock()
		if !allowed {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, errorResponse("rate_limited", "Too many attempts. Try again in a minute.", nil))
			return
		}
		c.Next()
	}
}
