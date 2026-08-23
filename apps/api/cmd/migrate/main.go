// Package main 提供 goose 迁移命令行工具（up/down）。
//
// 用法：
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/config"
	"github.com/Fanxiwen/recruitmate/apps/api/migrations"
	"github.com/pressly/goose/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: migrate <up|down>")
		os.Exit(2)
	}
	action := os.Args[1]
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	sqlDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		slog.Error("open db", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()
	ctx := context.Background()
	if err := sqlDB.PingContext(ctx); err != nil {
		slog.Error("ping db", "error", err)
		os.Exit(1)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("set dialect", "error", err)
		os.Exit(1)
	}

	switch action {
	case "up":
		if err := goose.UpContext(ctx, sqlDB, "."); err != nil {
			slog.Error("migrate up", "error", err)
			os.Exit(1)
		}
	case "down":
		if err := goose.DownContext(ctx, sqlDB, "."); err != nil {
			slog.Error("migrate down", "error", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "未知动作: %s（仅支持 up/down）\n", action)
		os.Exit(2)
	}
	slog.Info("migrate " + action + " done")
}
