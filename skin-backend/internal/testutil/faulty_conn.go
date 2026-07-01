package testutil

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type faultyRows struct {
	pgx.Rows
	scanErr error
	errErr  error
	closed  bool
}

func (r *faultyRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	return r.Rows.Scan(dest...)
}

func (r *faultyRows) Err() error {
	if r.errErr != nil {
		return r.errErr
	}
	return r.Rows.Err()
}

func (r *faultyRows) Close() {
	if !r.closed {
		r.closed = true
		r.Rows.Close()
	}
}

type FaultyConn struct {
	pool         *pgxpool.Pool
	scanErr      error
	scanErrQuery int
	errErr       error
	errErrQuery  int
	queryCount   int
}

func NewFaultyConn(pool *pgxpool.Pool) *FaultyConn {
	return &FaultyConn{pool: pool}
}

func (fc *FaultyConn) WithScanError(err error) *FaultyConn {
	fc.scanErr = err
	return fc
}

func (fc *FaultyConn) WithScanErrorOnQuery(queryNumber int, err error) *FaultyConn {
	fc.scanErrQuery = queryNumber
	fc.scanErr = err
	return fc
}

func (fc *FaultyConn) WithRowsErr(err error) *FaultyConn {
	fc.errErr = err
	return fc
}

func (fc *FaultyConn) WithRowsErrOnQuery(queryNumber int, err error) *FaultyConn {
	fc.errErrQuery = queryNumber
	fc.errErr = err
	return fc
}

func (fc *FaultyConn) Begin(ctx context.Context) (pgx.Tx, error) {
	return fc.pool.Begin(ctx)
}

func (fc *FaultyConn) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return fc.pool.Exec(ctx, sql, args...)
}

func (fc *FaultyConn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := fc.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	fc.queryCount++
	var scanErr, rowsErr error
	if fc.scanErrQuery == 0 || fc.scanErrQuery == fc.queryCount {
		scanErr = fc.scanErr
	}
	if fc.errErrQuery == 0 || fc.errErrQuery == fc.queryCount {
		rowsErr = fc.errErr
	}
	return &faultyRows{Rows: rows, scanErr: scanErr, errErr: rowsErr}, nil
}

func (fc *FaultyConn) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return fc.pool.QueryRow(ctx, sql, args...)
}

var ErrFaultInjected = errors.New("fault injected")
