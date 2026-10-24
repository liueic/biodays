package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"biodays/server/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	AdminCookieName  = "biodays_admin"
	AdminCSRFHeader  = "X-CSRF-Token"
	adminIdentityKey = "biodays.admin.identity"
)

func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		c.Next()
	}
}

// PrivateHeaders keeps sensitive pages out of search indexes and strips referrers.
func PrivateHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Robots-Tag", "noindex, nofollow")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	}
}

func AdminAuth(admin *service.AdminService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if admin == nil {
			abortJSON(c, http.StatusServiceUnavailable, "FEEDBACK_UNAVAILABLE", "feedback is not enabled")
			return
		}
		raw, err := c.Cookie(AdminCookieName)
		if err != nil || raw == "" {
			abortJSON(c, http.StatusUnauthorized, "UNAUTHENTICATED", "admin login required")
			return
		}
		identity, err := admin.Authenticate(c.Request.Context(), raw)
		if err != nil {
			abortJSON(c, http.StatusUnauthorized, "UNAUTHENTICATED", "admin session is invalid or expired")
			return
		}
		c.Set(adminIdentityKey, identity)
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

func RequireCSRF(admin *service.AdminService) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := AdminIdentity(c)
		if !ok {
			abortJSON(c, http.StatusUnauthorized, "UNAUTHENTICATED", "admin login required")
			return
		}
		if admin == nil || !admin.VerifyCSRF(identity, c.GetHeader(AdminCSRFHeader)) {
			abortJSON(c, http.StatusForbidden, "CSRF_INVALID", "CSRF token is missing or invalid")
			return
		}
		c.Next()
	}
}

// RequireOrigin blocks state-changing cross-site requests. Same-origin fetches
// from the Next proxy carry the site origin; requests without Origin are rejected.
func RequireOrigin(allowed []string) gin.HandlerFunc {
	set := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		if normalized := normalizeOrigin(origin); normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		origin := normalizeOrigin(c.GetHeader("Origin"))
		if origin == "" {
			abortJSON(c, http.StatusForbidden, "FORBIDDEN", "request origin is required")
			return
		}
		if _, ok := set[origin]; !ok {
			abortJSON(c, http.StatusForbidden, "FORBIDDEN", "request origin is not allowed")
			return
		}
		c.Next()
	}
}

func normalizeOrigin(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func AdminIdentity(c *gin.Context) (service.AdminIdentity, bool) {
	value, ok := c.Get(adminIdentityKey)
	if !ok {
		return service.AdminIdentity{}, false
	}
	identity, ok := value.(service.AdminIdentity)
	return identity, ok
}

func abortJSON(c *gin.Context, status int, code, message string) {
	c.Header("Cache-Control", "no-store")
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
