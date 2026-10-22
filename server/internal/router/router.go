package router

import (
	"fmt"
	"log/slog"
	"slices"
	"time"

	_ "biodays/server/docs"
	"biodays/server/internal/config"
	"biodays/server/internal/handler"
	"biodays/server/internal/middleware"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func New(cfg config.Config, st *store.Store, logger *slog.Logger) (*gin.Engine, error) {
	return NewWithClock(cfg, st, logger, time.Now)
}

func NewWithClock(cfg config.Config, st *store.Store, logger *slog.Logger, now func() time.Time) (*gin.Engine, error) {
	engine := gin.New()
	var trustedProxies []string
	if len(cfg.TrustedProxies) > 0 {
		trustedProxies = cfg.TrustedProxies
	}
	if err := engine.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	limiter := middleware.NewRateLimiter(cfg.RateLimit)

	// Chain mirrors SPEC 7.3: Recovery → request log → rate limit → CORS → routes.
	engine.Use(middleware.Recovery(logger))
	engine.Use(middleware.RequestLog(logger))
	engine.Use(middleware.RateLimit(limiter))
	engine.Use(cors.New(corsConfig(cfg.CORSOrigins)))

	healthHandler := handler.HealthHandler{}
	engine.GET("/healthz", healthHandler.Health)

	readyHandler := handler.NewReadyHandler(st, logger)
	engine.GET("/readyz", readyHandler.Ready)

	svc := service.NewStoryServiceWithClock(st, now)
	storyHandler := handler.NewStoryHandler(svc, logger)

	v1 := engine.Group("/api/v1")
	v1.Use(middleware.CachePublic(300))
	v1.GET("/today", storyHandler.Today)
	v1.GET("/stories", storyHandler.List)
	v1.GET("/stories/random", storyHandler.Random)
	v1.GET("/stories/:id", storyHandler.Get)
	v1.GET("/tags", storyHandler.Tags)
	v1.GET("/meta", storyHandler.Meta)

	engine.GET("/swagger/*any", withSwaggerTheme(ginSwagger.WrapHandler(swaggerFiles.Handler)))
	return engine, nil
}

func corsConfig(origins []string) cors.Config {
	cfg := cors.Config{
		AllowMethods:     []string{"GET", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Cache-Control", "X-RateLimit-Limit", "Retry-After"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
	if slices.Contains(origins, "*") {
		cfg.AllowAllOrigins = true
		return cfg
	}
	cfg.AllowOrigins = origins
	return cfg
}
