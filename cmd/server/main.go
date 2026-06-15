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
	// 收到 SIGINT/SIGTERM 时取消根 context（ECS 停容器发的是 SIGTERM）。
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
	// 指标。
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	metrics := observability.NewMetrics(reg)

	// Sample DB pool stats in the background.
	// 后台定时采集数据库连接池的统计指标。
	go metrics.SamplePool(ctx, pool, 10*time.Second)

	// Wire layers: repositories -> services -> handlers.
	// 装配各层：repository -> service -> handler。
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

	// Public server: business endpoints only.
	// 公开 server：只挂载业务接口。
	public := server.New(
		server.WithHostPorts(cfg.Addr),
		server.WithExitWaitTime(cfg.ShutdownTimeout),
	)
	api.RegisterPublicRoutes(public, metrics, log, handlers)

	// Internal admin server: metrics and probes, kept off the public interface.
	// 内部 admin server：指标与健康探针，不对外暴露。
	admin := server.New(
		server.WithHostPorts(cfg.MetricsAddr),
		server.WithExitWaitTime(cfg.ShutdownTimeout),
	)
	api.RegisterAdminRoutes(admin, reg, metrics, log, handlers)

	// Serve both in the background so we can manage shutdown ourselves.
	// 两个 server 都在后台启动，以便我们自己控制关闭流程。
	go func() {
		if err := public.Run(); err != nil {
			log.Error("public server stopped", "error", err)
		}
	}()
	go func() {
		if err := admin.Run(); err != nil {
			log.Error("admin server stopped", "error", err)
		}
	}()
	log.Info("servers started", "addr", cfg.Addr, "metrics_addr", cfg.MetricsAddr)

	// Block until a termination signal arrives.
	// 阻塞，直到收到终止信号。
	<-ctx.Done()
	log.Info("shutdown signal received, draining")

	// Flip readiness to 503 so the load balancer stops sending new traffic,
	// then shut both servers down within the configured budget.
	// 把就绪探针置为 503，让负载均衡停止转发新流量，
	// 然后在配置的超时预算内关闭两个 server。
	health.SetNotReady()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := public.Shutdown(shutdownCtx); err != nil {
		log.Error("public graceful shutdown failed", "error", err)
	}
	if err := admin.Shutdown(shutdownCtx); err != nil {
		log.Error("admin graceful shutdown failed", "error", err)
	}
	log.Info("servers stopped cleanly")
}
