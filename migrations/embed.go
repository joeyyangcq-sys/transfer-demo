// Package migrations embeds the SQL schema files so they ship with the binary.
// 包 migrations 通过 embed 把 SQL schema 文件打包进二进制一起发布。
package migrations

import "embed"

// FS holds the migration SQL files.
// FS 持有迁移用的 SQL 文件。
//
//go:embed *.sql
var FS embed.FS
