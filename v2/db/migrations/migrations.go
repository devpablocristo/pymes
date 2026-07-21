package migrations

import "embed"

// Files contains all Pymes v2 migrations. Files are applied lexicographically.
//
//go:embed *.sql
var Files embed.FS
