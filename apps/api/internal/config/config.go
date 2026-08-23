// Package config 负责加载服务配置。
//
// 配置以环境变量为主，支持从工作目录的 .env 文件加载（不覆盖已存在的环境变量）。
// 所有配置项在 .env.example 中列出。
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 服务全部配置。
type Config struct {
	AppEnv  string
	HTTPAddr string

	DatabaseURL string
	RedisAddr   string

	JWTSecret string
	JWTTTL    time.Duration

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3UseSSL    bool

	DeepSeekAPIKey    string
	DeepSeekBaseURL   string
	DeepSeekModel     string
	SiliconFlowAPIKey string
	SiliconFlowBaseURL string
	EmbeddingModel    string
	EmbeddingDim      int

	AITopN    int
	AILLMW    float64
	AISemanticW float64
	AIRuleW   float64

	AsyncWorkers int
	CORSOrigins  []string
}

// Load 读取 .env（若存在）与环境变量，填充默认值后返回配置。
func Load() (*Config, error) {
	loadDotEnv(".env")

	cfg := &Config{
		AppEnv:   getenv("APP_ENV", "dev"),
		HTTPAddr: getenv("HTTP_ADDR", ":8080"),

		DatabaseURL: getenv("DATABASE_URL", "postgres://recruitmate:recruitmate@localhost:5432/recruitmate?sslmode=disable"),
		RedisAddr:   getenv("REDIS_ADDR", "localhost:6379"),

		JWTSecret: getenv("JWT_SECRET", "dev-only-secret-change-me"),
		JWTTTL:    getenvDuration("JWT_TTL", 24*time.Hour),

		S3Endpoint:  getenv("S3_ENDPOINT", "localhost:9000"),
		S3AccessKey: getenv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey: getenv("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:    getenv("S3_BUCKET", "resumes"),
		S3UseSSL:    getenvBool("S3_USE_SSL", false),

		DeepSeekAPIKey:     getenv("DEEPSEEK_API_KEY", ""),
		DeepSeekBaseURL:    getenv("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1"),
		DeepSeekModel:      getenv("DEEPSEEK_MODEL", "deepseek-chat"),
		SiliconFlowAPIKey:  getenv("SILICONFLOW_API_KEY", ""),
		SiliconFlowBaseURL: getenv("SILICONFLOW_BASE_URL", "https://api.siliconflow.cn/v1"),
		EmbeddingModel:     getenv("EMBEDDING_MODEL", "BAAI/bge-m3"),
		EmbeddingDim:       getenvInt("EMBEDDING_DIM", 1024),

		AITopN:       getenvInt("AI_TOPN", 50),
		AILLMW:       getenvFloat("AI_LLM_W", 0.45),
		AISemanticW:  getenvFloat("AI_SEMANTIC_W", 0.30),
		AIRuleW:      getenvFloat("AI_RULE_W", 0.25),

		AsyncWorkers: getenvInt("ASYNC_WORKERS", 4),
		CORSOrigins:  getenvList("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174"),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("config: JWT_SECRET 不能为空")
	}
	return cfg, nil
}

// getenv 读取环境变量，为空时返回默认值。
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getenvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getenvList(key, def string) []string {
	raw := getenv(key, def)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv 极简 .env 解析器：逐行读取 KEY=VALUE，仅当环境变量未设置时写入。
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}
