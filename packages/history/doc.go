// Package history provides persistent storage for hitspec run results.
//
// It uses SQLite (via modernc.org/sqlite) to store runs, individual request
// results, and assertion outcomes. The schema is automatically applied on
// first use via [NewStore].
//
// The package is built on sqlc-generated code for type-safe database access,
// with a high-level [Store] wrapper for common operations.
package history
