package worker

import (
	"log/slog"

	"github.com/hibiken/asynq"
)

// Server asynq 任务服务（worker 与 API 同进程运行）。
type Server struct {
	srv *asynq.Server
	mux *asynq.ServeMux
}

// NewServer 创建 asynq server 并注册任务处理器。
func NewServer(redisAddr string, concurrency int, processor *Processor) *Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: concurrency,
		},
	)
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskResumeProcess, processor.HandleResumeProcess)
	mux.HandleFunc(TaskJobRescore, processor.HandleJobRescore)
	return &Server{srv: srv, mux: mux}
}

// Start 启动 worker（非阻塞）。
func (s *Server) Start() {
	if err := s.srv.Start(s.mux); err != nil {
		slog.Error("asynq server start failed", "error", err)
	}
	slog.Info("asynq worker started")
}

// Shutdown 优雅停止 worker。
func (s *Server) Shutdown() {
	s.srv.Shutdown()
	slog.Info("asynq worker stopped")
}
