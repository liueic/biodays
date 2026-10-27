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
	"biodays/server/internal/settings"
	"biodays/server/internal/store"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Deps carries the optional feedback subsystem. When nil the story API keeps
// working and feedback routes report FEEDBACK_UNAVAILABLE.
type Deps struct {
	Feedback        *store.FeedbackStore
	FeedbackService *service.FeedbackService
	AdminService    *service.AdminService
	// SettingsManager serves the admin settings API; nil keeps the routes
	// off (feedback subsystem not started).
	SettingsManager *settings.Manager
}

func New(cfg config.Config, st *store.Store, logger *slog.Logger) (*gin.Engine, error) {
	return NewWithClockAndDeps(cfg, st, logger, time.Now, Deps{})
}

func NewWithClock(cfg config.Config, st *store.Store, logger *slog.Logger, now func() time.Time) (*gin.Engine, error) {
	return NewWithClockAndDeps(cfg, st, logger, now, Deps{})
}

func NewWithDeps(cfg config.Config, st *store.Store, logger *slog.Logger, now func() time.Time, deps Deps) (*gin.Engine, error) {
	return NewWithClockAndDeps(cfg, st, logger, now, deps)
}

func NewWithClockAndDeps(cfg config.Config, st *store.Store, logger *slog.Logger, now func() time.Time, deps Deps) (*gin.Engine, error) {
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

	pingers := []handler.Pinger{st}
	if deps.Feedback != nil {
		pingers = append(pingers, deps.Feedback)
	}
	readyHandler := handler.NewReadyHandlers(logger, pingers...)
	engine.GET("/readyz", readyHandler.Ready)

	svc := service.NewStoryServiceWithClock(st, now)
	storyHandler := handler.NewStoryHandler(svc, logger)

	v1 := engine.Group("/api/v1")

	// Story routes keep the public cache; feedback and admin must never inherit it.
	stories := v1.Group("")
	stories.Use(middleware.CachePublic(300))
	stories.GET("/today", storyHandler.Today)
	stories.GET("/stories", storyHandler.List)
	stories.GET("/stories/random", storyHandler.Random)
	stories.GET("/stories/:id", storyHandler.Get)
	stories.GET("/tags", storyHandler.Tags)
	stories.GET("/meta", storyHandler.Meta)
	stories.POST("/history", storyHandler.History)

	allowedOrigins := append([]string{cfg.SiteURL}, cfg.CORSOrigins...)

	feedbackHandler := handler.NewFeedbackHandler(deps.FeedbackService, logger)
	feedback := v1.Group("/feedback")
	feedback.Use(middleware.NoStore())
	feedback.GET("/options", feedbackHandler.Options)
	feedback.POST("/subscriptions/confirm", feedbackHandler.ConfirmSubscription)
	feedback.POST("/subscriptions/resend", feedbackHandler.ResendConfirmation)
	feedback.POST("/subscriptions/unsubscribe", feedbackHandler.Unsubscribe)
	v1.POST("/feedback", middleware.NoStore(), middleware.RequirePublicOrigin(allowedOrigins), feedbackHandler.Submit)

	adminHandler := handler.NewAdminHandler(deps.AdminService, deps.FeedbackService, logger, secureCookie(cfg.SiteURL))
	mailHandler := handler.NewMailHandler(deps.FeedbackService, logger)
	settingsHandler := handler.NewSettingsHandler(deps.SettingsManager, deps.Feedback, logger)
	admin := v1.Group("/admin")
	admin.Use(middleware.PrivateHeaders())
	admin.Use(middleware.RequireOrigin(allowedOrigins))
	admin.POST("/login", adminHandler.Login)

	authed := admin.Group("")
	authed.Use(middleware.AdminAuth(deps.AdminService))
	authed.GET("/me", adminHandler.Me)
	authed.GET("/feedback", adminHandler.ListFeedback)
	authed.GET("/feedback/:id", adminHandler.GetFeedback)
	authed.GET("/mail/status", mailHandler.Status)
	authed.GET("/mail/outbox", mailHandler.Outbox)
	authed.GET("/settings", settingsHandler.GetSettings)
	authed.GET("/settings/audit", settingsHandler.ListSettingsAudit)

	writes := authed.Group("")
	writes.Use(middleware.RequireCSRF(deps.AdminService))
	writes.POST("/logout", adminHandler.Logout)
	writes.PATCH("/feedback/:id", adminHandler.UpdateFeedback)
	writes.DELETE("/feedback/:id", adminHandler.DeleteFeedback)
	writes.POST("/mail/test", mailHandler.SendTest)
	writes.POST("/mail/outbox/:id/retry", mailHandler.Retry)
	writes.PATCH("/settings", settingsHandler.UpdateSettings)

	// Story management accepts either the static API token (Bearer, for
	// scripts) or the admin cookie session (web UI); the group is mounted
	// separately so Bearer requests are not subject to RequireOrigin.
	storyAdminHandler := handler.NewStoryAdminHandler(
		service.NewStoryAdminService(st, now), svc, logger)
	storyAdmin := v1.Group("/admin/stories")
	storyAdmin.Use(middleware.PrivateHeaders())
	storyAdmin.Use(middleware.AdminAPIAuth(deps.AdminService, cfg.AdminAPIToken, allowedOrigins))
	storyAdmin.GET("", storyAdminHandler.List)
	storyAdmin.GET("/:id", storyAdminHandler.Get)
	storyWrites := storyAdmin.Group("")
	storyWrites.Use(middleware.RequireCSRFUnlessToken(deps.AdminService))
	storyWrites.POST("", storyAdminHandler.Upsert)
	storyWrites.PATCH("/:id", storyAdminHandler.Update)
	storyWrites.DELETE("/:id", storyAdminHandler.Delete)

	engine.GET("/swagger/*any", withSwaggerTheme(ginSwagger.WrapHandler(swaggerFiles.Handler)))
	return engine, nil
}

func secureCookie(siteURL string) bool {
	return len(siteURL) >= 5 && siteURL[:5] == "https"
}

func corsConfig(origins []string) cors.Config {
	cfg := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "X-CSRF-Token", "Authorization"},
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
