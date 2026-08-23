// Package main 服务入口：配置加载 → slog → DB（自动 goose Up）→ Redis → MinIO →
// asynq client+server → Gin 路由 → 优雅退出。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/ai"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/config"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/db"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/file"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/handler"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/middleware"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/repo"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/router"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/service"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/worker"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. 配置
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// 2. slog（JSON handler）
	level := slog.LevelInfo
	if cfg.AppEnv == "dev" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
	slog.Info("config loaded", "app_env", cfg.AppEnv, "http_addr", cfg.HTTPAddr, "ai_topn", cfg.AITopN)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 3. 数据库 + 自动迁移
	if err := db.RunMigrations(ctx, cfg.DatabaseURL); err != nil {
		return err
	}
	slog.Info("database migrated")
	pg, err := repo.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pg.Close()
	repos := pg.NewRepos()

	// 4. Redis（验证码 + asynq）
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return err
	}
	defer rdb.Close()

	// 5. MinIO
	storage, err := file.NewMinioStorage(file.Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Bucket:    cfg.S3Bucket,
		UseSSL:    cfg.S3UseSSL,
	})
	if err != nil {
		return err
	}
	if err := storage.EnsureBucket(ctx); err != nil {
		return err
	}
	slog.Info("minio ready", "endpoint", cfg.S3Endpoint, "bucket", cfg.S3Bucket)

	// 6. AI 客户端（缺 Key 自动降级）
	aiClient, err := ai.New(ctx, ai.Config{
		DeepSeekAPIKey:     cfg.DeepSeekAPIKey,
		DeepSeekBaseURL:    cfg.DeepSeekBaseURL,
		DeepSeekModel:      cfg.DeepSeekModel,
		SiliconFlowAPIKey:  cfg.SiliconFlowAPIKey,
		SiliconFlowBaseURL: cfg.SiliconFlowBaseURL,
		EmbeddingModel:     cfg.EmbeddingModel,
		EmbeddingDim:       cfg.EmbeddingDim,
	})
	if err != nil {
		return err
	}
	chatOK, embedOK := ai.Capabilities(aiClient)
	slog.Info("ai client ready", "chat_enabled", chatOK, "embed_enabled", embedOK,
		"chat_model", cfg.DeepSeekModel, "embedding_model", cfg.EmbeddingModel)

	// 7. asynq client + server（worker 与 API 同进程）
	queue := worker.NewQueue(cfg.RedisAddr)
	defer queue.Close()
	processor := &worker.Processor{
		Apps:    repos.Applications,
		Jobs:    repos.Jobs,
		AI:      aiClient,
		Storage: storage,
		Queue:   queue,
		Config: worker.ProcessorConfig{
			AILLMW:        cfg.AILLMW,
			AISemanticW:   cfg.AISemanticW,
			AIRuleW:       cfg.AIRuleW,
			AITopN:        cfg.AITopN,
			DeepSeekModel: cfg.DeepSeekModel,
		},
	}
	asynqServer := worker.NewServer(cfg.RedisAddr, cfg.AsyncWorkers, processor)
	asynqServer.Start()
	defer asynqServer.Shutdown()

	// 8. 服务与处理器
	authSvc := service.NewAuthService(repos.Users, repos.Candidates, repos.Applications, cfg.JWTSecret, cfg.JWTTTL, rdb)
	deptSvc := service.NewDepartmentService(repos.Departments)
	jobSvc := service.NewJobService(repos.Jobs, repos.Applications, repos.Audit, queue, storage)
	applySvc := service.NewApplyService(repos.Jobs, repos.Candidates, repos.Applications, storage, queue, repos.Audit)

	r := router.New(router.Deps{
		JWTSecret:    cfg.JWTSecret,
		PublicJobs:   handler.NewPublicJobHandler(jobSvc, applySvc, deptSvc),
		PublicAuth:   handler.NewPublicAuthHandler(authSvc),
		InternalAuth: handler.NewInternalAuthHandler(authSvc, deptSvc),
		InternalJobs: handler.NewInternalJobHandler(jobSvc),
		InternalApps: handler.NewInternalApplicationHandler(jobSvc),
	})
	r.Use(middleware.CORS(cfg.CORSOrigins))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 9. 启动与优雅退出
	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown", "error", err)
	}
	slog.Info("http server stopped")
	return nil
}
