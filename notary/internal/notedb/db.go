// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package notedb implements database-backed storage for notary data.
//
// This package is part of a DRAFT of what the Go module notary will look like.
// Do not assume the details here are final!
//
// This package assumes access to an underlying database that provides
// external consistency guarantees, such as a single-server database or
// Google Cloud Spanner.
package notedb

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/exp/notary/internal/database"
	"golang.org/x/exp/notary/internal/tlog"
)

// A DB is a notary database.
type DB struct {
	database database.DB
}

// schema is the tables we use in the database.DB
var schema = []*database.Table{
	{
		// hashes maps storage_id -> hash.
		// It is the storage for tlog.HashReader.
		Name: "hashes",
		Columns: []database.Column{
			{Name: "storage_id", Type: "int64", NotNull: true},
			{Name: "hash", Type: "bytes", NotNull: true},
		},
		PrimaryKey: []string{"storage_id"},
	},
	{
		// meta maps arbitrary key -> value.
		// It holds notary configuration settings (see Config and SetConfig)
		// and also key = "treesize" holds a decimal string giving the size of the tree.
		Name: "meta",
		Columns: []database.Column{
			{Name: "key", Type: "string", NotNull: true},
			{Name: "value", Type: "string", NotNull: true},
		},
		PrimaryKey: []string{"key"},
	},
	{
		// modules maps hash("module@version") (modverhash)
		// to hash(record data) (recordhash) and the record's level 0 id in the notary log.
		// The recordhash is not strictly necessary—we could turn id into a
		// storage_id and look it up in hashes instead—but it is convenient
		// when checking for inconsistent writes in Add.
		Name: "modules",
		Columns: []database.Column{
			{Name: "modverhash", Type: "bytes", Size: 128, NotNull: true},
			{Name: "recordhash", Type: "bytes", Size: 128, NotNull: true},
			{Name: "id", Type: "int64", NotNull: true},
		},
		PrimaryKey: []string{"modverhash"},
	},
	{
		// records maps hash(record data) to record data.
		// It is not strictly necessary—we could store it directly in the modules table,
		// and perhaps we should. Separating it out makes it possible to support
		// lookup by record hash, like in CT, but we don't plan to do that at the start.
		Name: "records",
		Columns: []database.Column{
			{Name: "recordhash", Type: "bytes", Size: 128, NotNull: true},
			{Name: "record", Type: "bytes", Size: 64 * 1024, NotNull: true},
		},
		PrimaryKey: []string{"recordhash"},
	},
}

// Create initializes a new notary database given an empty database.DB.
func Create(ctx context.Context, db database.DB) (*DB, error) {
	if err := db.CreateTables(ctx, schema); err != nil {
		return nil, err
	}
	ndb := &DB{database: db}
	err := db.ReadWrite(ctx, func(ctx context.Context, tx database.Transaction) error {
		// Note: Not using db.writeTreeSize because we want to insert a fresh value and fail if one already exists.
		return tx.BufferWrite([]database.Mutation{{
			Op: database.Insert, Table: "meta",
			Cols: []string{"key", "value"},
			Vals: []interface{}{"treesize", "0"},
		}})
	})
	if err != nil {
		return nil, err
	}
	return ndb, nil
}

// Open opens an existing notary database stored in a database.DB.
func Open(ctx context.Context, db database.DB) (*DB, error) {
	// Check that the database is initialized,
	// by reading the tree size.
	ndb := &DB{database: db}
	err := db.ReadOnly(ctx, func(ctx context.Context, tx database.Transaction) error {
		_, err := ndb.readTreeSize(ctx, tx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("invalid database: %v", err)
	}
	return ndb, nil
}

// NumRecords returns the number of records in the database.
func (db *DB) NumRecords(ctx context.Context) (int64, error) {
	var size int64
	err := db.database.ReadOnly(ctx, func(ctx context.Context, tx database.Transaction) error {
		var err error
		size, err = db.readTreeSize(ctx, tx)
		return err
	})
	if err != nil {
		return 0, err
	}
	return size, nil
}

// writeTreeSize buffers a write of the tree size within a transaction.
func (db *DB) writeTreeSize(ctx context.Context, tx database.Transaction, size int64) error {
	return tx.BufferWrite([]database.Mutation{{
		Op: database.Update, Table: "meta",
		Cols: []string{"key", "value"},
		Vals: []interface{}{"treesize", strconv.FormatInt(size, 10)},
	}})
}

// readTreeSize reads and returns the current tree size within a transaction.
func (db *DB) readTreeSize(ctx context.Context, tx database.Transaction) (int64, error) {
	row, err := tx.ReadRow(ctx, "meta", database.Key{"treesize"}, []string{"value"})
	if err != nil {
		return 0, err
	}
	var s string
	if err := row.Column(0, &s); err != nil {
		return 0, err
	}
	size, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return size, nil
}

// A hashMapLogger pretends the read and return hashes
// but actually just logs the indexes of the requested hashes in a map.
// We can run an algorithm like tlog.TreeHash once with a hashMapLogger
// to determine all the needed hashes, then read them all into a hashMap
// in one database read, and then rerun the algorithm with the retrieved
// hashes to get the final result.
type hashMapLogger map[int64]bool

func (h hashMapLogger) ReadHashes(indexes []int64) ([]tlog.Hash, error) {
	for _, x := range indexes {
		h[x] = true
	}
	return make([]tlog.Hash, len(indexes)), nil
}

// A hashMap implements tlog.HashReader using a fixed set of hashes in a map.
type hashMap map[int64]tlog.Hash

func (h hashMap) ReadHashes(indexes []int64) ([]tlog.Hash, error) {
	out := make([]tlog.Hash, len(indexes))
	for i, x := range indexes {
		th, ok := h[x]
		if !ok {
			return nil, fmt.Errorf("missing hash")
		}
		out[i] = th
	}
	return out, nil
}

// readHashesByIndex reads and returns the hashes with the given storage indexes.
func (db *DB) readHashesByIndex(ctx context.Context, indexes []int64) ([]tlog.Hash, error) {
	need := make(hashMapLogger)
	for _, x := range indexes {
		need[x] = true
	}
	var hashes hashMap
	err := db.database.ReadOnly(ctx, func(ctx context.Context, tx database.Transaction) error {
		var err error
		hashes, err = db.readHashesInTx(ctx, tx, need)
		return err
	})
	if err != nil {
		return nil, err
	}
	list := make([]tlog.Hash, len(indexes))
	for i, x := range indexes {
		list[i] = hashes[x]
	}
	return list, nil
}

// readHashes reads and returns the needed hashes.
func (db *DB) readHashes(ctx context.Context, need hashMapLogger) (hashMap, error) {
	var hashes hashMap
	err := db.database.ReadOnly(ctx, func(ctx context.Context, tx database.Transaction) error {
		var err error
		hashes, err = db.readHashesInTx(ctx, tx, need)
		return err
	})
	if err != nil {
		return nil, err
	}
	return hashes, nil
}

// readHashesInTx reads and returns the needed hashes, within an existing transaction.
func (db *DB) readHashesInTx(ctx context.Context, tx database.Transaction, need hashMapLogger) (hashMap, error) {
	var keys database.Keys
	for needID := range need {
		keys.List = append(keys.List, database.Key{needID})
	}
	hashes := make(hashMap)
	err := tx.Read(ctx, "hashes", keys, []string{"storage_id", "hash"}).Do(func(r database.Row) error {
		var storageID int64
		if err := r.Column(0, &storageID); err != nil {
			return err
		}
		hash, err := db.readTlogHash(r, 1)
		if err != nil {
			return err
		}
		hashes[storageID] = hash
		return nil
	})
	return hashes, err
}

// readTlogHash reads row r's index'th column (of type "bytes")
// as a tlog.Hash.
func (db *DB) readTlogHash(r database.Row, index int) (tlog.Hash, error) {
	var b []byte
	if err := r.Column(index, &b); err != nil {
		return tlog.Hash{}, err
	}
	var h tlog.Hash
	if len(b) != len(h) {
		return tlog.Hash{}, fmt.Errorf("wrong-size hash %d != %d", len(b), len(h))
	}
	copy(h[:], b)
	return h, nil
}

// readKeyHash reads row r's index'th column (of type "bytes")
// as a key hash (SHA256 hash).
func (db *DB) readKeyHash(r database.Row, index int) ([sha256.Size]byte, error) {
	var b []byte
	if err := r.Column(index, &b); err != nil {
		return [sha256.Size]byte{}, err
	}
	var h [sha256.Size]byte
	if len(b) != len(h) {
		return [sha256.Size]byte{}, fmt.Errorf("wrong-size hash %d != %d", len(b), len(h))
	}
	copy(h[:], b)
	return h, nil
}

func (db *DB) hashReader(ctx context.Context) tlog.HashReader {
	return tlog.HashReaderFunc(func(indexes []int64) ([]tlog.Hash, error) {
		return db.readHashesByIndex(ctx, indexes)
	})
}

// TreeHash returns the top-level tree hash for the tree with n records.
func (db *DB) TreeHash(ctx context.Context, n int64) (tlog.Hash, error) {
	return tlog.TreeHash(n, db.hashReader(ctx))
}

// ProveRecord returns the proof that the tree of size t contains the record with index n.
func (db *DB) ProveRecord(ctx context.Context, t, n int64) (tlog.RecordProof, error) {
	return tlog.ProveRecord(t, n, db.hashReader(ctx))
}

// ProveTree returns the proof that the tree of size t contains as a prefix
// all the records from the tree of smaller size n.
func (db *DB) ProveTree(ctx context.Context, t, n int64) (tlog.TreeProof, error) {
	return tlog.ProveTree(t, n, db.hashReader(ctx))
}

// ReadTileData reads the hashes for the tile t and returns the corresponding tile data.
func (db *DB) ReadTileData(ctx context.Context, t tlog.Tile) ([]byte, error) {
	return tlog.ReadTileData(t, db.hashReader(ctx))
}

// FindKey looks up a record by its associated key ("module@version"),
// returning the record ID.
func (db *DB) FindKey(ctx context.Context, key string) (int64, error) {
	keyHash := sha256.Sum256([]byte(key))
	var id int64
	err := db.database.ReadOnly(ctx, func(ctx context.Context, tx database.Transaction) error {
		id = -1
		row, err := tx.ReadRow(ctx, "modules", database.Key{keyHash[:]}, []string{"id"})
		if err == nil {
			return row.Column(0, &id)
		}
		return err
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Config returns the database configuration value for the given key.
func (db *DB) Config(ctx context.Context, key string) (string, error) {
	var v string
	err := db.database.ReadOnly(ctx, func(ctx context.Context, tx database.Transaction) error {
		row, err := tx.ReadRow(ctx, "meta", database.Key{"config." + key}, []string{"value"})
		if err != nil {
			return err
		}
		return row.Column(0, &v)
	})
	if err != nil {
		return "", err
	}
	return v, nil
}

// SetConfig sets the database configuration value for the given key to the value.
func (db *DB) SetConfig(ctx context.Context, key, value string) error {
	return db.database.ReadWrite(ctx, func(ctx context.Context, tx database.Transaction) error {
		return tx.BufferWrite([]database.Mutation{{
			Table: "meta",
			Op:    database.Replace,
			Cols:  []string{"key", "value"},
			Vals:  []interface{}{"config." + key, value},
		}})
	})
}

// ReadContent returns the content for the given record.
func (db *DB) ReadContent(ctx context.Context, id int64) ([]byte, error) {
	var data []byte
	err := db.database.ReadOnly(ctx, func(ctx context.Context, tx database.Transaction) error {
		row, err := tx.ReadRow(ctx, "hashes", database.Key{tlog.StoredHashIndex(0, id)}, []string{"hash"})
		if err != nil {
			return err
		}
		rhash, err := db.readTlogHash(row, 0)
		if err != nil {
			return err
		}
		row, err = tx.ReadRow(ctx, "records", database.Key{rhash[:]}, []string{"record"})
		if err != nil {
			return err
		}
		if err := row.Column(0, &data); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return data, nil

}

// maxKeyLen is the maximum length record key the database accepts.
const maxKeyLen = 4096

var errKeyTooLong = errors.New("key too long")

// A NewRecord tracks a new record to be added to the database.
// The caller initializes Key and Content, and Add sets ID and Err.
type NewRecord struct {
	Key     string // record key ("module@version")
	Content []byte // record content
	ID      int64  // record log ID sequence number
	Err     error  // error inserting record, if any
}

// A newRecord tracks additional information about a NewRecord during Add.
type newRecord struct {
	*NewRecord
	khash [sha256.Size]byte // hash of key
	rhash tlog.Hash         // record hash of record content
	next  *newRecord
	dup   bool // is duplicate of another record
}

// Add adds a new record with the given content and associated key.
// It returns the record id for the new record.
// If the key is the empty string, the new record has no key.
//
// If a record already exists with the same content and key,
// Add returns the record id for the existing record
// instead of adding a new one.
//
// If a record already exists with the same content but a different key,
// or the same key but different content,
// Add returns an error.
func (db *DB) Add(ctx context.Context, records []NewRecord) error {
	// Build list of records being written, with computed hashes.
	recs := make([]*newRecord, 0, len(records))
	byKeyHash := make(map[[sha256.Size]byte]*newRecord)
	for i := range records {
		r := &newRecord{NewRecord: &records[i]}
		r.khash = sha256.Sum256([]byte(r.Key))
		r.rhash = tlog.RecordHash(r.Content)
		recs = append(recs, r)
		if old := byKeyHash[r.khash]; old != nil {
			// Multiple writes of same record in one batch. Track all.
			if r.rhash != old.rhash {
				r.Err = fmt.Errorf("different content for preexisting record")
			}
			// Chain this record onto first record.
			r.next = old.next
			r.dup = true
			old.next = r
			continue
		}
		byKeyHash[r.khash] = r
	}

	// Add any new records to the tree in a single database transaction.
	err := db.database.ReadWrite(ctx, func(ctx context.Context, tx database.Transaction) error {
		// Clear state in case transaction is being retried.
		for _, r := range recs {
			r.ID = -1
			r.Err = nil
		}

		// Read existing entries for all requested module@versions.
		var keys database.Keys
		for _, r := range recs {
			if !r.dup {
				keys.List = append(keys.List, database.Key{r.khash[:]})
			}
		}
		nfound := 0
		err := tx.Read(ctx, "modules", keys, []string{"modverhash", "recordhash", "id"}).Do(func(r database.Row) error {
			// Parse column data.
			khash, err := db.readKeyHash(r, 0)
			if err != nil {
				return err
			}
			rhash, err := db.readTlogHash(r, 1)
			if err != nil {
				return err
			}
			var id int64
			if err := r.Column(2, &id); err != nil {
				return err
			}

			// Record preexisting ID for duplicate write.
			rec := byKeyHash[khash]
			if rec == nil {
				return fmt.Errorf("unexpected key hash")
			}
			rec.ID = id
			if rhash != rec.rhash {
				rec.Err = fmt.Errorf("different content for preexisting record")
			}
			nfound++
			for dup := rec.next; dup != nil; dup = dup.next {
				dup.ID = rec.ID
				// Note dup.Err may be non-nil already, if dup.Content differs from rec.Content.
				if dup.Err == nil {
					dup.Err = rec.Err
				}
				nfound++
			}
			return nil
		})
		if err != nil {
			return err
		}

		// If we found all the records we were trying to write, we're done.
		if nfound == len(recs) {
			return nil
		}

		// Now that we know which records we're writing,
		// prepare new hashes for tree.
		treeSize, err := db.readTreeSize(ctx, tx)
		if err != nil {
			return err
		}
		storageID := tlog.StoredHashCount(treeSize)

		// To compute the new permanent hashes,
		// we need the existing hashes along the right-most fringe
		// of the tree. Those happen to be the same ones that
		// tlog.TreeHash needs, so use tlog.TreeHash to identify them.
		need := make(hashMapLogger)
		tlog.TreeHash(treeSize, need)

		// Read all those hashes in one read operation.
		hashes, err := db.readHashesInTx(ctx, tx, need)
		if err != nil {
			return err
		}

		// Queue the writes of the new records,
		// including their new permanent hashes.
		var writes []database.Mutation
		for _, rec := range recs {
			if rec.ID >= 0 || rec.dup {
				continue
			}
			rec.ID = treeSize
			for dup := rec.next; dup != nil; dup = dup.next {
				dup.ID = treeSize
			}
			treeSize++

			// Queue data writes.
			writes = append(writes,
				database.Mutation{
					Op: database.Replace, Table: "records",
					Cols: []string{"recordhash", "record"},
					Vals: []interface{}{rec.rhash[:], rec.Content},
				},
				database.Mutation{
					Op: database.Insert, Table: "modules",
					Cols: []string{"modverhash", "recordhash", "id"},
					Vals: []interface{}{rec.khash[:], rec.rhash[:], int64(rec.ID)},
				},
			)

			// Queue hash writes.
			toStore, err := tlog.StoredHashesForRecordHash(rec.ID, rec.rhash, hashes)
			if err != nil {
				return err
			}
			for _, h := range toStore {
				h := h
				writes = append(writes, database.Mutation{
					Table: "hashes",
					Op:    database.Replace,
					Cols:  []string{"storage_id", "hash"},
					Vals:  []interface{}{storageID, h[:]},
				})
				hashes[storageID] = h
				storageID++
			}

			// Sanity check; can't happen unless this code's logic is wrong.
			if storageID != tlog.StoredHashCount(treeSize) {
				return fmt.Errorf("out of sync %d %d", storageID, tlog.StoredHashCount(treeSize))
			}
		}
		if err := tx.BufferWrite(writes); err != nil {
			return err
		}

		// Record new tree size.
		return db.writeTreeSize(ctx, tx, treeSize)
	})
	if err != nil {
		return err
	}
	return nil
}
