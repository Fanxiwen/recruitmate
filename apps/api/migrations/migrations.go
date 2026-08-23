// Package migrations 嵌入 goose 迁移 SQL 文件，供服务启动时自动执行。
package migrations

import "embed"

// FS 内嵌全部 *.sql 迁移文件。
//
//go:embed *.sql
var FS embed.FS
