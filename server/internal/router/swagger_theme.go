package router

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// swaggerWin98CSS replaces gin-swagger's default index.css so the docs page matches the
// Windows 98 look of the web frontend.
//
//go:embed assets/swagger-win98.css
var swaggerWin98CSS []byte

// withSwaggerTheme serves the Win98 stylesheet for index.css and delegates everything else
// (HTML, JS bundles, doc.json) to the stock gin-swagger handler.
func withSwaggerTheme(next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && strings.HasSuffix(c.Request.URL.Path, "/index.css") {
			c.Data(http.StatusOK, "text/css; charset=utf-8", swaggerWin98CSS)
			return
		}
		next(c)
	}
}
