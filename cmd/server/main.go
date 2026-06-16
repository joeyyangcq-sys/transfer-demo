package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/joeyyang/transfer-demo/internal/api"
	"github.com/joeyyang/transfer-demo/internal/config"
	"github.com/joeyyang/transfer-demo/internal/observability"
	"github.com/joeyyang/transfer-demo/internal/platform/postgres"
	"github.com/joeyyang/transfer-demo/internal/repository"
	"github.com/joeyyang/transfer-demo/internal/service"
)

func main() {
	log, closeLog, err := observability.NewLogger(os.Getenv("LOG_LEVEL"), os.Getenv("LOG_FILE"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = closeLog() }()

	cfg, err := config.Load()
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}

	// Root context cancelled on SIGINT/SIGTERM (ECS sends SIGTERM).
	// 收到 SIGINT/SIGTERM 时取消根 context（ECS 停容器发的是 SIGTERM）。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Metrics. Built before the pool so the query tracer can report into them.
	// 指标。在建池之前构建，以便查询 tracer 能向其上报。
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	metrics := observability.NewMetrics(reg)

	// Trace every query's latency, labelled by operation.
	// 为每条查询计时，按操作名打标签。
	tracer := postgres.NewQueryTracer(func(op string, sec float64) {
		metrics.DBQueryDuration.WithLabelValues(op).Observe(sec)
	})

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, cfg.DBMaxConns, tracer)
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
