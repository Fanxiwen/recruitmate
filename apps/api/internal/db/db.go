// Package db 提供数据库连接与 goose 迁移执行。
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Fanxiwen/recruitmate/apps/api/migrations"
	"github.com/pressly/goose/v3"
	// pgx stdlib 驱动（goose 需要 *sql.DB）
	_ "github.com/jackc/pgx/v5/stdlib"
)

// RunMigrations 执行 goose 迁移（Up，幂等）。
func RunMigrations(ctx context.Context, dsn string) error {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("db: open sql: %w", err)
	}
	defer sqlDB.Close()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("db: ping: %w", err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("db: set dialect: %w", err)
	}
	if err := goose.UpContext(ctx, sqlDB, "."); err != nil {
		return fmt.Errorf("db: migrate up: %w", err)
	}
	return nil
}
