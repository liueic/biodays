package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CachePublic(seconds int) gin.HandlerFunc {
	value := fmt.Sprintf("public, max-age=%d", seconds)
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			c.Header("Cache-Control", value)
		}
		c.Next()
	}
}
