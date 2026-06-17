package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// dbConn wraps *sql.DB with dialect-aware SQL translation. It exposes the same
// method shapes the codebase already uses on *sql.DB (Query → *sql.Rows,
// QueryRow → *sql.Row, Exec, Begin, Close) so the ~300 call sites compile
// unchanged; every method translates the query string for the active dialect
// before delegating.
type dbConn struct {
	sdb *sql.DB
	d   dialect
}

func (c *dbConn) Exec(q string, args ...any) (sql.Result, error) {
	return c.sdb.Exec(translate(c.d, q), args...)
}

func (c *dbConn) Query(q string, args ...any) (*sql.Rows, error) {
	return c.sdb.Query(translate(c.d, q), args...)
}

func (c *dbConn) QueryRow(q string, args ...any) *sql.Row {
	return c.sdb.QueryRow(translate(c.d, q), args...)
}

func (c *dbConn) Begin() (*dbTx, error) {
	tx, err := c.sdb.Begin()
	if err != nil {
		return nil, err
	}
	return &dbTx{stx: tx, d: c.d}, nil
}

func (c *dbConn) Close() error { return c.sdb.Close() }

// ExecInsert runs a single-row INSERT and returns the new row id, abstracting
// SQLite's LastInsertId vs Postgres' RETURNING id. Targets must have an "id" PK.
func (c *dbConn) ExecInsert(q string, args ...any) (int64, error) {
	return execInsert(c.d, c, q, args...)
}

// dbTx mirrors dbConn for transactions, wrapping *sql.Tx.
type dbTx struct {
	stx *sql.Tx
	d   dialect
}

func (t *dbTx) Exec(q string, args ...any) (sql.Result, error) {
	return t.stx.Exec(translate(t.d, q), args...)
}

func (t *dbTx) Query(q string, args ...any) (*sql.Rows, error) {
	return t.stx.Query(translate(t.d, q), args...)
}

func (t *dbTx) QueryRow(q string, args ...any) *sql.Row {
	return t.stx.QueryRow(translate(t.d, q), args...)
}

func (t *dbTx) Commit() error   { return t.stx.Commit() }
func (t *dbTx) Rollback() error { return t.stx.Rollback() }

func (t *dbTx) ExecInsert(q string, args ...any) (int64, error) {
	return execInsert(t.d, t, q, args...)
}

// execQueryer is satisfied by both *dbConn and *dbTx.
type execQueryer interface {
	Exec(string, ...any) (sql.Result, error)
	QueryRow(string, ...any) *sql.Row
}

func execInsert(d dialect, eq execQueryer, q string, args ...any) (int64, error) {
	if d == dialectPostgres {
		var id int64
		// QueryRow translates placeholders; RETURNING id has none.
		if err := eq.QueryRow(q+" RETURNING id", args...).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}
	res, err := eq.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// isUniqueViolation reports whether err is a unique-constraint violation. It
// matches both SQLite ("UNIQUE constraint failed: …") and Postgres ("duplicate
// key value violates unique constraint …").
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}

// isPostgresURL reports whether the -db value selects the Postgres backend.
func isPostgresURL(s string) bool {
	return strings.HasPrefix(s, "postgres://") || strings.HasPrefix(s, "postgresql://")
}

// IsPostgresURL is the exported form, for callers outside this package.
func IsPostgresURL(s string) bool { return isPostgresURL(s) }

// IsPostgres reports whether this store is backed by PostgreSQL.
func (s *Store) IsPostgres() bool { return s.db.d == dialectPostgres }

// PingDSN opens a short-lived connection to a Postgres DSN and verifies it is
// reachable. Used by the first-run setup to validate user-entered credentials
// before committing to them.
func PingDSN(dsn string) error {
	if !isPostgresURL(dsn) {
		return fmt.Errorf("not a postgres URL")
	}
	d, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer d.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return d.PingContext(ctx)
}
