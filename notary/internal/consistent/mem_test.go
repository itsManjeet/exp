// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package consistent

import (
	"context"
	"testing"
)

func TestMemDB(t *testing.T) {
	// Basic memory database test.
	ctx := context.Background()
	db := new(MemDB)
	_ = DB(db)
	err := db.CreateTables(ctx, []*Table{
		{
			Name: "t1",
			Columns: []Column{
				{
					Name: "c1",
					Type: "int64",
				},
				{
					Name: "c2",
					Type: "int64",
				},
			},
			PrimaryKey: []string{"c2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert records in different orders.
	err = db.ReadWrite(ctx, func(ctx context.Context, tx Transaction) error {
		for i := int64(0); i < 10; i++ {
			err := tx.BufferWrite([]Mutation{
				{
					Table: "t1",
					Op:    Insert,
					Cols:  []string{"c1", "c2"},
					Vals:  []interface{}{i, -i},
				},
				{
					Table: "t1",
					Op:    Insert,
					Cols:  []string{"c2", "c1"},
					Vals:  []interface{}{-1000 - i, 1000 + i},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Read the records back.
	err = db.ReadOnly(ctx, func(ctx context.Context, tx Transaction) error {
		for i := int64(0); i < 1010; i++ {
			if i == 10 {
				i = 1000
			}
			row, err := tx.ReadRow(ctx, "t1", Key{-i}, []string{"c1"})
			if err != nil {
				tx.Read(ctx, "t1", Keys{All: true}, []string{"c1", "c2"}).Do(func(r Row) error {
					var i, j int64
					r.Column(0, &i)
					r.Column(1, &j)
					t.Logf("%v\t%v", i, j)
					return nil
				})
				t.Fatalf("reading %v: %v", -i, err)
			}
			var j int64
			err = row.Column(0, &j)
			if err != nil {
				t.Fatalf("reading %v column 0: %v", -i, err)
			}
			if j != i {
				t.Errorf("reading %v column 0: have %v, want %v", -i, j, i)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadNoKey(t *testing.T) {
	// Test that Read of non-existent key does not return an error.
	ctx := context.Background()
	db := new(MemDB)
	err := db.CreateTables(ctx, []*Table{
		{
			Name: "t1",
			Columns: []Column{
				{
					Name: "key",
					Type: "int64",
				},
				{
					Name: "value",
					Type: "string",
				},
			},
			PrimaryKey: []string{"key"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.ReadWrite(ctx, func(ctx context.Context, tx Transaction) error {
		return tx.BufferWrite([]Mutation{
			{Table: "t1", Op: Insert, Cols: []string{"key", "value"}, Vals: []interface{}{int64(1), "hello"}},
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.ReadOnly(ctx, func(ctx context.Context, tx Transaction) error {
		var v string
		row, err := tx.ReadRow(ctx, "t1", Key{int64(1)}, []string{"value"})
		if err != nil {
			t.Fatal(err)
		}
		err = row.Column(0, &v)
		if err != nil {
			t.Fatal(err)
		}
		if v != "hello" {
			t.Fatalf("read t1: got %q want %q", v, "hello")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
