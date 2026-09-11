// Package migrations carries the SQL schema files. They are embedded so that the
// deployable artefact stays the binary plus its assets.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
