// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spandb

import (
	"context"
	"flag"
	"testing"

	"golang.org/x/exp/notary/internal/consistent"
)

var testInstance = flag.String("spanner", "", "test spanner instance (projects/xxx/instances/yyy)")

func TestSpannerDB(t *testing.T) {
	if *testInstance == "" {
		t.Skip("no test instance given in -spanner flag")
	}

	// Test basic spanner operations
	// (exercising interface wrapper, not spanner itself).
	ctx := context.Background()

	DeleteTestDB(ctx, *testInstance+"/databases/test_spandb")
	db, err := CreateDB(ctx, *testInstance+"/databases/test_spandb")
	if err != nil {
		t.Fatal(err)
	}
	_ = (consistent.DB)(db)
	defer DeleteTestDB(ctx, *testInstance+"/databases/test_spandb")

	// Create table.
	err = db.CreateTables(ctx, []*consistent.Table{
		{
			Name: "t1",
			Columns: []consistent.Column{
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

	// Insert some records.
	err = db.ReadWrite(ctx, func(ctx context.Context, tx consistent.Transaction) error {
		for i := int64(0); i < 10; i++ {
			err := tx.BufferWrite([]consistent.Mutation{
				{
					Table: "t1",
					Op:    consistent.Insert,
					Cols:  []string{"c1", "c2"},
					Vals:  []interface{}{i, -i},
				},
				{
					Table: "t1",
					Op:    consistent.Insert,
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

	// Read them back.
	err = db.ReadOnly(ctx, func(ctx context.Context, tx consistent.Transaction) error {
		for i := int64(0); i < 1010; i++ {
			if i == 10 {
				i = 1000
			}
			row, err := tx.ReadRow(ctx, "t1", consistent.Key{-i}, []string{"c1"})
			if err != nil {
				tx.Read(ctx, "t1", consistent.Keys{All: true}, []string{"c1", "c2"}, 0).Do(func(r consistent.Row) error {
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

		err := tx.Read(ctx, "t1", consistent.Keys{List: []consistent.Key{{-999}, {0}}}, []string{"c1", "c2"}, 0).Do(func(r consistent.Row) error {
			var i, j int64
			r.Column(0, &i)
			r.Column(1, &j)
			t.Logf("%v\t%v", i, j)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
