// Package worker 实现 asynq 任务队列（Redis 驱动）与 AI 匹配流水线。
//
// 任务：
//   - resume:process：单份投递的完整匹配流水线（文本提取 → LLM 解析 → 硬性检查/规则分 → 向量化 → LLM 评委 → 综合分落库）
//   - job:rescore：岗位变更后的批量重算（重新生成岗位向量 + 全量重新入队）
//
// 防重复：resume:process 使用 asynq.Unique(24h)，唯一键由任务类型+载荷决定，
// 载荷中的 Version 取岗位 updated_at 的 unix 秒数 —— 岗位内容变更后版本号变化，
// 即可对同一投递重新入队（重算），未变更时 24 小时内重复入队被去重。
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

// 任务类型常量。
const (
	TaskResumeProcess = "resume:process"
	TaskJobRescore    = "job:rescore"
)

// 去重 TTL 常量。
const (
	// UniqueResumeTTL 单份投递 24 小时内不重复处理。
	UniqueResumeTTL = 24 * time.Hour
	// UniqueRescoreTTL job:rescore 10 分钟内不重复触发。
	UniqueRescoreTTL = 10 * time.Minute
)

// PayloadResumeProcess resume:process 任务载荷。
type PayloadResumeProcess struct {
	ApplicationID string `json:"application_id"`
	// Version 岗位更新版本（updated_at unix），参与唯一键实现「变更后可重跑」。
	Version int64 `json:"version"`
}

// PayloadJobRescore job:rescore 任务载荷。
type PayloadJobRescore struct {
	JobID string `json:"job_id"`
}

// Queue asynq 任务入队器。
type Queue struct {
	client *asynq.Client
}

// NewQueue 创建入队器（连接 Redis）。
func NewQueue(redisAddr string) *Queue {
	return &Queue{client: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})}
}

// Close 关闭客户端连接。
func (q *Queue) Close() error { return q.client.Close() }

// EnqueueResumeProcess 入队简历处理任务；version 为岗位更新版本。
func (q *Queue) EnqueueResumeProcess(ctx context.Context, applicationID string, version int64) error {
	payload, err := json.Marshal(PayloadResumeProcess{ApplicationID: applicationID, Version: version})
	if err != nil {
		return fmt.Errorf("worker: marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskResumeProcess, payload, asynq.Unique(UniqueResumeTTL), asynq.MaxRetry(3))
	_, err = q.client.EnqueueContext(ctx, task)
	if err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			slog.Warn("worker: resume:process duplicate task skipped",
				"application_id", applicationID, "version", version)
			return nil
		}
		return fmt.Errorf("worker: enqueue resume:process: %w", err)
	}
	slog.Info("worker: resume:process enqueued", "application_id", applicationID, "version", version)
	return nil
}

// EnqueueJobRescore 入队岗位重算任务（Unique TTL 10min）。
func (q *Queue) EnqueueJobRescore(ctx context.Context, jobID string) error {
	payload, err := json.Marshal(PayloadJobRescore{JobID: jobID})
	if err != nil {
		return fmt.Errorf("worker: marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskJobRescore, payload, asynq.Unique(UniqueRescoreTTL), asynq.MaxRetry(3))
	_, err = q.client.EnqueueContext(ctx, task)
	if err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			slog.Warn("worker: job:rescore duplicate task skipped", "job_id", jobID)
			return nil
		}
		return fmt.Errorf("worker: enqueue job:rescore: %w", err)
	}
	slog.Info("worker: job:rescore enqueued", "job_id", jobID)
	return nil
}
