package middleware

import (
	"net/http"
	"strings"

	"biodays/server/internal/security"
	"biodays/server/internal/service"

	"github.com/gin-gonic/gin"
)

const adminTokenAuthKey = "biodays.admin.token"

// bearerCredential extracts the token from an "Authorization: Bearer <token>"
// header and reports whether the header carries the Bearer scheme at all.
func bearerCredential(header string) (string, bool) {
	scheme, credential, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(strings.TrimSpace(scheme), "Bearer") {
		return "", false
	}
	return strings.TrimSpace(credential), true
}

// AdminAPIAuth authenticates story-management requests either by the static
// Bearer token (scripts and curl) or by the regular admin cookie session
// (the web UI). Bearer requests skip the Origin and CSRF checks: the token is
// an explicit, non-ambient credential, so cross-site request forgery does not
// apply and non-browser clients send no Origin header. Session requests keep
// the same origin enforcement as the rest of the admin surface.
func AdminAPIAuth(admin *service.AdminService, token string, allowedOrigins []string) gin.HandlerFunc {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if normalized := normalizeOrigin(origin); normalized != "" {
			origins[normalized] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		if credential, ok := bearerCredential(c.GetHeader("Authorization")); ok {
			if token == "" || !security.ConstantTimeEqual(credential, token) {
				abortJSON(c, http.StatusUnauthorized, "UNAUTHENTICATED", "admin api token is invalid")
				return
			}
			c.Set(adminTokenAuthKey, true)
			c.Header("Cache-Control", "no-store")
			c.Next()
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions {
			origin := normalizeOrigin(c.GetHeader("Origin"))
			if origin == "" {
				abortJSON(c, http.StatusForbidden, "FORBIDDEN", "request origin is required")
				return
			}
			if _, allowed := origins[origin]; !allowed {
				abortJSON(c, http.StatusForbidden, "FORBIDDEN", "request origin is not allowed")
				return
			}
		}
		AdminAuth(admin)(c)
	}
}

// RequireCSRFUnlessToken keeps the CSRF header requirement for
// session-authenticated writes and skips it for token-authenticated ones.
func RequireCSRFUnlessToken(admin *service.AdminService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if value, exists := c.Get(adminTokenAuthKey); exists {
			if enabled, ok := value.(bool); ok && enabled {
				c.Next()
				return
			}
		}
		RequireCSRF(admin)(c)
	}
}
