package notedisk

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/exp/notary/internal/tlog"
)

func tmpdir(t *testing.T) (dir string, cleanup func()) {
	dir, err := ioutil.TempDir("", "notedisk-test-")
	if err != nil {
		t.Fatal(err)
	}
	return dir, func() {
		os.RemoveAll(dir)
	}
}

func tmpdb(t testing.TB) (db *DB, cleanup func()) {
	dir, err := ioutil.TempDir("", "diskstore-test-")
	if err != nil {
		t.Fatal(err)
	}
	db, err = Create(filepath.Join(dir, "notary.db"))
	if err != nil {
		t.Fatal(err)
	}
	return db, func() {
		db.Close()
		os.RemoveAll(dir)
	}
}

func TestDB(t *testing.T) {
	db, cleanup := tmpdb(t)
	defer cleanup()

	db2, err := Open(db.Name())
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		id, err := db.Add([]byte(fmt.Sprintf("content #%d", i)), fmt.Sprintf("key #%d", i))
		if err != nil {
			t.Fatalf("Add(%d): %v", i, err)
		}
		if id != int64(i) {
			t.Fatalf("Add(%d): unexpected id %d", i, id)
		}
		if n := db.NumRecords(); n != int64(i+1) {
			t.Fatalf("NumRecords() = %d, want %d", n, i+1)
		}
	}
	wrote := time.Now()

	for i := 0; i < 10; i++ {
		id, err := db.FindHash(tlog.RecordHash([]byte(fmt.Sprintf("content #%d", i))))
		if err != nil {
			t.Fatalf("FindHash(%d): %v", i, err)
		}
		if id != int64(i) {
			t.Fatalf("FindHash(%d): unexpected id %d", i, id)
		}
	}

	for i := 0; i < 10; i++ {
		id, err := db.FindKey(fmt.Sprintf("key #%d", i))
		if err != nil {
			t.Fatalf("FindKey(%d): %v", i, err)
		}
		if id != int64(i) {
			t.Fatalf("FindKey(%d): unexpected id %d", i, id)
		}
	}

	for i := 0; i < 10; i++ {
		r, err := db.ReadContent(int64(i))
		if err != nil {
			t.Fatalf("ReadContent(%d): %v", i, err)
		}
		data, err := ioutil.ReadAll(r)
		if err != nil {
			t.Fatalf("ReadContent(%d): reading: %v", i, err)
		}
		want := fmt.Sprintf("content #%d", i)
		if string(data) != want {
			t.Fatalf("ReadContent(%d) = %q, want %q", i, data, want)
		}
	}

	p, err := tlog.ProveRecord(9, 2, db)
	if err != nil {
		t.Fatal(err)
	}
	thash, err := tlog.TreeHash(9, db)
	if err != nil {
		t.Fatal(err)
	}
	rhash := tlog.RecordHash([]byte("content #2"))
	if err := tlog.CheckRecord(p, 9, thash, 2, rhash); err != nil {
		t.Fatal(err)
	}

	// Check that records appear in other open database
	// after a suitable interval.
	time.Sleep(time.Until(wrote.Add(101 * time.Second / 100)))
	for i := 0; i < 10; i++ {
		id, err := db2.FindKey(fmt.Sprintf("key #%d", i))
		if err != nil {
			t.Fatalf("db2.FindKey(%d): %v", i, err)
		}
		if id != int64(i) {
			t.Fatalf("db2.FindKey(%d): unexpected id %d", i, id)
		}
	}
}

func BenchmarkAdd(b *testing.B) {
	db, cleanup := tmpdb(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Add([]byte(fmt.Sprintf("content #%d", i)), fmt.Sprintf("key #%d", i))
	}
	b.StopTimer()
}
