package database

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"
)

var pgxPlaceholder = regexp.MustCompile(`\$(\d+)`)

// SQLitePool wraps *sql.DB to implement the Pool interface.
// It automatically converts PostgreSQL-style $1,$2 placeholders to SQLite ? placeholders.
type SQLitePool struct {
	db *sql.DB
}

// NewSQLitePool creates a new SQLite adapter.
func NewSQLitePool(db *sql.DB) *SQLitePool {
	return &SQLitePool{db: db}
}

// convertQuery converts PostgreSQL syntax to SQLite-compatible syntax and
// returns the placeholder permutation for bindArgs.
//
// SQLite binds ? positionally, while callers supply arguments the pgx way:
// args[i] supplies $<i+1> (duplicates of a label share one argument). When
// the SQL text lists labels out of textual order, or repeats one, the
// argument slice has to be reordered so each ? receives the value of its own
// $N label. Queries that already use native ? placeholders (or mix both
// styles) keep plain positional binding.
func convertQuery(query string) (string, []int) {
	perm := placeholderPerm(query)
	// PostgreSQL NOW() → SQLite CURRENT_TIMESTAMP
	q := strings.ReplaceAll(query, "NOW()", "CURRENT_TIMESTAMP")
	q = strings.ReplaceAll(q, "now()", "CURRENT_TIMESTAMP")
	// PostgreSQL ILIKE → SQLite LIKE (SQLite LIKE is case-insensitive by default for ASCII)
	q = strings.ReplaceAll(q, " ILIKE ", " LIKE ")
	// PostgreSQL type casts ::text, ::integer, ::uuid → remove them (SQLite is dynamically typed)
	q = regexp.MustCompile(`::\w+`).ReplaceAllString(q, "")
	// PostgreSQL json_build_object → json_object (SQLite compat)
	q = strings.ReplaceAll(q, "json_build_object", "json_object")
	// PostgreSQL json_agg → group_concat
	q = strings.ReplaceAll(q, "json_agg", "json_group_array")
	// PostgreSQL array_agg → group_concat
	q = strings.ReplaceAll(q, "array_agg", "group_concat")
	return pgxPlaceholder.ReplaceAllString(q, "?"), perm
}

// placeholderPerm returns perm[i] = N-1 for the $N label at textual
// position i, or nil when positional binding already matches (no $N labels,
// native ? present, or malformed labels).
func placeholderPerm(query string) []int {
	if !strings.Contains(query, "$") {
		return nil
	}
	// Mixed ?/$N styles cannot be reordered safely — keep positional.
	if strings.Contains(query, "?") {
		return nil
	}
	matches := pgxPlaceholder.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		return nil
	}
	perm := make([]int, len(matches))
	identity := true
	for i, m := range matches {
		n, err := strconv.Atoi(query[m[2]:m[3]])
		if err != nil || n < 1 {
			return nil // malformed label → legacy positional binding
		}
		perm[i] = n - 1
		if perm[i] != i {
			identity = false
		}
	}
	if identity {
		return nil // labels already ascend 1..n positionally
	}
	return perm
}

// bindArgs reorders pgx-ordered arguments to match SQLite's textual ? order.
// Falls back to the caller's slice whenever the labels and arguments
// disagree in count, so malformed calls error exactly as before.
func bindArgs(perm []int, args []any) []any {
	if len(perm) == 0 {
		return args
	}
	out := make([]any, len(perm))
	for i, idx := range perm {
		if idx >= len(args) {
			return args
		}
		out[i] = args[idx]
	}
	return out
}

func (s *SQLitePool) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	query, perm := convertQuery(sql)
	result, err := s.db.ExecContext(ctx, query, bindArgs(perm, args)...)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	return &sqlCommandTag{affected: affected}, nil
}

func (s *SQLitePool) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	query, perm := convertQuery(sql)
	rows, err := s.db.QueryContext(ctx, query, bindArgs(perm, args)...)
	if err != nil {
		return nil, err
	}
	return &sqlRows{rows: rows}, nil
}

func (s *SQLitePool) QueryRow(ctx context.Context, sql string, args ...any) Row {
	query, perm := convertQuery(sql)
	return &sqlRow{row: s.db.QueryRowContext(ctx, query, bindArgs(perm, args)...)}
}

func (s *SQLitePool) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLitePool) Begin(ctx context.Context) (Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &sqlTx{tx: tx}, nil
}

// RawDB returns the underlying *sql.DB for subsystems that need direct access.
func (s *SQLitePool) RawDB() *sql.DB {
	return s.db
}

// sqlCommandTag wraps sql.Result.
type sqlCommandTag struct {
	affected int64
}

func (t *sqlCommandTag) RowsAffected() int64 {
	return t.affected
}

// sqlRows wraps sql.Rows.
type sqlRows struct {
	rows *sql.Rows
}

func (r *sqlRows) Next() bool             { return r.rows.Next() }
func (r *sqlRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *sqlRows) Close()                 { _ = r.rows.Close() }
func (r *sqlRows) Err() error             { return r.rows.Err() }

// sqlRow wraps sql.Row.
type sqlRow struct {
	row *sql.Row
}

func (r *sqlRow) Scan(dest ...any) error { return r.row.Scan(dest...) }

// sqlTx wraps sql.Tx.
type sqlTx struct {
	tx *sql.Tx
}

func (t *sqlTx) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	query, perm := convertQuery(sql)
	result, err := t.tx.ExecContext(ctx, query, bindArgs(perm, args)...)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	return &sqlCommandTag{affected: affected}, nil
}

func (t *sqlTx) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	query, perm := convertQuery(sql)
	rows, err := t.tx.QueryContext(ctx, query, bindArgs(perm, args)...)
	if err != nil {
		return nil, err
	}
	return &sqlRows{rows: rows}, nil
}

func (t *sqlTx) QueryRow(ctx context.Context, sql string, args ...any) Row {
	query, perm := convertQuery(sql)
	return &sqlRow{row: t.tx.QueryRowContext(ctx, query, bindArgs(perm, args)...)}
}

func (t *sqlTx) Commit(ctx context.Context) error   { return t.tx.Commit() }
func (t *sqlTx) Rollback(ctx context.Context) error { return t.tx.Rollback() }
