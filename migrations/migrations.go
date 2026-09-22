// Package migrations embeds the goose SQL migrations so every binary carries
// the schema it expects. Files are applied by cmd/migrate and by the test
// infrastructure; the API never applies them implicitly.
package migrations

import "embed"

// FS contains every *.sql migration in this directory.
//
//go:embed *.sql
var FS embed.FS
