// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package notedb

import (
	"context"
	"flag"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/exp/notary/internal/consistent"
	"golang.org/x/exp/notary/internal/consistent/spandb"
	"golang.org/x/exp/notary/internal/tlog"
)

var testInstance = flag.String("spanner", "", "test spanner instance (projects/xxx/instances/yyy)")

func TestDB(t *testing.T) {
	ctx := context.Background()

	// Use in-memory database for quick testing,
	// unless -spanner flag is given.
	var cdb consistent.DB
	if *testInstance == "" {
		cdb = new(consistent.MemDB)
	} else {
		if err := spandb.DeleteTestDB(ctx, *testInstance+"/databases/test_notedb"); err != nil {
			t.Fatal(err)
		}
		db, err := spandb.CreateDB(ctx, *testInstance+"/databases/test_notedb")
		if err != nil {
			t.Fatal(err)
		}
		defer spandb.DeleteTestDB(ctx, *testInstance+"/databases/test_notedb")
		cdb = db
	}

	db, err := Create(ctx, cdb)
	if err != nil {
		t.Fatal(err)
	}

	db2, err := Open(ctx, cdb)
	if err != nil {
		t.Fatal(err)
	}

	// Test writes some number in sequence, then some number in parallel, then some number in a batch.
	// 10 is usually fine for shaking out problems, but can turn up for basic timings.
	const (
		NumSeq   = 10
		NumPar   = 10
		NumBatch = 10
	)
	var seq [NumSeq + NumPar + NumBatch]int64
	runtime.GOMAXPROCS(len(seq))

	newRecord := func(i int) NewRecord {
		return NewRecord{
			Key: fmt.Sprintf("key #%d", i), Content: []byte(fmt.Sprintf("content #%d", i)),
		}
	}
	add1 := func(i, expect int) {
		start := time.Now()
		r := []NewRecord{newRecord(i)}
		err := db.Add(ctx, r)
		if err != nil {
			t.Fatalf("Add(%d): %v", i, err)
		}
		if r[0].Err != nil {
			t.Fatalf("Add(%d): %v", i, r[0].Err)
		}
		id := r[0].ID
		t.Logf("add: %.3fs\n", time.Since(start).Seconds())
		if expect >= 0 && id != int64(expect) {
			t.Fatalf("Add(%d): unexpected id %d", i, id)
		}
		seq[i] = id
		if expect >= 0 {
			start = time.Now()
			if n, err := db.NumRecords(ctx); err != nil || n != int64(expect+1) {
				t.Fatalf("NumRecords() = %d, %v; want %d, nil", n, err, expect+1)
			}
			t.Logf("numRecords: %.3fs\n", time.Since(start).Seconds())
		}
	}

	// Sequential.
	i := 0
	for ; i < NumSeq; i++ {
		add1(i, i)
	}

	// Parallel.
	start := time.Now()
	var wg sync.WaitGroup
	for ; i < NumSeq+NumPar; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			add1(i, -1)
		}(i)
	}
	wg.Wait()
	t.Logf("add%d: %.3fs %v\n", NumPar, time.Since(start).Seconds(), seq[NumSeq:NumSeq+NumPar])

	// Batch.
	start = time.Now()
	var recs []NewRecord
	for ; i < NumSeq+NumPar+NumBatch; i++ {
		recs = append(recs, newRecord(i))
	}
	err = db.Add(ctx, recs)
	if err != nil {
		t.Fatalf("Add(batch): %v", err)
	}
	for i, r := range recs {
		if r.Err != nil {
			t.Fatalf("Add(%d): %v", NumSeq+NumPar+i, r.Err)
		}
		seq[NumSeq+NumPar+i] = r.ID
	}
	t.Logf("add%d: %.3fs\n", NumBatch, time.Since(start).Seconds())

	// Read records back.
	start = time.Now()
	if n, err := db.NumRecords(ctx); err != nil || n != int64(len(seq)) {
		t.Fatalf("NumRecords() = %d, %v; want %d, nil", n, err, len(seq))
	}
	t.Logf("numRecords: %.3fs\n", time.Since(start).Seconds())

	// First by key.
	for i := 0; i < len(seq); i++ {
		start := time.Now()
		id, err := db.FindKey(ctx, fmt.Sprintf("key #%d", i))
		if err != nil {
			t.Fatalf("FindKey(%d => %d): %v", i, seq[i], err)
		}
		if id != seq[i] {
			t.Fatalf("FindKey(%d => %d): unexpected id %d", i, seq[i], id)
		}
		_ = start
		if *testInstance != "" {
			i += 100
		}
	}

	// Next by index.
	for i := 0; i < len(seq); i++ {
		start := time.Now()
		data, err := db.ReadContent(ctx, seq[i])
		if err != nil {
			t.Fatalf("ReadContent(%d): %v", i, err)
		}
		want := fmt.Sprintf("content #%d", i)
		if string(data) != want {
			t.Fatalf("ReadContent(%d) = %q, want %q", i, data, want)
		}
		t.Logf("ReadContent: %.3fs\n", time.Since(start).Seconds())
		if *testInstance != "" {
			i += 99
		}
	}

	// Check that proofs can be carried out.
	start = time.Now()
	p, err := db.ProveRecord(ctx, 9, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("proveRecord: %.3fs\n", time.Since(start).Seconds())
	start = time.Now()
	thash, err := db.TreeHash(ctx, 9)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("treeHash: %.3fs\n", time.Since(start).Seconds())
	rhash := tlog.RecordHash([]byte("content #2"))
	if err := tlog.CheckRecord(p, 9, thash, 2, rhash); err != nil {
		t.Fatal(err)
	}

	// Check that db2 sees all the same data.
	for i := 0; i < 10; i++ {
		id, err := db2.FindKey(ctx, fmt.Sprintf("key #%d", i))
		if err != nil {
			t.Fatalf("db2.FindKey(%d): %v", i, err)
		}
		if id != seq[i] {
			t.Fatalf("db2.FindKey(%d): unexpected id %d want %d", i, id, seq[i])
		}
	}
}
