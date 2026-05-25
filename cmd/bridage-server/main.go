package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nuts/bridage/internal/admin"
	"github.com/nuts/bridage/internal/config"
	"github.com/nuts/bridage/internal/httpserver"
	"github.com/nuts/bridage/internal/logger"
	"github.com/nuts/bridage/internal/publicapi"
	"github.com/nuts/bridage/internal/relay"
	"github.com/nuts/bridage/internal/store/postgres"
	migrations "github.com/nuts/bridage/migrations"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("config error: " + err.Error())
	}

	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		panic("logger error: " + err.Error())
	}
	defer log.Sync() //nolint:errcheck

	// Connect DB
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("db connect failed", zap.Error(err))
	}
	defer pool.Close()

	// Run migrations
	if err := postgres.RunMigrations(cfg.DatabaseURL, migrations.FS); err != nil {
		log.Fatal("migrations failed", zap.Error(err))
	}
	log.Info("migrations applied")

	// Stores
	provStore := postgres.NewProviderStore(pool)
	modelStore := postgres.NewModelStore(pool)
	apiKeyStore := postgres.NewAPIKeyStore(pool)
	usageStore := postgres.NewUsageStore(pool)
	adminStore := postgres.NewAdminStore(pool)
	auditStore := postgres.NewAuditStore(pool)

	// Bootstrap admin if configured
	if cfg.AdminBootstrap != "" {
		n, _ := adminStore.Count(ctx)
		if n == 0 {
			hashed, hashErr := bcrypt.GenerateFromPassword([]byte(cfg.AdminBootstrap), bcrypt.DefaultCost)
			if hashErr != nil {
				log.Warn("bootstrap admin hash failed", zap.Error(hashErr))
			} else if _, err := adminStore.Create(ctx, cfg.AdminBootstrap, string(hashed)); err != nil {
				log.Warn("bootstrap admin failed", zap.Error(err))
			} else {
				log.Info("bootstrap admin created", zap.String("username", cfg.AdminBootstrap))
			}
		}
	}

	// Services
	relaySvc := relay.NewService(
		apiKeyStore, provStore, modelStore, usageStore,
		cfg.MasterKey,
		cfg.ProviderTimeout,
		cfg.ProviderRetries,
		log,
	)

	pubHandler := publicapi.NewHandler(relaySvc, modelStore, usageStore)
	admHandler := admin.NewHandler(
		adminStore, provStore, modelStore, apiKeyStore, usageStore, auditStore,
		cfg.MasterKey, cfg.JWTSecret, cfg.JWTExpiry, log,
	)

	router := httpserver.NewRouter(relaySvc, pubHandler, admHandler, strings.Split(cfg.CORSOrigins, ","), log)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting", zap.String("addr", cfg.ListenAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-quit
	log.Info("shutting down...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
	log.Info("server stopped")
}
