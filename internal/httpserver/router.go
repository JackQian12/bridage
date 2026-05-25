package httpserver

import (
	"io/fs"
	"net/http"
	"strings"
	"sync"
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
	maxBodyBytes int64,
	log *zap.Logger,
) *gin.Engine {
	r := gin.New()

	// Middleware
	r.Use(requestID())
	r.Use(ginZapLogger(log))
	r.Use(gin.Recovery())
	r.Use(securityHeaders())
	r.Use(bodyLimit(maxBodyBytes))
	r.Use(corsMiddleware(corsOrigins))

	// ── Public unauthenticated ──────────────────────────────────────────────
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Admin bootstrap (only works if no admin exists; requires X-Bootstrap-Token)
	r.POST("/admin/bootstrap", adm.BootstrapAdmin)
	// Login with per-IP rate limiting
	r.POST("/admin/login", loginRateLimit(), adm.Login)

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
			// Authorization header intentionally omitted to prevent credential leakage.
		)
	}
}

// securityHeaders adds defensive HTTP response headers to every response.
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		// X-XSS-Protection is legacy; modern browsers use CSP.
		// Setting to "0" disables buggy browser XSS auditors.
		c.Header("X-XSS-Protection", "0")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// Strict-Transport-Security should only be set when TLS is active.
		// Uncomment and configure via a HSTS_ENABLED env var in production.
		// c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		c.Next()
	}
}

// bodyLimit rejects requests whose body exceeds maxBytes, protecting against
// resource-exhaustion (DoS) via oversized payloads.
func bodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "request body too large",
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// ─── Login rate limiter ────────────────────────────────────────────────────────

const (
	loginMaxAttempts = 10
	loginWindow      = 10 * time.Minute
)

type loginBucket struct {
	count   int
	resetAt time.Time
}

var (
	loginBuckets   = make(map[string]*loginBucket)
	loginBucketsMu sync.Mutex
)

// loginRateLimit enforces a per-IP sliding-window limit on /admin/login.
// Exceeding loginMaxAttempts within loginWindow returns 429.
func loginRateLimit() gin.HandlerFunc {
	// Background cleanup of expired buckets.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			now := time.Now()
			loginBucketsMu.Lock()
			for ip, b := range loginBuckets {
				if now.After(b.resetAt) {
					delete(loginBuckets, ip)
				}
			}
			loginBucketsMu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		loginBucketsMu.Lock()
		b, ok := loginBuckets[ip]
		if !ok || now.After(b.resetAt) {
			loginBuckets[ip] = &loginBucket{count: 1, resetAt: now.Add(loginWindow)}
			loginBucketsMu.Unlock()
			c.Next()
			return
		}
		if b.count >= loginMaxAttempts {
			loginBucketsMu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many login attempts, please try again later",
			})
			return
		}
		b.count++
		loginBucketsMu.Unlock()
		c.Next()
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
