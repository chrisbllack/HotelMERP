package database

import (
	"reflect"
	"strings"
	"testing"
)

// The adapter's contract: pgx-style arguments (args[i] supplies $<i+1>,
// duplicates share one argument) must reach SQLite's positional ? binding
// with the value of their own label — or the query silently writes the
// wrong column / matches zero rows.

func TestConvertQueryAscendingIsPositional(t *testing.T) {
	q, perm := convertQuery("SELECT id FROM t WHERE a = $1 AND b = $2")
	if perm != nil {
		t.Fatalf("ascending labels must keep positional binding, perm=%v", perm)
	}
	if strings.Contains(q, "$") {
		t.Errorf("placeholders not converted: %s", q)
	}
	if q != "SELECT id FROM t WHERE a = ? AND b = ?" {
		t.Errorf("unexpected SQL: %s", q)
	}
}

func TestConvertQueryReordersDescendingLabels(t *testing.T) {
	q, perm := convertQuery("UPDATE reservations SET status = $3 WHERE id = $1 AND property_id = $2")
	if !reflect.DeepEqual(perm, []int{2, 0, 1}) {
		t.Fatalf("perm = %v, want [2 0 1]", perm)
	}
	// Caller passes (id, propertyID, status) pgx-style.
	args := bindArgs(perm, []any{"id-1", "prop-1", "checked_in"})
	if !reflect.DeepEqual(args, []any{"checked_in", "id-1", "prop-1"}) {
		t.Errorf("bindArgs = %v", args)
	}
	if strings.Contains(q, "$") {
		t.Errorf("placeholders not converted: %s", q)
	}
}

func TestConvertQueryRepeatedLabelSharesOneArgument(t *testing.T) {
	// The loyalty UPDATE pattern: repeated $1 references, one $2 WHERE —
	// two arguments total.
	perm := placeholderPerm(`
		UPDATE guests SET points = points + $1,
			tier = CASE WHEN points + $1 >= 1000 THEN 'vip' ELSE 'member' END
		WHERE id = $2`)
	if !reflect.DeepEqual(perm, []int{0, 0, 1}) {
		t.Fatalf("perm = %v, want [0 0 1]", perm)
	}
	args := bindArgs(perm, []any{250, "guest-1"})
	want := []any{250, 250, "guest-1"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("bindArgs = %v, want %v", args, want)
	}
}

func TestNativePlaceholdersStayPositional(t *testing.T) {
	q, perm := convertQuery("INSERT INTO t VALUES (?, ?, ?)")
	if perm != nil {
		t.Fatalf("native ? query must stay positional, perm=%v", perm)
	}
	if q != "INSERT INTO t VALUES (?, ?, ?)" {
		t.Errorf("unexpected SQL: %s", q)
	}
}

func TestMixedStylesFallBackPositionally(t *testing.T) {
	_, perm := convertQuery("SELECT $1, ?")
	if perm != nil {
		t.Fatalf("mixed styles must fall back, perm=%v", perm)
	}
}

func TestMissingArgumentFallsBack(t *testing.T) {
	perm := placeholderPerm("UPDATE t SET a = $2")
	args := bindArgs(perm, []any{"only-one"})
	if !reflect.DeepEqual(args, []any{"only-one"}) {
		t.Errorf("out-of-range label must fall back to caller slice, got %v", args)
	}
}

func TestConvertQueryRewritesPgSyntax(t *testing.T) {
	q, _ := convertQuery("UPDATE t SET at = NOW() WHERE x::uuid = $1 AND n ILIKE $2")
	if strings.Contains(q, "NOW()") || strings.Contains(q, "::uuid") || strings.Contains(q, "ILIKE") {
		t.Errorf("PostgreSQL syntax not rewritten: %s", q)
	}
	if !strings.Contains(q, "CURRENT_TIMESTAMP") {
		t.Errorf("NOW() not converted: %s", q)
	}
}
