// Package migrations embeds the SQL migrations so the API binary and the
// migrate command can apply them without needing the files on disk.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
