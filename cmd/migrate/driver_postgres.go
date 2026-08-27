//go:build postgres

// Build with `-tags postgres` in an environment with package-registry
// network access (the SOMA Genesis Sandbox) after running
// `go mod tidy -tags postgres` to add the driver to go.mod/go.sum.
package main

import _ "github.com/jackc/pgx/v5/stdlib"
