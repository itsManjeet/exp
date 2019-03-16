// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/exp/notary/internal/database"
	"golang.org/x/exp/notary/internal/googlecloud/spandb"
	"golang.org/x/exp/notary/internal/note"
	"golang.org/x/exp/notary/internal/notedb"
	"golang.org/x/exp/notary/internal/noteweb"
	"golang.org/x/exp/notary/internal/tlog"
)

var memFlag = flag.Bool("mem", false, "use in-memory database")

const spannerDB = "projects/rsc-goog/instances/rsc-test/databases/test_notary"

func main() {
	flag.Parse()
	handler := &noteweb.Handler{Server: noteServer{}}
	for _, path := range noteweb.Paths {
		http.Handle(path, handler)
	}
	http.HandleFunc("/shell", shell)
	http.HandleFunc("/ping1", ping)
	http.HandleFunc("/_ah/health", healthCheckHandler)
	http.HandleFunc("/cmd", cmd)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

type noteServer struct{}

func (noteServer) NewContext(r *http.Request) (context.Context, error) {
	return context.Background(), nil
}

func (noteServer) Signed(ctx context.Context) ([]byte, error) {
	db := getDB(ctx)
	size, err := db.NumRecords(ctx)
	if err != nil {
		return nil, err
	}
	hash, err := db.TreeHash(ctx, size)
	if err != nil {
		return nil, err
	}
	privateKey, err := db.Config(ctx, "private-key")
	if err != nil {
		return nil, err
	}
	signer, err := note.NewSigner(privateKey)
	if err != nil {
		return nil, err
	}
	msg, err := note.Sign(&note.Note{Text: string(tlog.FormatTree(tlog.Tree{size, hash}))}, signer)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (noteServer) ReadRecords(ctx context.Context, id, n int64) ([][]byte, error) {
	return getDB(ctx).ReadRecords(ctx, id, n)
}

func (noteServer) FindKey(ctx context.Context, key string) (int64, error) {
	// TODO: Store @ keys.
	key = strings.Replace(key, "@", " ", 1)
	return getDB(ctx).FindKey(ctx, key)
}

var tileCache struct {
	sync.Mutex
	m map[tlog.Tile][]byte
}

func (noteServer) ReadTileData(ctx context.Context, t tlog.Tile) ([]byte, error) {
	tileCache.Lock()
	data := tileCache.m[t]
	tileCache.Unlock()

	if data != nil {
		return data, nil
	}
	data, err := getDB(ctx).ReadTileData(ctx, t)
	if err != nil {
		return nil, err
	}

	tileCache.Lock()
	if tileCache.m == nil {
		tileCache.m = make(map[tlog.Tile][]byte)
	}
	tileCache.m[t] = data
	tileCache.Unlock()

	return data, nil
}

func dropcache(ctx context.Context, w io.Writer, args []string) {
	tileCache.Lock()
	tileCache.m = nil
	tileCache.Unlock()

	fmt.Fprintf(w, "dropped cache\n")
}

func doAuth(w http.ResponseWriter, r *http.Request) bool {
	return true
	user, pass, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", "Basic")
		w.WriteHeader(401)
		return false
	}
	sum := sha256.Sum256([]byte(user + " " + pass))
	hex := fmt.Sprintf("%x", sum[:])
	if hex != "d89db2536d1398a2c795994158a94c0dd4c171190fa86d7bc58d836808d9ce6b" {
		w.WriteHeader(403)
		fmt.Fprintf(w, "sorry\n")
		return false
	}
	return true
}

func shell(w http.ResponseWriter, r *http.Request) {
	if !doAuth(w, r) {
		return
	}
	w.Write([]byte(shellPage))
}

func ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong!\n"))
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "you never call, you never write, i might be dead for all you know.")
}

func cmd(w http.ResponseWriter, r *http.Request) {
	if !doAuth(w, r) {
		return
	}
	cmd := r.FormValue("cmd")
	f := strings.Fields(cmd)
	if len(f) == 0 {
		fmt.Fprintf(w, "no command\n")
		return
	}

	ctx := context.Background()
	if fn := cmds[f[0]]; fn != nil {
		start := time.Now()
		func() {
			defer func() {
				if e := recover(); e != nil {
					fmt.Fprintf(w, "\nPANIC: %v\n%s", e, debug.Stack())
				}
			}()
			fn(ctx, w, f)
		}()
		fmt.Fprintf(w, "\n[%.3fs elapsed]\n", time.Since(start).Seconds())
		return
	}

	fmt.Fprintf(w, "?unknown command\n")
}

var cmds = map[string]func(context.Context, io.Writer, []string){
	"cfg":       cfg,
	"create":    createDB,
	"date":      date,
	"delete":    deleteDB,
	"dropcache": dropcache,
	"echo":      echo,
	"exit":      exit,
	"insert":    insert,
	"insert1":   insert1,
	"newkey":    newkey,
	"delkey":    delkey,
	"sign":      sign,
	"size":      size,
	"stack":     stack,
}

func init() {
	cmds["help"] = help
}

func help(ctx context.Context, w io.Writer, args []string) {
	var names []string
	for name := range cmds {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "%s\n", name)
	}
}

func echo(ctx context.Context, w io.Writer, args []string) {
	fmt.Fprintf(w, "%s\n", strings.Join(args, " "))
}

func date(ctx context.Context, w io.Writer, args []string) {
	fmt.Fprintf(w, time.Now().Format(time.RFC3339Nano))
}

func createDB(ctx context.Context, w io.Writer, args []string) {
	sdb, err := spandb.CreateDB(ctx, spannerDB)
	if err != nil {
		fmt.Fprintf(w, "?%v\n", err)
		return
	}
	_, err = notedb.Create(ctx, sdb)
	if err != nil {
		fmt.Fprintf(w, "?%v\n", err)
		return
	}
	fmt.Fprintf(w, "created\n")
}

func deleteDB(ctx context.Context, w io.Writer, args []string) {
	err := spandb.DeleteTestDB(ctx, spannerDB)
	if err != nil {
		fmt.Fprintf(w, "?%v\n", err)
		return
	}
	fmt.Fprintf(w, "deleted\n")
}

var dbcache struct {
	sync.Mutex
	db *notedb.DB
}

func getDB(ctx context.Context) *notedb.DB {
	dbcache.Lock()
	defer dbcache.Unlock()
	if dbcache.db != nil {
		return dbcache.db
	}
	if *memFlag {
		db, err := notedb.Create(ctx, new(database.MemDB))
		if err != nil {
			panic(err)
		}
		dbcache.db = db
		return db
	}

	sdb, err := spandb.OpenDB(ctx, spannerDB)
	if err != nil {
		panic(err)
	}
	db, err := notedb.Open(ctx, sdb)
	if err != nil {
		panic(err)
	}
	dbcache.db = db
	return db
}

func size(ctx context.Context, w io.Writer, args []string) {
	db := getDB(ctx)
	n, err := db.NumRecords(ctx)
	if err != nil {
		fmt.Fprintf(w, "?size: %v\n", err)
		return
	}
	fmt.Fprintf(w, "%d\n", n)
}

func insert1(ctx context.Context, w io.Writer, args []string) {
	if len(args) != 1+6 || args[1] != args[4] || args[2]+"/go.mod" != args[5] {
		fmt.Fprintf(w, "?usage: insert1 mod v0.1.2 h1:asdf mod v0.1.2/go.mod h1:asdf\n")
		return
	}

	r := []notedb.NewRecord{{
		Key: args[1] + " " + args[2],
		Content: []byte(args[1] + " " + args[2] + " " + args[3] + "\n" +
			args[4] + " " + args[5] + " " + args[6] + "\n"),
	}}

	db := getDB(ctx)
	err := db.Add(ctx, r)
	if err != nil {
		fmt.Fprintf(w, "?add: %v\n", err)
		return
	}
	if r[0].Err != nil {
		fmt.Fprintf(w, "?add: %v [%d]\n", r[0].Err, r[0].ID)
		return
	}

	fmt.Fprintf(w, "%d\n", r[0].ID)
}

func cfg(ctx context.Context, w io.Writer, args []string) {
	if len(args) <= 1 {
		fmt.Fprintf(w, "?usage: cfg key [value]\n")
		return
	}
	db := getDB(ctx)
	if len(args) == 2 {
		key := args[1]
		val, err := db.Config(ctx, key)
		if err != nil {
			fmt.Fprintf(w, "?%v\n", err)
			return
		}
		fmt.Fprintf(w, "%s\n", val)
		return
	}
	err := db.SetConfig(ctx, args[1], strings.Join(args[2:], " "))
	if err != nil {
		fmt.Fprintf(w, "?%v\n", err)
		return
	}
	fmt.Fprintf(w, "updated %s\n", args[1])
}

func insert(ctx context.Context, w io.Writer, args []string) {
	start := time.Now()
	f := new(flag.FlagSet)
	pFlag := f.Int("p", 1, "parallelism")
	bFlag := f.Int("b", 2000, "max batch size")
	ok := true
	f.SetOutput(w)
	f.Usage = func() {
		fmt.Fprintf(w, "?usage: insert [-b B] [-p P] N\n")
		f.PrintDefaults()
		ok = false
		return
	}
	f.Parse(args[1:])
	if !ok {
		return
	}
	if f.NArg() != 1 {
		f.Usage()
		return
	}

	n, err := strconv.ParseInt(f.Arg(0), 0, 64)
	if err != nil {
		fmt.Fprintf(w, "?bad N: %v\n", err)
		return
	}
	r64 := make([]byte, 8*int(n))
	_, err = rand.Read(r64)
	if err != nil {
		fmt.Fprintf(w, "?rand: %v\n", err)
		return
	}
	db := getDB(ctx)
	treeSize, err := db.NumRecords(ctx)
	if err != nil {
		fmt.Fprintf(w, "?size: %v\n", err)
		return
	}
	var recs []notedb.NewRecord
	for i := int64(0); i < n; i++ {
		id := int64(binary.BigEndian.Uint64(r64) << 1 >> 1)
		r64 = r64[8:]
		r := notedb.NewRecord{
			Key:     fmt.Sprintf("golang.org/x/tools v0.0.%d", id),
			Content: []byte(fmt.Sprintf("golang.org/x/tools v0.0.%d h1:+FlnIV8DSQnT7NZ43hcVKcdJdzZoeCmJj4Ql8gq5keA=\ngolang.org/x/tools v0.0.%d/go.mod h1:+FlnIV8DSQnT7NZ43hcVKcdJdzZoeCmJj4Ql8gq5keA=\n", id, id)),
		}
		// fmt.Fprintf(w, "%s\n", r.Key)
		recs = append(recs, r)
	}

	for len(recs) > 0 {
		batch := recs
		if len(batch) > *bFlag {
			batch = batch[:*bFlag]
		}
		recs = recs[len(batch):]

		c := make(chan error)
		p := *pFlag
		for i := 0; i < p; i++ {
			go func(recs []notedb.NewRecord) {
				c <- db.Add(ctx, recs)
			}(batch[i*len(batch)/p : (i+1)*len(batch)/p])
		}
		for i := 0; i < p; i++ {
			if err := <-c; err != nil {
				fmt.Fprintf(w, "?adding: %v\n", err)
			}
		}
	}

	fmt.Fprintf(w, "inserted %d (%.1f QPS)\n", n, float64(n)/time.Since(start).Seconds())
	newTreeSize, err := db.NumRecords(ctx)
	if err != nil {
		fmt.Fprintf(w, "?size: %v\n", err)
		return
	}
	if newTreeSize-treeSize != n {
		fmt.Fprintf(w, "?%d records + %d records = %d records!\n", treeSize, n, newTreeSize)
	}
}

func exit(ctx context.Context, w io.Writer, args []string) {
	os.Exit(2)
}

func stack(ctx context.Context, w io.Writer, args []string) {
	buf := make([]byte, 16<<20)
	n := runtime.Stack(buf, true)
	w.Write(buf[:n])
}

func newkey(ctx context.Context, w io.Writer, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(w, "?usage: newkey name\n")
		return
	}
	db := getDB(ctx)
	val, _ := db.Config(ctx, "public-key")
	if val != "" {
		fmt.Fprintf(w, "?existing key: %s\n", val)
		return
	}
	skey, vkey, err := note.GenerateKey(rand.Reader, args[1])
	if err != nil {
		fmt.Fprintf(w, "?%v\n", err)
	}
	if err := db.SetConfig(ctx, "public-key", vkey); err != nil {
		fmt.Fprintf(w, "?%v\n", err)
		return
	}
	if err := db.SetConfig(ctx, "private-key", skey); err != nil {
		fmt.Fprintf(w, "?%v\n", err)
		return
	}
	fmt.Fprintf(w, "%s\n", vkey)
}

func delkey(ctx context.Context, w io.Writer, args []string) {
	db := getDB(ctx)
	if err := db.SetConfig(ctx, "public-key", ""); err != nil {
		fmt.Fprintf(w, "?%v\n", err)
		return
	}
	if err := db.SetConfig(ctx, "private-key", ""); err != nil {
		fmt.Fprintf(w, "?%v\n", err)
		return
	}
}

func sign(ctx context.Context, w io.Writer, args []string) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(w, "?keygen: %v\n", err)
	}
	if len(args) < 2 {
		fmt.Fprintf(w, "?usage: sign T\n")
		return
	}
	t, err := strconv.ParseInt(args[1], 0, 64)
	if err != nil {
		fmt.Fprintf(w, "?bad T: %v\n", err)
		return
	}

	start := time.Now()
	lastTreeSize := int64(0)
	n := 0
	nsign := 0
	for ; time.Since(start) < time.Duration(t)*time.Second; time.Sleep(100 * time.Millisecond) {
		n++
		db := getDB(ctx)
		treeSize, err := db.NumRecords(ctx)
		if err != nil {
			fmt.Fprintf(w, "?size: %v\n", err)
			return
		}
		if treeSize == lastTreeSize {
			continue
		}
		lastTreeSize = treeSize
		h, err := db.TreeHash(ctx, treeSize)
		if err != nil {
			fmt.Fprintf(w, "?treehash: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}
		fmt.Fprintf(w, "sign %d\n", treeSize)
		nsign++
		data := ed25519.Sign(key, h[:])
		if err := db.SetConfig(ctx, "signed", fmt.Sprintf("%x", data)); err != nil {
			fmt.Fprintf(w, "?store sign: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}
	}
	fmt.Fprintf(w, "loaded tree %d times in %.3fs seconds\n", n, time.Since(start).Seconds())
	fmt.Fprintf(w, "found and signed %d distinct tree hashes\n", nsign)

}

const shellPage = `<!DOCTYPE html>
<script src="https://ajax.googleapis.com/ajax/libs/jquery/1.8.2/jquery.min.js"></script>

<h1>Notary Shell</h1>

<div id="output">
</div>

<form action="javascript:function(){}()">
<input id="cmd" type="text" size="100" autocomplete=off />
<input type="submit" style="display:none"/>
</form>
<script>
var escapeMap = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
  '/': '&#x2F;',
  '\x60': '&#x60;',
  '=': '&#x3D;'
};

function escape (string) {
  return String(string).replace(/[&<>"'\x60=\/]/g, function (s) {
    return escapeMap[s];
  });
}

var total = 1;

$('form').submit(function(event) {
	event.preventDefault();
	var text = $('#cmd').val();
	var n = total++;
	$('#output').append("<pre><b>"+escape(text)+"</b>\n</pre><div id=out"+n+"></div>\n")
	$.post("/cmd", {cmd: text}, function(data) {
		$('#out'+n).append("<pre>"+escape(data)+"</pre>\n")
	}).fail(function() {
		$('#out'+n).append("<pre>"+escape(data)+"</pre>\n")
	})
	$('#cmd').val("");
});
</script>
`
