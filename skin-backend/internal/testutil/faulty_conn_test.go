package testutil

import (
	"context"
	"errors"
	"testing"
)

func TestFaultyConnForwardsDatabaseOperationsAndInjectsExactRowsErrors(t *testing.T) {
	db, _ := NewTestApp(t)
	ctx := context.Background()
	conn := NewFaultyConn(db.Pool)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin returned unexpected error: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback returned unexpected error: %v", err)
	}

	tag, err := conn.Exec(ctx, `CREATE TEMP TABLE faulty_conn_values (id INT PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("Exec create temp table returned unexpected error: %v", err)
	}
	if got := tag.String(); got != "CREATE TABLE" {
		t.Fatalf("Create tag mismatch: got=%q want=%q", got, "CREATE TABLE")
	}
	tag, err = conn.Exec(ctx, `INSERT INTO faulty_conn_values (id, name) VALUES (1, 'alpha')`)
	if err != nil {
		t.Fatalf("Exec insert returned unexpected error: %v", err)
	}
	if got := tag.RowsAffected(); got != 1 {
		t.Fatalf("Insert rows affected mismatch: got=%d want=1", got)
	}

	var rowName string
	if err := conn.QueryRow(ctx, `SELECT name FROM faulty_conn_values WHERE id=$1`, 1).Scan(&rowName); err != nil {
		t.Fatalf("QueryRow scan returned unexpected error: %v", err)
	}
	if rowName != "alpha" {
		t.Fatalf("QueryRow value mismatch: got=%q want=%q", rowName, "alpha")
	}

	scanErr := errors.New("scan injection")
	rows, err := NewFaultyConn(db.Pool).WithScanError(scanErr).Query(ctx, `SELECT name FROM faulty_conn_values WHERE id=$1`, 1)
	if err != nil {
		t.Fatalf("Query with scan injection returned unexpected error: %v", err)
	}
	if !rows.Next() {
		t.Fatal("Query with scan injection should return one row")
	}
	if err := rows.Scan(&rowName); !errors.Is(err, scanErr) {
		t.Fatalf("Scan error mismatch: got=%v want=%v", err, scanErr)
	}
	rows.Close()
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("Rows Err after scan injection mismatch: got=%v want=<nil>", err)
	}

	rowsErr := errors.New("rows err injection")
	rows, err = NewFaultyConn(db.Pool).WithRowsErr(rowsErr).Query(ctx, `SELECT name FROM faulty_conn_values WHERE id=$1`, 1)
	if err != nil {
		t.Fatalf("Query with rows error injection returned unexpected error: %v", err)
	}
	rows.Close()
	if err := rows.Err(); !errors.Is(err, rowsErr) {
		t.Fatalf("Rows Err mismatch: got=%v want=%v", err, rowsErr)
	}

	targetedScanErr := errors.New("targeted scan injection")
	conn = NewFaultyConn(db.Pool).WithScanErrorOnQuery(2, targetedScanErr)
	rows, err = conn.Query(ctx, `SELECT name FROM faulty_conn_values WHERE id=$1`, 1)
	if err != nil {
		t.Fatalf("first targeted query returned unexpected error: %v", err)
	}
	if !rows.Next() {
		t.Fatal("first targeted query should return one row")
	}
	if err := rows.Scan(&rowName); err != nil {
		t.Fatalf("first targeted query should not inject scan error, got %v", err)
	}
	rows.Close()
	rows, err = conn.Query(ctx, `SELECT name FROM faulty_conn_values WHERE id=$1`, 1)
	if err != nil {
		t.Fatalf("second targeted query returned unexpected error: %v", err)
	}
	if !rows.Next() {
		t.Fatal("second targeted query should return one row")
	}
	if err := rows.Scan(&rowName); !errors.Is(err, targetedScanErr) {
		t.Fatalf("targeted scan error mismatch: got=%v want=%v", err, targetedScanErr)
	}
	rows.Close()

	targetedRowsErr := errors.New("targeted rows injection")
	conn = NewFaultyConn(db.Pool).WithRowsErrOnQuery(2, targetedRowsErr)
	rows, err = conn.Query(ctx, `SELECT name FROM faulty_conn_values WHERE id=$1`, 1)
	if err != nil {
		t.Fatalf("first targeted rows query returned unexpected error: %v", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("first targeted rows query should not inject rows error: %v", err)
	}
	rows, err = conn.Query(ctx, `SELECT name FROM faulty_conn_values WHERE id=$1`, 1)
	if err != nil {
		t.Fatalf("second targeted rows query returned unexpected error: %v", err)
	}
	rows.Close()
	if err := rows.Err(); !errors.Is(err, targetedRowsErr) {
		t.Fatalf("targeted rows error mismatch: got=%v want=%v", err, targetedRowsErr)
	}
}
