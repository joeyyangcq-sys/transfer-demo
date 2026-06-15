package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/joeyyang/internal-transfers/internal/api"
	"github.com/joeyyang/internal-transfers/internal/config"
	"github.com/joeyyang/internal-transfers/internal/observability"
	"github.com/joeyyang/internal-transfers/internal/platform/postgres"
	"github.com/joeyyang/internal-transfers/internal/repository"
	"github.com/joeyyang/internal-transfers/internal/service"
)

func main() {
	log := observability.NewLogger(os.Getenv("LOG_LEVEL"))

	cfg, err := config.Load()
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}

	// Root context cancelled on SIGINT/SIGTERM (ECS sends SIGTERM).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.RunMigrations {
		if err := postgres.Migrate(ctx, pool); err != nil {
			log.Error("run migrations", "error", err)
			os.Exit(1)
		}
		log.Info("migrations applied")
	}

	// Metrics.
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	metrics := observability.NewMetrics(reg)

	// Sample DB pool stats in the background.
	go metrics.SamplePool(ctx, pool, 10*time.Second)

	// Wire layers: repositories -> services -> handlers.
	accountRepo := repository.NewAccountRepository()
	transferRepo := repository.NewTransferRepository()
	txManager := repository.NewTxManager(pool)

	accountSvc := service.NewAccountService(pool, accountRepo)
	transferSvc := service.NewTransferService(pool, txManager, accountRepo, transferRepo)

	resp := api.NewResponder(metrics, log)
	health := api.NewHealthHandler(pool)
	handlers := api.Handlers{
		Account:     api.NewAccountHandler(accountSvc, resp, metrics),
		Transaction: api.NewTransactionHandler(transferSvc, resp, metrics),
		Health:      health,
	}

	h := server.New(
		server.WithHostPorts(cfg.Addr),
		server.WithExitWaitTime(cfg.ShutdownTimeout),
	)
	api.RegisterRoutes(h, reg, metrics, log, handlers)

	// Serve in the background so we can manage shutdown ourselves.
	go func() {
		if err := h.Run(); err != nil {
			log.Error("server stopped", "error", err)
		}
	}()
	log.Info("server started", "addr", cfg.Addr)

	// Block until a termination signal arrives.
	<-ctx.Done()
	log.Info("shutdown signal received, draining")

	// Flip readiness to 503 so the load balancer stops sending new traffic,
	// then shut the server down within the configured budget.
	health.SetNotReady()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := h.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}
	log.Info("server stopped cleanly")
}
