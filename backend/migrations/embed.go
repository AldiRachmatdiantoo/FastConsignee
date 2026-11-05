// Package migrations menyimpan skema database di dalam binary.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
