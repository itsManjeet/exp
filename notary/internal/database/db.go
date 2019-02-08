// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package database defines the database interface used by the notary.
// It operates at a lower level of abstraction than database/sql,
// modeled on Google Cloud Spanner.
package database

import (
	"bytes"
	"context"
	"fmt"
	"time"
	"unicode/utf8"
)

// A DB is a connection to a database.
type DB interface {
	// CreateTables creates tables with the given configurations.
	// If any named tables already exist, the call fails.
	// The creation of the tables is a single transaction: either it succeeds in full or it does nothing.
	CreateTables(ctx context.Context, tables []*Table) error

	// ReadOnly runs f in a read-only transaction.
	// It is equivalent to ReadWrite except that the
	// transaction's BufferWrite method will fail unconditionally.
	// (The implementation may be able to optimize the
	// transaction if it knows at the start that no writes will happen.)
	ReadOnly(ctx context.Context, f func(context.Context, Transaction) error) error

	// ReadWrite runs f in a read-write transaction.
	// If f returns an error, the transaction aborts and returns that error.
	// If f returns nil, the transaction attempts to commit and then then return nil.
	// Otherwise it tries again. Note that f may be called multiple times and that
	// the result applies only describes the effect of the final call to f.
	// The caller must take care not to use any state computed during
	// earlier calls to f, or even the last call to f when an error is returned.
	ReadWrite(ctx context.Context, f func(context.Context, Transaction) error) error
}

// A Table describes a table to be created.
type Table struct {
	Name       string   // name of the table
	Columns    []Column // columns for table
	PrimaryKey []string // names of primary key columns
}

// A Column describes a column in a table.
type Column struct {
	Name    string // name of column
	Type    string // data type: "int64", "string", "bytes", "timestamp"
	Size    int    // max size for Type "string", "bytes"; unlimited for Size <= 0
	NotNull bool   // if true, column values must not be null
}

// A Transaction provides read and write operations within a database transaction,
// as executed by DB's ReadOnly or ReadWrite methods.
type Transaction interface {
	// Read reads from table the rows with the given keys
	// and returns the specified columns of those rows.
	// If keys.List lists keys not present in the table, no corresponding
	// rows are returned, but the overall Read still succeeds: it does not report an error.
	// If keys.All is true, the returned rows are in primary key order.
	// Otherwise the order is undefined.
	Read(ctx context.Context, table string, keys Keys, columns []string) Rows

	// ReadRow reads from table the single row with the given key
	// and returns the specified columns of that row.
	// If there is no row with that key, ReadRow returns an error of type *RowNotExistError.
	ReadRow(ctx context.Context, table string, key Key, columns []string) (Row, error)

	// BufferWrite buffers the given writes to be applied at the end of the transaction.
	// It returns an error if this is a ReadOnly transaction
	// or if the writes are detected to be inconsistent with the current database state.
	BufferWrite(writes []Mutation) error
}

// A Key is a single database key, giving the values of each of the primary key columns,
// listed in the table's primary key order (as defined by Table's PrimaryKey field).
type Key []interface{}

// A Keys describes a set of row keys to return from Read.
type Keys struct {
	All  bool  // if true, return all rows
	List []Key // otherwise, return any rows from keys on this list
}

// A Rows is a set of rows returned by a Transaction's Read method.
type Rows interface {
	// Do invokes f for each Row in the set of rows.
	// Do can only be called once for a given Rows.
	// If f returns an error, Do stops iterating over the rows and returns that error.
	// Do may also return an error if there was a problem with the Read operation.
	Do(f func(Row) error) error
}

// A Row is a single database row returned by a Transaction's Read or ReadRow method.
type Row interface {
	// Column reads the value of the column with the given index
	// (in the list of column names passed to Read or ReadRow)
	// and stores it into *dst, which must be a value of type appropriate
	// to the column type (int64, string, or []byte for bytes).
	Column(index int, dst interface{}) error
}

// ReadRow returns a *RowNotExistError when the specific requested row is not in the table.
type RowNotExistError struct {
	Err error
}

func (e *RowNotExistError) Error() string {
	return e.Err.Error()
}

// IsRowNotExist reports whether err is a *RowNotExistError.
func IsRowNotExist(err error) bool {
	_, ok := err.(*RowNotExistError)
	return ok
}

// A MutationOp specifies the specific kind of database mutation.
type MutationOp string

const (
	// Delete deletes any rows matching Keys.
	// It is not an error to list Keys for which there are no existing rows.
	Delete MutationOp = "delete"

	// Insert inserts a row with the named columns Cols set to the values Vals.
	// The row key is derived from the corresponding column values.
	// It is an error if a row with that key already exists.
	Insert MutationOp = "insert"

	// InsertOrUpdate either inserts a new row or updates an existing one.
	InsertOrUpdate MutationOp = "insert-or-update"

	// Replace is like Insert but if a row with the key exists already,
	// it overwrites that existing row, ignoring any previous column values.
	Replace MutationOp = "replace"

	// Update updates an existing row to set the named columns Cols to the values Vals.
	// The row key is derived from the corresponding column values.
	// It is an error if a row with that key does not already exist.
	// The values in the existing row for columns not listed in Cols are preserved.
	Update MutationOp = "update"
)

// A Mutation represents a single mutation to apply at the end of a transaction.
type Mutation struct {
	Op    MutationOp    // operation
	Table string        // table name
	Cols  []string      // for Op != Delete
	Vals  []interface{} // for Op != Delete
	Keys  Keys          // for Op == Delete
}

// String returns a printable string form of a key.
// It is a bracketed comma-separated list.
// A value from a column of type bytes (Go []byte)
// is printed as a quoted string if the value is valid UTF-8,
// and as a hexadecimal byte sequence otherwise.
// A value of column type timestamp (Go time.Time)
// is printed in RFC3339 format.
func (k Key) String() string {
	var buf bytes.Buffer
	buf.WriteString("[")
	for i, v := range k {
		if i > 0 {
			buf.WriteString(", ")
		}
		switch v := v.(type) {
		case time.Time:
			buf.WriteString(v.Format(time.RFC3339Nano))
		case []byte:
			if utf8.Valid(v) {
				fmt.Fprintf(&buf, "%q", v)
			} else {
				fmt.Fprintf(&buf, "%x", v)
			}
		default:
			fmt.Fprint(&buf, v)
		}
	}
	buf.WriteString("]")
	return buf.String()
}
