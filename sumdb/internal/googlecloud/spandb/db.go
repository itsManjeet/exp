// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package spandb implements a database.DB using Google Cloud Spanner.
package spandb

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/spanner"
	admin "cloud.google.com/go/spanner/admin/database/apiv1"
	"golang.org/x/exp/sumdb/internal/database"
	adminpb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"
	"google.golang.org/grpc/codes"
)

// A DB is a connection to a Spanner database.
type DB struct {
	name   string
	client *spanner.Client
	admin  *admin.DatabaseAdminClient
}

// OpenDB opens the Spanner database with the given name
// (for example, "projects/my-project/instances/my-instance/databases/my_db").
// The database must already exist.
func OpenDB(ctx context.Context, name string) (*DB, error) {
	client, err := spanner.NewClient(ctx, name)
	if err != nil {
		return nil, err
	}
	db := &DB{name: name, client: client}
	return db, nil
}

// CreateDB creates a Spanner database with the given name
// (for example, "projects/my-project/instances/my-instance/databases/my_db").
// The database must not already exist.
func CreateDB(ctx context.Context, name string) (*DB, error) {
	f := strings.Split(name, "/")
	if len(f) != 6 || f[0] != "projects" || f[2] != "instances" || f[4] != "databases" {
		return nil, fmt.Errorf("malformed name %q", name)
	}
	adminClient, err := admin.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	req := &adminpb.CreateDatabaseRequest{
		Parent:          strings.Join(f[:4], "/"),
		CreateStatement: "CREATE DATABASE " + f[5],
	}
	op, err := adminClient.CreateDatabase(ctx, req)
	if err != nil {
		return nil, err
	}
	if _, err := op.Wait(ctx); err != nil {
		return nil, err
	}
	client, err := spanner.NewClient(ctx, name)
	if err != nil {
		return nil, err
	}
	db := &DB{name: name, client: client, admin: adminClient}
	return db, nil
}

// DeleteTestDB deletes the Spaner database with the given name.
// (for example, "projects/my-project/instances/my-instance/databases/test_my_db").
// To avoid unfortunate accidents, DeleteTestDB returns an error
// if the database name does not begin with "test_".
func DeleteTestDB(ctx context.Context, name string) error {
	f := strings.Split(name, "/")
	if len(f) != 6 || f[0] != "projects" || f[2] != "instances" || f[4] != "databases" {
		return fmt.Errorf("malformed name %q", name)
	}
	if !strings.HasPrefix(f[5], "test_") {
		return fmt.Errorf("can only delete test dbs")
	}
	adminClient, err := admin.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	req := &adminpb.DropDatabaseRequest{
		Database: name,
	}
	return adminClient.DropDatabase(ctx, req)
}

// CreateTables creates the described tables.
func (db *DB) CreateTables(ctx context.Context, tables []*database.Table) error {
	if db.admin == nil {
		adminClient, err := admin.NewDatabaseAdminClient(ctx)
		if err != nil {
			return err
		}
		db.admin = adminClient
	}

	var stmts []string
	for _, table := range tables {
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "CREATE TABLE `%s` (\n", table.Name)
		for _, col := range table.Columns {
			fmt.Fprintf(&buf, "\t`%s` %s", col.Name, strings.ToUpper(col.Type))
			switch col.Type {
			case "string", "bytes":
				if col.Size <= 0 {
					fmt.Fprintf(&buf, "(MAX)")
				} else {
					fmt.Fprintf(&buf, "(%d)", col.Size)
				}
			}
			fmt.Fprintf(&buf, ",\n")
		}
		fmt.Fprintf(&buf, ") PRIMARY KEY(")
		for i, key := range table.PrimaryKey {
			if i > 0 {
				fmt.Fprintf(&buf, ", ")
			}
			fmt.Fprintf(&buf, "`%s`", key)
		}
		fmt.Fprintf(&buf, ")\n")
		stmts = append(stmts, buf.String())
	}

	req := &adminpb.UpdateDatabaseDdlRequest{
		Database:   db.name,
		Statements: stmts,
	}
	op, err := db.admin.UpdateDatabaseDdl(ctx, req)
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

// ReadOnly executes f in a read-only transaction.
func (db *DB) ReadOnly(ctx context.Context, f func(context.Context, database.Transaction) error) error {
	tx := db.client.ReadOnlyTransaction()
	return f(ctx, &spannerTx{r: tx})
}

// ReadWrite executes f in a read-write transaction.
func (db *DB) ReadWrite(ctx context.Context, f func(context.Context, database.Transaction) error) error {
	_, err := db.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		return f(ctx, &spannerTx{r: tx, w: tx})
	})
	return consistentError(err)
}

// A spannerTx is the underlying spanner transaction.
type spannerTx struct {
	r spannerTxR
	w spannerTxW // nil for read-only transaction
}

// spannerTxR is the common read-only interface provided
// by both *spanner.ReadOnlyTransaction and *spanner.ReadWriteTransaction.
type spannerTxR interface {
	Read(context.Context, string, spanner.KeySet, []string) *spanner.RowIterator
	ReadRow(context.Context, string, spanner.Key, []string) (*spanner.Row, error)
}

// spannerTxW is the additional read-write interface provided
// only by *spanner.ReadWriteTransaction.
type spannerTxW interface {
	BufferWrite([]*spanner.Mutation) error
}

// Read reads rows matching keys from the database.
func (tx *spannerTx) Read(ctx context.Context, table string, keys database.Keys, columns []string) database.Rows {
	return &spannerRows{tx.r.Read(ctx, table, toKeySet(keys), columns)}
}

// A spannerRows is an implementation of database.Rows backed by a Spanner row iterator.
type spannerRows struct {
	rows *spanner.RowIterator
}

func (r *spannerRows) Do(f func(database.Row) error) error {
	return r.rows.Do(func(r *spanner.Row) error {
		return f(r)
	})
}

// ReadRow reads a single row matching key from the database.
func (tx *spannerTx) ReadRow(ctx context.Context, table string, key database.Key, columns []string) (database.Row, error) {
	row, err := tx.r.ReadRow(ctx, table, toKey(key), columns)
	if err != nil {
		return nil, consistentError(err)
	}
	return row, nil
}

// BufferWrite buffers the given writes.
func (tx *spannerTx) BufferWrite(writes []database.Mutation) error {
	if tx.w == nil {
		return fmt.Errorf("readonly")
	}

	var swrites []*spanner.Mutation
	for _, m := range writes {
		var sm *spanner.Mutation
		switch m.Op {
		default:
			return fmt.Errorf("unknown mutation operation %q", m.Op)
		case database.Insert:
			sm = spanner.Insert(m.Table, m.Cols, m.Vals)
		case database.InsertOrUpdate:
			sm = spanner.InsertOrUpdate(m.Table, m.Cols, m.Vals)
		case database.Replace:
			sm = spanner.Replace(m.Table, m.Cols, m.Vals)
		case database.Update:
			sm = spanner.Update(m.Table, m.Cols, m.Vals)
		case database.Delete:
			sm = spanner.Delete(m.Table, toKeySet(m.Keys))
		}
		swrites = append(swrites, sm)
	}
	return tx.w.BufferWrite(swrites)
}

// consistentError maps err to an appropriate error for the database.DB interfaces.
func consistentError(err error) error {
	// Detect row not exist and turn it into database.RowNotExistError.
	if err != nil && spanner.ErrCode(err) == codes.NotFound {
		err = &database.RowNotExistError{Err: err}
	}
	return err
}

// toKey maps a database.Key to a spanner.Key.
// (The representations are the same.)
func toKey(key database.Key) spanner.Key {
	return []interface{}(key)
}

// toKeySet maps a database.Keys to a spanner.KeySet.
func toKeySet(keys database.Keys) spanner.KeySet {
	if keys.All {
		return spanner.AllKeys()
	}
	var skeys []spanner.KeySet
	for _, key := range keys.List {
		skeys = append(skeys, toKey(key))
	}
	return spanner.KeySets(skeys...)
}
