// Package notedisk implements disk-based storage for notary data.
//
// This package is part of a DRAFT of what the Go module notary will look like.
// Do not assume the details here are final!
//
// This package is intended mainly as an example of a log storage layer,
// not a production implementation for a global notary log.
// Note in particular that DB maintains an in-memory index
// consuming approximately 80 bytes per log record.
package notedisk

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/exp/notary/tlog"
)

// DB implements a database for a basic transparent log.
// The data is stored in a directory of files, with the directory
// conventionally named notary.db.
//
// A DB is safe for concurrent use by multiple goroutines.
//
// A DB implements the tlog.HashReader interface and can be
// passed to functions such as tlog.TreeHash, tlog.ProveLeaf, and so on.
//
// Each record in the database can be retrieved by its sequence number
// (returned by Add), by its content hash (using FindHash),
// or by an associated string key (using FindKey) set when the record
// was added.
type DB struct {
	name string

	// write protects writing of hash, rec, toc, numRecord, and lastReload.
	// It is OK to use ReadAt on the files without holding the mutex
	write sync.Mutex
	hash  *os.File // hash list
	toc   *os.File // table of contents
	rec   *os.File // data sequence

	// numRecord is the number of records stored.
	// It should be read using atomic.LoadInt64.
	// It should be written using atomic.StoreInt64
	// but only when holding the db.write mutex.
	numRecord int64

	// lastReload is the Unix nano time of the last check for reload.
	// It should be read using atomic.LoadInt64.
	// It should be written using atomic.StoreInt64
	// but only when holding the db.write mutex.
	lastReload int64

	// config key-value pairs.
	// cfgMap is a map[string]string.
	// It can be read at any time but must be written while holding db.write.
	// db.write also protects cfgTime, cfgSize, and the writing of the config file.
	cfgMap  atomic.Value
	cfgTime time.Time // mtime of cfg at last read
	cfgSize int64     // size of cfg at last read

	// maps protects reading and writing of byHash and byKey,
	// which map 8-byte hashes to leaf numbers.
	maps   sync.RWMutex
	byHash map[uint64]int64 // map key is prefix of hash of data
	byKey  map[uint64]int64 // map key is prefix of hash of record key
}

func (db *DB) Name() string {
	return db.name
}

// Data Representation
//
// The database consists of four files.
//
// The notary.cfg file contains key-value config pairs, in the form "key: value\n".
//
// The notary.rec file stores the sequence of variable size records.
// Each record starts with three 8-byte integers: the record sequence number,
// the data size, and the key size. Those integers are followed directly
// by the data and key bytes.
//
// The notary.hash file stores a sequence of fixed-size hashes, almost two per record.
// There are more hashes than records because the file stores interior hashes
// for completed subtrees in the logging file. The hashes are ordered by
// tlog.StoredHashIndex.
//
// The notary.toc file stores a sequence of fixed-size table-of-contents entries, one per record.
// The table of contents entry consists of four 8-byte integers:
// a truncated SHA256 hash of the key (zero for no key),
// the offset to the start of the data record,
// the number of key bytes, and the number of data bytes.
//
// The toc file is the source of truth for how many records are stored in the log.
// That is, records are not committed until they are listed in the toc file.
// If a crash happens before the toc file is updated, any extra bytes at the
// end of the data and hash files will be ignored and eventually overwritten
// with new records.

const (
	hashSize      = len(tlog.Hash{})
	tocSize       = 32
	tocHashOffset = 24
	recHdrSize    = 24
)

// keyHash returns the uint64 record key hash.
func keyHash(key string) uint64 {
	h := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint64(h[:])
}

// Open opens an existing database.
func Open(name string) (*DB, error) {
	return open(name, false)
}

// Create creates a new database.
// It must not already exist.
func Create(name string) (*DB, error) {
	return open(name, true)
}

func open(name string, create bool) (db *DB, err error) {
	defer func() {
		// In case of failure, close the files.
		if err != nil {
			err = fmt.Errorf("database open %v: %v", name, err)
			if db != nil {
				db.Close()
				db = nil
			}
		}
	}()

	info, err := os.Stat(name)
	if err == nil && create {
		return nil, fmt.Errorf("%s already exists", name)
	}
	if err != nil && create {
		if err := os.Mkdir(name, 0777); err != nil {
			return nil, err
		}
		info, err = os.Stat(name)
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s exists but is not a notary database directory", name)
	}

	creat := 0
	if create {
		creat = os.O_CREATE
	}
	db = &DB{name: name}
	db.hash, err = os.OpenFile(filepath.Join(name, "notary.hash"), os.O_RDWR|creat, 0666)
	if err != nil {
		return nil, err
	}
	db.rec, err = os.OpenFile(filepath.Join(name, "notary.rec"), os.O_RDWR|creat, 0666)
	if err != nil {
		return nil, err
	}
	db.toc, err = os.OpenFile(filepath.Join(name, "notary.toc"), os.O_RDWR|creat, 0666)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(name, "notary.cfg"), os.O_RDWR|creat, 0666)
	if err != nil {
		return nil, err
	}
	f.Close()

	db.byHash = make(map[uint64]int64)
	db.byKey = make(map[uint64]int64)
	if err := db.reload(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) reload() error {
	const reloadInterval = 1e9 // 1 second

	if time.Now().UnixNano()-atomic.LoadInt64(&db.lastReload) < reloadInterval {
		return nil
	}

	db.write.Lock()
	defer db.write.Unlock()

	now := time.Now().UnixNano()
	if now-db.lastReload < reloadInterval {
		return nil
	}
	defer atomic.StoreInt64(&db.lastReload, now)

	cfgName := filepath.Join(db.name, "notary.cfg")
	info, err := os.Stat(cfgName)
	if info.ModTime() != db.cfgTime || info.Size() != db.cfgSize {
		data, err := ioutil.ReadFile(cfgName)
		if err != nil {
			return err
		}
		m := make(map[string]string)
		for _, line := range strings.Split(string(data), "\n") {
			i := strings.Index(line, ":")
			if i < 0 {
				continue
			}
			m[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		}
		db.cfgTime = info.ModTime()
		db.cfgSize = info.Size()
		db.cfgMap.Store(m)
	}

	info, err = db.toc.Stat()
	if err != nil {
		return err
	}
	numRecord := info.Size() / tocSize
	if numRecord > db.numRecord {
		if err := extendMap(db.byHash, db.hash, tlog.StoredHashCount(db.numRecord), tlog.StoredHashCount(numRecord), 0, hashSize, hashToRecord); err != nil {
			return err
		}
		if err := extendMap(db.byKey, db.toc, db.numRecord, numRecord, tocHashOffset, tocSize, nil); err != nil {
			return err
		}
		atomic.StoreInt64(&db.numRecord, numRecord)
	}
	return nil
}

// Config returns the database configuration value for the given key.
func (db *DB) Config(key string) string {
	db.reload()
	m, _ := db.cfgMap.Load().(map[string]string)
	return m[key]
}

// SetConfig sets the database configuration value for the given key to the value.
func (db *DB) SetConfig(key, value string) error {
	db.reload()
	db.write.Lock()
	defer db.write.Unlock()

	old, _ := db.cfgMap.Load().(map[string]string)
	m := make(map[string]string)
	for k, v := range old {
		if v != "" {
			m[k] = v
		}
	}
	if value == "" {
		delete(m, key)
	} else {
		m[key] = value
	}
	db.cfgMap.Store(m)

	var buf bytes.Buffer
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&buf, "%s: %s\n", k, m[k])
	}
	cfgName := filepath.Join(db.name, "notary.cfg")
	err := ioutil.WriteFile(cfgName, buf.Bytes(), 0666)
	if err != nil {
		return err
	}
	return nil
}

// Close closes the database.
func (db *DB) Close() {
	if db.hash != nil {
		db.hash.Close()
	}
	if db.rec != nil {
		db.rec.Close()
	}
	if db.toc != nil {
		db.toc.Close()
	}
}

// hashToRecord reconstructs the record index for a given hash index.
func hashToRecord(index int64) int64 {
	// There is an amortized bound of no more than 2 hashes per index,
	// so index/2 is certainly <= the id we are looking for.
	// We can walk forward from there, doing at most log(NumRecord) probes
	// to get where we need to go.
	// Maybe there's a faster way to do this but it probably doesn't matter.
	id := index / 2
	for tlog.StoredHashIndex(0, id) < index {
		id++
	}
	if tlog.StoredHashIndex(0, id) == index {
		return id
	}
	// index was an interior hash node; ignore it.
	return -1
}

// extendMap extends an existing lookup map,
// adding the entries for records of size entrySize at index start up to but not including end.
// The map key is at offset o in each record.
// If idfunc is non-nil, it is applied to each index to derive the actual record id.
// If idfunc returns a negative value, that entry is skipped.
func extendMap(m map[uint64]int64, f *os.File, start, end int64, o, entrySize int, idfunc func(int64) int64) error {
	const block = 1 << 20
	entries := block / entrySize
	if int64(entries) > end-start {
		entries = int(end - start)
	}
	buf := make([]byte, entries*entrySize)
	for index := int64(start); index < end; {
		count := entries
		if int64(count) > end-index {
			count = int(end - index)
		}
		_, err := f.ReadAt(buf[:count*entrySize], index*int64(entrySize))
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				err = fmt.Errorf("%s shorter than expected", f.Name())
			}
			return err
		}
		for i := 0; i < count; i++ {
			h := binary.BigEndian.Uint64(buf[i*entrySize+o:])
			id := index
			if idfunc != nil {
				id = idfunc(index)
			}
			if h != 0 && id >= 0 {
				m[h] = id
			}
			index++
		}
	}
	return nil
}

// NumRecords returns the number of added records.
func (db *DB) NumRecords() int64 {
	db.reload()
	return atomic.LoadInt64(&db.numRecord)
}

// ReadHash implements tlog.HashReader.
func (db *DB) ReadHash(level int, n int64) (tlog.Hash, error) {
	db.reload()

	var h tlog.Hash
	_, err := db.hash.ReadAt(h[:], int64(hashSize)*tlog.StoredHashIndex(level, n))
	if err != nil {
		if err == io.EOF {
			err = fmt.Errorf("no such hash %d,%d", level, n)
		}
		return tlog.Hash{}, fmt.Errorf("database read hash: %v", err)
	}
	return h, nil
}

// Content provides read access to a single record's content.
type Content interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

// ReadContent returns the content for the given record.
func (db *DB) ReadContent(id int64) (Content, error) {
	db.reload()

	offset, dataSize, _, _, err := db.readToc(id)
	if err != nil {
		return nil, err
	}
	return io.NewSectionReader(db.rec, offset+recHdrSize, dataSize), nil
}

// We reject keys that are very long to avoid needing to load them into memory.
const maxKeyLen = 4096

var errKeyTooLong = errors.New("key too long")

// doSync specifies whether to sync the data to disk on each write.
// This makes the server quite a bit slower than it would otherwise be
// (on my MacBook Pro, 54 writes/second instead of 18,000+ writes/second),
// but it also should help avoid data loss on a system crash.
const doSync = true

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
func (db *DB) Add(content []byte, key string) (int64, error) {
	db.reload()

	if len(key) > maxKeyLen {
		return 0, errKeyTooLong
	}

	h := tlog.RecordHash(content)
	kh := keyHash(key)

	db.write.Lock()
	defer db.write.Unlock()

	// Look for record by content hash.
	// We don't use FindHash because we want to diagnose
	// truncated hash collisions and mismatched keys.
	db.maps.RLock()
	id, ok := db.byHash[binary.BigEndian.Uint64(h[:])]
	db.maps.RUnlock()
	if ok {
		h1, err := db.ReadHash(0, id)
		if err != nil {
			return 0, err
		}
		if h1 != h {
			return 0, fmt.Errorf("truncated sha256 hash collision %v %v", h1, h)
		}
		k, err := db.key(id)
		if err != nil {
			return 0, err
		}
		if k != key {
			return 0, fmt.Errorf("content already stored with different key")
		}
		return id, nil
	}

	// Data is new; is key new?
	_, err := db.FindKey(key)
	if err == nil {
		return 0, fmt.Errorf("key in use by different content")
	}

	// Prepare to write record.
	id = db.numRecord
	hashes, err := tlog.StoredHashes(id, content, db)
	if err != nil {
		return 0, err
	}

	hbuf := make([]byte, 0, len(hashes)*hashSize)
	for i := range hashes {
		hbuf = append(hbuf, hashes[i][:]...)
	}

	// Write record after last good record.
	var offset int64
	if id > 0 {
		lastOffset, lastDataSize, lastKeySize, _, err := db.readToc(id - 1)
		if err != nil {
			return 0, err
		}
		offset = lastOffset + recHdrSize + lastDataSize + lastKeySize
	}
	hdr := make([]byte, recHdrSize)
	binary.BigEndian.PutUint64(hdr[0:], uint64(id))
	binary.BigEndian.PutUint64(hdr[8:], uint64(len(content)))
	binary.BigEndian.PutUint64(hdr[16:], uint64(len(key)))
	if _, err := db.rec.WriteAt(hdr, offset); err != nil {
		return 0, fmt.Errorf("database add: writing record: %v", err)
	}
	if _, err := db.rec.WriteAt(content, offset+recHdrSize); err != nil {
		return 0, fmt.Errorf("database add: writing record: %v", err)
	}
	if _, err := db.rec.WriteAt([]byte(key), offset+recHdrSize+int64(len(content))); err != nil {
		return 0, fmt.Errorf("database add: writing record: %v", err)
	}

	// Write hashes.
	if _, err := db.hash.WriteAt(hbuf, int64(hashSize)*tlog.StoredHashIndex(0, id)); err != nil {
		return 0, fmt.Errorf("database add: writing hashes: %v", err)
	}

	// Sync data before writing toc. Don't want toc ahead of data.
	if doSync {
		if err := db.rec.Sync(); err != nil {
			return 0, fmt.Errorf("database add: syncing record: %v", err)
		}
		if err := db.hash.Sync(); err != nil {
			return 0, fmt.Errorf("database add: syncing hashes: %v", err)
		}
	}

	// Write toc. This step commits the write:
	// if we crash before the index is written out and restart,
	// we'll just ignore the fact that the rec and hash
	// files are too large and overwrite the old useless
	// bits with new writes.
	// Also, any other DBs watching this directory
	// don't try to scan the new entries until they see the toc grow.
	toc := make([]byte, tocSize)
	binary.BigEndian.PutUint64(toc[0:], uint64(offset))
	binary.BigEndian.PutUint64(toc[8:], uint64(len(content)))
	binary.BigEndian.PutUint64(toc[16:], uint64(len(key)))
	binary.BigEndian.PutUint64(toc[24:], kh)
	if _, err := db.toc.WriteAt(toc, id*tocSize); err != nil {
		return 0, fmt.Errorf("database add: writing toc: %v", err)
	}

	// Sync toc before responding. Don't want to lose a reported write.
	if doSync {
		if err := db.toc.Sync(); err != nil {
			return 0, fmt.Errorf("database add: syncing toc: %v", err)
		}
	}

	db.maps.Lock()
	db.byHash[binary.BigEndian.Uint64(h[:])] = id
	db.byKey[kh] = id
	db.maps.Unlock()

	atomic.AddInt64(&db.numRecord, +1)

	return id, nil
}

// readToc reads and returns the table of contents entry for the given record.
func (db *DB) readToc(id int64) (offset, contentSize, keySize, keyHash int64, err error) {
	var toc [32]byte
	if _, err := db.toc.ReadAt(toc[:], id*tocSize); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("database read toc: %v", err)
	}
	return int64(binary.BigEndian.Uint64(toc[0:])),
		int64(binary.BigEndian.Uint64(toc[8:])),
		int64(binary.BigEndian.Uint64(toc[16:])),
		int64(binary.BigEndian.Uint64(toc[24:])),
		nil
}

var errNoRecord = errors.New("no such record")

// FindHash looks up a record by its content hash,
// as computed by tlog.RecordHash.
func (db *DB) FindHash(hash tlog.Hash) (int64, error) {
	db.reload()

	db.maps.RLock()
	id, ok := db.byHash[binary.BigEndian.Uint64(hash[:])]
	db.maps.RUnlock()
	if !ok {
		return 0, errNoRecord
	}

	// Found possible match.
	// Load the full SHA-256 hash to double-check.
	// (Anyone can just make up a colliding hash.)
	h, err := db.ReadHash(0, id)
	if err != nil {
		return 0, err
	}
	if h != hash {
		return 0, errNoRecord
	}

	return id, nil
}

// FindKey looks up a record by its associated key.
func (db *DB) FindKey(key string) (int64, error) {
	db.reload()

	if len(key) > maxKeyLen {
		return 0, errKeyTooLong
	}
	if key == "" {
		return 0, errNoRecord
	}
	h := keyHash(key)
	db.maps.RLock()
	id, ok := db.byKey[h]
	db.maps.RUnlock()
	if !ok {
		return 0, errNoRecord
	}

	// Found possible match.
	// Load actual key to check.
	key2, err := db.key(id)
	if err != nil || key2 != key {
		println("key hash collision", key, key2, err)
		return 0, errNoRecord
	}

	return id, nil
}

// key returns the key for a record.
func (db *DB) key(id int64) (string, error) {
	offset, contentSize, keySize, _, err := db.readToc(id)
	if err != nil {
		return "", err
	}
	if int(keySize) < 0 || int(keySize) > maxKeyLen {
		return "", errKeyTooLong
	}
	buf := make([]byte, keySize)
	if _, err := db.rec.ReadAt(buf, offset+recHdrSize+contentSize); err != nil {
		return "", err
	}
	return string(buf), nil
}
