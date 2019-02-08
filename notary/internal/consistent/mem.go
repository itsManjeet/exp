// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package consistent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// MemDB returns a new in-memory implementation of DB.
// The implementation is meant for tests and does not store
// any data to persistent storage.
//
// The zero value is a MemDB ready for use.
type MemDB struct {
	mu     sync.RWMutex
	tables map[string]*memTable
}

// A memTable is a single table in a MemDB.
type memTable struct {
	cfg  *Table
	rows treeMap
	keyx []int // indexes of primary key columns
}

// A memTx is a transaction in a MemDB.
type memTx struct {
	db         *MemDB   // database
	writes     []func() // buffered writes for end of transaction
	commitTime time.Time
}

// CreateTables creates the list of tables.
// It is an error to create a table that already exists.
// If there is any error, no tables are created.
func (db *MemDB) CreateTables(ctx context.Context, tables []*Table) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	var mtables []*memTable
	for _, t := range tables {
		if db.tables[t.Name] != nil {
			return fmt.Errorf("table %s already exists", t.Name)
		}
		mt := &memTable{
			cfg: cloneTable(t),
		}
		mt.rows.KeyCmp = mt.keyCmp
		mtables = append(mtables, mt)
	Names:
		for _, name := range t.PrimaryKey {
			for j, c := range t.Columns {
				if c.Name == name {
					mt.keyx = append(mt.keyx, j)
					continue Names
				}
			}
			var cnames []string
			for _, c := range t.Columns {
				cnames = append(cnames, c.Name)
			}
			return fmt.Errorf("table %s primary key %q is not a column (%q)", t.Name, name, cnames)
		}
	}

	// Tables are all OK; add them.
	if db.tables == nil {
		db.tables = make(map[string]*memTable)
	}
	for _, mt := range mtables {
		db.tables[mt.cfg.Name] = mt
	}
	return nil
}

// cloneTable makes a deep copy of t,
// sharing no memory with the original.
func cloneTable(t *Table) *Table {
	t1 := *t
	t1.Columns = append([]Column(nil), t1.Columns...)
	t1.PrimaryKey = append([]string(nil), t1.PrimaryKey...)
	return &t1
}

// errRetry is an internal sentinel indicating that the transaction should be retried.
// It is never returned to the caller.
var errRetry = errors.New("retry")

// ReadOnly runs f in a read-only transaction.
func (db *MemDB) ReadOnly(ctx context.Context, f func(context.Context, Transaction) error) error {
	tx := &memTx{db: db}
	for {
		err := func() error {
			db.mu.Lock()
			defer db.mu.Unlock()

			if err := f(ctx, tx); err != nil {
				return err
			}
			// Spurious retry with 10% probability.
			if rand.Intn(10) == 0 {
				return errRetry
			}
			return nil
		}()
		if err != errRetry {
			return err
		}
	}
}

// ReadWrite runs f in a read-write transaction.
func (db *MemDB) ReadWrite(ctx context.Context, f func(context.Context, Transaction) error) error {

	tx := &memTx{db: db}
	for {
		err := func() error {
			db.mu.Lock()
			defer db.mu.Unlock()

			tx.writes = []func(){}
			if err := f(ctx, tx); err != nil {
				return err
			}
			// Spurious retry with 10% probability.
			if rand.Intn(10) == 0 {
				return errRetry
			}
			for _, write := range tx.writes {
				write()
			}
			return nil
		}()
		if err != errRetry {
			return err
		}
	}
}

// A memRow is a single row result in a MemDB.
// It is a subset of the columns from that row,
// in a caller-specified order.
type memRow struct {
	cols []Column // column order
	colx []int    // vals[colx[i]] is for cols[i]
	vals values   // values from entire row
}

// values is an internal type used for the in-memory database values,
// to make printing those values nicer.
type values []interface{}

func (v values) String() string {
	return Key(v).String()
}

// A rowFunc is an implementation of Rows by a plain function (providing the Do method).
type rowFunc func(func(Row) error) error

func (r rowFunc) Do(f func(Row) error) error {
	return r(f)
}

// Read returns the requested columns from the matching rows in the table.
func (tx *memTx) Read(ctx context.Context, table string, keys Keys, columns []string) Rows {
	used := false
	return rowFunc(func(f func(Row) error) error {
		if used {
			return fmt.Errorf("Rows.Do can only be used once")
		}
		used = true
		t := tx.db.tables[table]
		if t == nil {
			return fmt.Errorf("no such table %s", table)
		}
		cols, colx, err := t.cols(columns)
		if err != nil {
			return err
		}

		if keys.All {
			err = t.rows.Visit(func(key, value interface{}) error {
				row := &memRow{cols, colx, value.(values)}
				return f(row)
			})
		} else {
			for _, key := range keys.List {
				// Note that this loop ends up with the rows not sorted by key.
				// The Transaction interface says this is OK, but if existing
				// databases always return the rows by key, we probably should
				// strengthen the interface.
				if value := t.rows.Lookup(key); value != nil {
					row := &memRow{cols, colx, value.(values)}
					if err = f(row); err != nil {
						break
					}
				}
			}
		}
		return err
	})
}

// ReadRow reads from table and returns the single row with the given key.
// Only the listed columns are retrieved.
func (tx *memTx) ReadRow(ctx context.Context, table string, key Key, columns []string) (Row, error) {
	var row Row
	rows := tx.Read(ctx, table, Keys{List: []Key{key}}, columns)
	err := rows.Do(func(r Row) error {
		row = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

// Column stores the i'th column (indexed relative to the column list
// specified in Read or ReadRow) from row and stores it in dst.
func (r *memRow) Column(index int, dst interface{}) error {
	if index < 0 || index >= len(r.cols) {
		var names []string
		for _, c := range r.cols {
			names = append(names, c.Name)
		}
		return fmt.Errorf("column index %d out of range (have %v)", index, names)
	}
	col := &r.cols[index]
	src := r.vals[r.colx[index]]
	switch col.Type {
	default:
		return fmt.Errorf("unexpected column type %v", col.Type)

	case "int64":
		switch dst := dst.(type) {
		default:
			goto BadDst
		case *int64:
			switch src := src.(type) {
			default:
				goto BadSrc
			case int64:
				*dst = src
			}
		}

	case "bytes":
		switch dst := dst.(type) {
		default:
			goto BadDst
		case *[]byte:
			switch src := src.(type) {
			default:
				goto BadSrc
			case []byte:
				*dst = src
			case nil:
				*dst = nil
			}
		}

	case "string":
		switch dst := dst.(type) {
		default:
			goto BadDst
		case *string:
			switch src := src.(type) {
			default:
				goto BadSrc
			case string:
				*dst = src
			}
		}

	case "timestamp":
		switch dst := dst.(type) {
		default:
			goto BadDst
		case *time.Time:
			switch src := src.(type) {
			default:
				goto BadSrc
			case time.Time:
				*dst = src
			}
		}
	}
	return nil

BadDst:
	return fmt.Errorf("column %v of type %v: unexpected destination type %T", col.Name, col.Type, dst)
BadSrc:
	return fmt.Errorf("column %v of type %v: unexpected source type %T", col.Name, col.Type, src)
}

// cols returns the Column descriptions corresponding to names,
// along with the indexes of those columns in the original table column order.
// That is, the value for the column named names[i] is row.vals[colx[i]].
func (t *memTable) cols(names []string) (cols []Column, colx []int, err error) {
	cols = make([]Column, len(names))
	colx = make([]int, len(names))
Names:
	for i, name := range names {
		for j, c := range t.cfg.Columns {
			if c.Name == name {
				cols[i] = c
				colx[i] = j
				continue Names
			}
		}
		return nil, nil, fmt.Errorf("table %s has no column %s", t.cfg.Name, name)
	}
	return cols, colx, nil
}

// BufferWrite buffers a list of mutations to be applied
// to the table when the transaction commits.
// The changes are not visible to reads within the transaction.
func (tx *memTx) BufferWrite(ms []Mutation) error {
	if tx.writes == nil {
		panic("BufferWrite on read-only transaction")
	}
	for _, m := range ms {
		table := tx.db.tables[m.Table]
		if table == nil {
			return fmt.Errorf("no such table %s", m.Table)
		}
		_, colx, err := table.cols(m.Cols)
		if err != nil {
			return err
		}
		switch m.Op {
		default:
			return fmt.Errorf("table %s: unsupported operation %q", m.Table, m.Op)
		case Insert, InsertOrUpdate, Replace, Update:
			vals := make(values, len(table.cfg.Columns))
			for i := range m.Vals {
				vals[colx[i]] = m.Vals[i]
			}
			key := make(Key, len(table.keyx))
			for i, x := range table.keyx {
				if vals[x] == nil && table.cfg.Columns[x].NotNull {
					return fmt.Errorf("table %s %s: primary key column %s missing, must be non-null", m.Table, m.Op, table.cfg.Columns[x].Name)
				}
				key[i] = vals[x]
			}

			oldVals := table.rows.Lookup(key)
			if m.Op == Insert && oldVals != nil {
				return fmt.Errorf("table %s %s %v: row already exists", m.Table, m.Op, key)
			}
			if m.Op == Update && oldVals == nil {
				return fmt.Errorf("table %s %s %v: row does not exist", m.Table, m.Op, key)
			}
			if oldVals != nil {
				copy(vals, oldVals.(values))
				for i := range m.Vals {
					vals[colx[i]] = m.Vals[i]
				}
			}
			for i, v := range vals {
				col := table.cfg.Columns[i]
				if v == nil && col.NotNull {
					return fmt.Errorf("table %s %s %v: column %s missing, must be non-null", m.Table, m.Op, key, table.cfg.Columns[i].Name)
				}
				ok := false
				switch col.Type {
				case "int64":
					switch v.(type) {
					case int64:
						ok = true
					}
				case "string":
					switch v.(type) {
					case string:
						ok = true
					}
				case "bytes":
					switch v.(type) {
					case []byte, nil:
						ok = true
					}
				case "timestamp":
					switch v := v.(type) {
					case time.Time:
						ok = true
						v = v.UTC()
						vals[i] = v
					}
				}
				if !ok {
					return fmt.Errorf("table %s %s %v: column %s of type %s has unexpected Go value of type %T", m.Table, m.Op, key, col.Name, col.Type, v)
				}

			}

			tx.writes = append(tx.writes, func() {
				table.rows.Insert(key, vals)
			})

		case Delete:
			if m.Keys.All {
				tx.writes = append(tx.writes, func() {
					table.rows.DeleteAll()
				})
			} else {
				var copyKeys []Key
				for _, key := range m.Keys.List {
					copyKeys = append(copyKeys, append(Key{}, key...))
				}
				tx.writes = append(tx.writes, func() {
					for _, key := range copyKeys {
						table.rows.Delete(key)
					}
				})
			}
		}
	}
	return nil
}

// keyCmp compares the two keys key1 and key2
// for ordering a pair of rows.
func (t *memTable) keyCmp(ikey1, ikey2 interface{}) int {
	key1 := ikey1.(Key)
	key2 := ikey2.(Key)
	if len(key1) != len(key2) {
		panic("misuse of keyCmp")
	}
	for i := range key1 {
		k1 := key1[i]
		k2 := key2[i]
		if k1 == nil && k2 == nil {
			continue
		}
		if k1 == nil {
			return -1
		}
		if k2 == nil {
			return +1
		}

		switch k1 := k1.(type) {
		case int64:
			k2 := k2.(int64)
			if k1 < k2 {
				return -1
			}
			if k1 > k2 {
				return +1
			}

		case string:
			cmp := strings.Compare(k1, k2.(string))
			if cmp != 0 {
				return cmp
			}
		case []byte:
			cmp := bytes.Compare(k1, k2.([]byte))
			if cmp != 0 {
				return cmp
			}
		}
	}
	return 0
}
