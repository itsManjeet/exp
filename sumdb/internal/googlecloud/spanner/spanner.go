// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package spanner implements tkv.Storage using Google Cloud Spanner.
package spanner

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/sumdb/internal/tkv"

	"cloud.google.com/go/spanner"
	admin "cloud.google.com/go/spanner/admin/database/apiv1"
	adminpb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"
	"google.golang.org/grpc/codes"
)

var _ tkv.Storage = (*Storage)(nil)

const tableName = "tkv"

// A Storage is a connection to Spanner storage, implementing tkv.Storage.
type Storage struct {
	name   string
	client *spanner.Client
	admin  *admin.DatabaseAdminClient
}

// Open opens the Spanner database with the given name
// (for example, "projects/my-project/instances/my-instance/databases/my_db").
// The database must already exist.
func OpenStorage(ctx context.Context, name string) (*Storage, error) {
	client, err := spanner.NewClient(ctx, name)
	if err != nil {
		return nil, err
	}
	s := &Storage{name: name, client: client}
	return s, nil
}

// CreateStorage creates a Spanner database with the given name
// (for example, "projects/my-project/instances/my-instance/databases/my_db").
// The database must not already exist.
func CreateStorage(ctx context.Context, name string) (*Storage, error) {
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

	req1 := &adminpb.UpdateDatabaseDdlRequest{
		Database:   name,
		Statements: []string{"CREATE TABLE `" + tableName + "` (`key` STRING (MAX), `value` STRING (MAX)) PRIMARY KEY (`key`)\n"},
	}
	op1, err := adminClient.UpdateDatabaseDdl(ctx, req1)
	if err != nil {
		return nil, err
	}
	if err := op1.Wait(ctx); err != nil {
		return nil, err
	}

	client, err := spanner.NewClient(ctx, name)
	if err != nil {
		return nil, err
	}
	s := &Storage{name: name, client: client}
	return s, nil
}

// DeleteTestStorage deletes the Spaner database with the given name.
// (for example, "projects/my-project/instances/my-instance/databases/test_my_db").
// To avoid unfortunate accidents, DeleteTestDB returns an error
// if the database name does not begin with "test_".
func DeleteTestStorage(ctx context.Context, name string) error {
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

// ReadOnly executes f in a read-only transaction.
func (s *Storage) ReadOnly(ctx context.Context, f func(context.Context, tkv.Transaction) error) error {
	tx := s.client.ReadOnlyTransaction()
	return f(ctx, &spannerTx{r: tx})
}

// ReadWrite executes f in a read-write transaction.
func (s *Storage) ReadWrite(ctx context.Context, f func(context.Context, tkv.Transaction) error) error {
	_, err := s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		return f(ctx, &spannerTx{r: tx, w: tx})
	})
	return err
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

func (tx *spannerTx) ReadValue(ctx context.Context, key string) (string, error) {
	row, err := tx.r.ReadRow(ctx, tableName, spanner.Key{key}, []string{"value"})
	if err != nil && spanner.ErrCode(err) == codes.NotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var v string
	if err := row.Column(0, &v); err != nil {
		return "", err
	}
	return v, nil
}

func (tx *spannerTx) ReadValues(ctx context.Context, keys []string) ([]string, error) {
	var skeys []spanner.KeySet
	for _, k := range keys {
		skeys = append(skeys, spanner.Key{k})
	}
	m := make(map[string]string)
	err := tx.r.Read(ctx, tableName, spanner.KeySets(skeys...), []string{"key", "value"}).Do(func(r *spanner.Row) error {
		var k, v string
		if err := r.Column(0, &k); err != nil {
			return err
		}
		if err := r.Column(1, &v); err != nil {
			return err
		}
		m[k] = v
		return nil
	})
	if err != nil {
		return nil, err
	}

	vals := make([]string, len(keys))
	for i, k := range keys {
		vals[i] = m[k]
	}
	return vals, nil
}

func (tx *spannerTx) BufferWrites(writes []tkv.Write) error {
	if tx.w == nil {
		return fmt.Errorf("readonly")
	}

	var swrites []*spanner.Mutation
	for _, w := range writes {
		var m *spanner.Mutation
		if w.Value == "" {
			m = spanner.Delete(tableName, spanner.Key{w.Key})
		} else {
			m = spanner.InsertOrUpdate(tableName, []string{"key", "value"}, []interface{}{w.Key, w.Value})
		}
		swrites = append(swrites, m)
	}
	return tx.w.BufferWrite(swrites)
}
