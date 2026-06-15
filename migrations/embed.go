// Package migrations embeds the SQL schema files so they ship with the binary.
package migrations

import "embed"

// FS holds the migration SQL files.
//
//go:embed *.sql
var FS embed.FS
