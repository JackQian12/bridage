package httpserver

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuts/bridage/internal/admin"
	"github.com/nuts/bridage/internal/publicapi"
	"github.com/nuts/bridage/internal/relay"
	web "github.com/nuts/bridage/web"
	"go.uber.org/zap"
)

// NewRouter builds and returns the main Gin engine.
func NewRouter(
	relayService *relay.Service,
	pub *publicapi.Handler,
	adm *admin.Handler,
	corsOrigins []string,
	log *zap.Logger,
) *gin.Engine {
	r := gin.New()

	// Middleware
	r.Use(requestID())
	r.Use(ginZapLogger(log))
	r.Use(gin.Recovery())
	r.Use(corsMiddleware(corsOrigins))

	// ── Public unauthenticated ──────────────────────────────────────────────
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Admin bootstrap (only works if no admin exists)
	r.POST("/admin/bootstrap", adm.BootstrapAdmin)
	r.POST("/admin/login", adm.Login)

	// ── Admin authenticated routes ─────────────────────────────────────────
	adminGroup := r.Group("/admin")
	adminGroup.Use(adm.ValidateJWT)
	{
		// Presets
		adminGroup.GET("/presets", adm.ListPresets)
		adminGroup.POST("/presets/:slug/bootstrap", adm.BootstrapPreset)

		// Providers
		adminGroup.GET("/providers", adm.ListProviders)
		adminGroup.POST("/providers", adm.CreateProvider)
		adminGroup.GET("/providers/:id", adm.GetProvider)
		adminGroup.PUT("/providers/:id", adm.UpdateProvider)
		adminGroup.DELETE("/providers/:id", adm.DeleteProvider)

		// Models
		adminGroup.GET("/models", adm.ListModels)
		adminGroup.POST("/models", adm.CreateModel)
		adminGroup.GET("/models/:id", adm.GetModel)
		adminGroup.PUT("/models/:id", adm.UpdateModel)
		adminGroup.DELETE("/models/:id", adm.DeleteModel)

		// API Keys
		adminGroup.GET("/keys", adm.ListAPIKeys)
		adminGroup.POST("/keys", adm.CreateAPIKey)
		adminGroup.GET("/keys/:id", adm.GetAPIKey)
		adminGroup.PUT("/keys/:id", adm.UpdateAPIKey)
		adminGroup.DELETE("/keys/:id", adm.DeleteAPIKey)

		// Usage
		adminGroup.GET("/keys/:id/usage", adm.GetUsage)
	}

	// ── Public /v1 relay routes ─────────────────────────────────────────────
	v1 := r.Group("/v1")
	v1.Use(downstreamKeyAuth(relayService))
	{
		v1.GET("/models", pub.ListModels)
		v1.POST("/chat/completions", pub.ChatCompletions)
		v1.POST("/responses", pub.Responses)
		v1.POST("/embeddings", pub.Embeddings)
		v1.POST("/images/generations", pub.Images)

		// Account info (key-owner self-service)
		v1.GET("/account/key", pub.GetAccountKey)
		v1.GET("/account/usage", pub.GetAccountUsage)
	}

	// ── Web UIs (static file serving) ──────────────────────────────────────
	adminStaticFS, _ := fs.Sub(web.FS, "admin/static")
	r.GET("/web/admin", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/web/admin/")
	})
	r.StaticFS("/web/admin", http.FS(adminStaticFS))

	userStaticFS, _ := fs.Sub(web.FS, "user/static")
	r.GET("/web/user", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/web/user/")
	})
	r.StaticFS("/web/user", http.FS(userStaticFS))

	return r
}

// ─── Middleware ────────────────────────────────────────────────────────────────

func downstreamKeyAuth(svc *relay.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if len(raw) < 8 || strings.ToLower(raw[:7]) != "bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"type": "unauthorized", "message": "missing or invalid authorization header",
			}})
			return
		}
		token := raw[7:]
		if !strings.HasPrefix(token, "brg_") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"type": "unauthorized", "message": "invalid api key format",
			}})
			return
		}
		result, err := svc.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"type": "unauthorized", "message": err.Error(),
			}})
			return
		}
		c.Set(publicapi.ContextAPIKey, result.Key)
		c.Next()
	}
}

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = generateRequestID()
		}
		c.Set("request_id", id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

func generateRequestID() string {
	// simple timestamp-based ID; uuid would require importing uuid here
	return "req_" + strings.ReplaceAll(strings.ReplaceAll(
		time.Now().UTC().Format("20060102T150405.999999999Z"), ".", ""), ":", "")
}

func ginZapLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("request_id", c.GetString("request_id")),
		)
	}
}

func corsMiddleware(origins []string) gin.HandlerFunc {
	allowAll := len(origins) == 0 || (len(origins) == 1 && origins[0] == "*")
	allowedSet := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowedSet[o] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowAll {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if _, ok := allowedSet[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-Id")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
