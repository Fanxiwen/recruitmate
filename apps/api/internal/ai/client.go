// Package ai 封装 AI 能力：LLM 结构化输出（DeepSeek，Eino OpenAI 兼容组件）
// 与文本向量化（SiliconFlow，OpenAI 兼容 /embeddings，标准库 net/http 直连）。
//
// 所有 AI 调用记录耗时与 token 用量到 slog，便于成本与质量观测。
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

// ErrDisabled 表示对应 AI 能力未配置（缺 API Key）而被禁用。
var ErrDisabled = errors.New("ai client disabled: missing api key")

// Client AI 客户端接口：ChatJSON 用于 LLM 结构化输出，Embed 用于文本向量化。
type Client interface {
	// ChatJSON 将 system/user 提示词发给 LLM，并把返回的 JSON 解析进 out。
	ChatJSON(ctx context.Context, system, user string, out any) error
	// Embed 将文本转为 embedding 向量。
	Embed(ctx context.Context, text string) ([]float32, error)
}

// StatusProvider 能力状态查询（供匹配层判断降级策略）。
type StatusProvider interface {
	ChatEnabled() bool
	EmbedEnabled() bool
}

// Capabilities 查询客户端各项能力是否启用。
func Capabilities(c Client) (chat, embed bool) {
	if sp, ok := c.(StatusProvider); ok {
		return sp.ChatEnabled(), sp.EmbedEnabled()
	}
	return true, true
}

// Config AI 客户端配置。
type Config struct {
	DeepSeekAPIKey     string
	DeepSeekBaseURL    string
	DeepSeekModel      string
	SiliconFlowAPIKey  string
	SiliconFlowBaseURL string
	EmbeddingModel     string
	EmbeddingDim       int
	HTTPClient         *http.Client
}

// New 构造 AI 客户端。各能力独立降级：
//   - 未配置 DeepSeek Key → ChatJSON 返回 ErrDisabled；
//   - 未配置 SiliconFlow Key → Embed 返回 ErrDisabled。
//
// matching 层依据 ChatJSON 是否可用决定 engine='ai' 或 'rule'。
func New(ctx context.Context, cfg Config) (Client, error) {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	if cfg.DeepSeekBaseURL == "" {
		cfg.DeepSeekBaseURL = "https://api.deepseek.com/v1"
	}
	if cfg.DeepSeekModel == "" {
		cfg.DeepSeekModel = "deepseek-chat"
	}
	if cfg.SiliconFlowBaseURL == "" {
		cfg.SiliconFlowBaseURL = "https://api.siliconflow.cn/v1"
	}
	if cfg.EmbeddingModel == "" {
		cfg.EmbeddingModel = "BAAI/bge-m3"
	}
	if cfg.EmbeddingDim == 0 {
		cfg.EmbeddingDim = 1024
	}
	c := &client{cfg: cfg}
	if cfg.DeepSeekAPIKey != "" {
		chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  cfg.DeepSeekAPIKey,
			BaseURL: cfg.DeepSeekBaseURL,
			Model:   cfg.DeepSeekModel,
			Timeout: 60 * time.Second,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("ai: create chat model: %w", err)
		}
		c.chatModel = chatModel
	}
	return c, nil
}

// client 具体实现：内部持有 chat 与 embed 两个子组件。
type client struct {
	cfg       Config
	chatModel *openai.ChatModel
}

// ChatEnabled 是否启用 LLM 结构化输出。
func (c *client) ChatEnabled() bool { return c.cfg.DeepSeekAPIKey != "" }

// EmbedEnabled 是否启用向量化。
func (c *client) EmbedEnabled() bool { return c.cfg.SiliconFlowAPIKey != "" }

// ChatJSON 调用 DeepSeek 结构化输出。
func (c *client) ChatJSON(ctx context.Context, system, user string, out any) error {
	if c.chatModel == nil {
		return ErrDisabled
	}
	start := time.Now()

	msgs := []*schema.Message{
		schema.SystemMessage(system),
		schema.UserMessage(user),
	}
	resp, err := c.chatModel.Generate(ctx, msgs)
	elapsed := time.Since(start)
	if err != nil {
		slog.Error("ai: chat generate failed",
			"model", c.cfg.DeepSeekModel, "elapsed_ms", elapsed.Milliseconds(), "error", err)
		return fmt.Errorf("ai chat: generate: %w", err)
	}
	// 注：Eino openai 组件的 schema.Message 不暴露 token 用量，仅记录耗时；
	// 若需用量统计，可在生产环境通过 LLM 网关/服务商后台获取。
	slog.Info("ai: chat complete", "model", c.cfg.DeepSeekModel, "elapsed_ms", elapsed.Milliseconds())

	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		return errors.New("ai chat: empty response")
	}
	cleaned := RepairJSON(resp.Content)
	if err := json.Unmarshal([]byte(cleaned), out); err != nil {
		slog.Error("ai: chat json unmarshal failed", "error", err, "raw", truncate(resp.Content, 500))
		return fmt.Errorf("ai chat: unmarshal: %w", err)
	}
	return nil
}

// Embed 调用 SiliconFlow /embeddings 文本向量化。
func (c *client) Embed(ctx context.Context, text string) ([]float32, error) {
	if c.cfg.SiliconFlowAPIKey == "" {
		return nil, ErrDisabled
	}
	start := time.Now()
	body, err := json.Marshal(map[string]any{
		"model": c.cfg.EmbeddingModel,
		"input": text,
	})
	if err != nil {
		return nil, fmt.Errorf("ai embed: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.SiliconFlowBaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai embed: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.SiliconFlowAPIKey)

	resp, err := c.cfg.HTTPClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		slog.Error("ai: embed request failed", "model", c.cfg.EmbeddingModel, "elapsed_ms", elapsed.Milliseconds(), "error", err)
		return nil, fmt.Errorf("ai embed: request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("ai embed: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		slog.Error("ai: embed non-200", "status", resp.StatusCode, "body", truncate(string(raw), 300))
		return nil, fmt.Errorf("ai embed: status %d", resp.StatusCode)
	}

	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("ai embed: unmarshal: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, errors.New("ai embed: empty data")
	}
	vec := parsed.Data[0].Embedding
	if len(vec) != c.cfg.EmbeddingDim {
		slog.Warn("ai: embedding dim mismatch",
			"expected", c.cfg.EmbeddingDim, "actual", len(vec), "model", c.cfg.EmbeddingModel)
	}
	slog.Info("ai: embed complete",
		"model", c.cfg.EmbeddingModel, "dim", len(vec),
		"total_tokens", parsed.Usage.TotalTokens, "elapsed_ms", elapsed.Milliseconds())
	return vec, nil
}

// RepairJSON 修复 LLM 输出的 JSON：去除 markdown code fence、截取首尾花括号。
func RepairJSON(s string) string {
	s = strings.TrimSpace(s)
	// 去除 ```json ... ``` 围栏
	if i := strings.Index(s, "```"); i >= 0 {
		rest := s[i+3:]
		rest = strings.TrimPrefix(rest, "json")
		if j := strings.LastIndex(rest, "```"); j >= 0 {
			rest = rest[:j]
		}
		s = rest
	}
	// 截取首个 { 与末尾 }
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		s = s[start : end+1]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
