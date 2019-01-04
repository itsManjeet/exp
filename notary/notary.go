// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Notary is a simple Go module notary implementation.
//
// This command is part of a DRAFT of what the Go module notary will look like.
// Do not assume the details here are final!
//
// TODO: Write documentation.
//
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/exp/notary/note"
	"golang.org/x/exp/notary/notedb"
	"golang.org/x/exp/notary/tlog"
)

var _ tlog.Hash

func usage() {
	os.Stderr.WriteString(usageMessage)
	os.Exit(2)
}

var usageMessage = `Usage: notary cmd [flags] [args ...]

Commands:
	add           add new raw record (for debugging)
	check         check proof
	config        edit configuration
	hash          print hash of file
	init          initialize database
	log           print record log
	prove         prove tree contains leaf or earlier tree
	serve         serve HTTP (unimplemented)
	show          print record or tree
	sign          download and sign module version
`

/*
URLS

/notary/proveroot?root=tid&id=id
tid thash oid ohash
proof hash list

/notary/prove?tree=tid&id=id/hash/key
tid thash id hash
proof hash list

/notary/root?id=id&hash=1&sign=1
/notary/show?id=id/hash/key&hash=1&sign=1
/notary/sign?module=module@version (POST only)
/notary/log?start=1&end=10
id size\n
<size bytes>\n
id size\n
<size bytes>\n
...

*/

func main() {
	log.SetFlags(0)
	log.SetPrefix("notary: ")
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		usage()
	}

	cmd := cmds[args[0]]
	if cmd == nil {
		usage()
	}
	log.SetPrefix("notary " + args[0] + ": ")
	cmd(args[1:])
}

var cmds = map[string]func([]string){
	"add":    cmdAdd,
	"check":  cmdCheck,
	"config": cmdConfig,
	"find":   cmdFind,
	"hash":   cmdHash,
	"init":   cmdInit,
	"log":    cmdLog,
	"prove":  cmdProve,
	// TODO serve
	"show": cmdShow,
	"sign": cmdSign,
	"tree": cmdTree,
}

var dbname = "notary.db"

func needDB() {
	flag.StringVar(&dbname, "db", dbname, "database `file`")
}

func mustOpenDB() *notedb.DB {
	db, err := notedb.Open(dbname)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func mustReadFile(file string) []byte {
	var data []byte
	var err error
	if file == "-" {
		data, err = ioutil.ReadAll(os.Stdin)
	} else {
		data, err = ioutil.ReadFile(file)
	}
	if err != nil {
		log.Fatal(err)
	}
	return data
}

func mustParseInt(s string) int64 {
	i, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		log.Fatalf("invalid int %q", s) // nicer looking than strconv's error
	}
	return i
}

func mustParseHash(s string) tlog.Hash {
	h, err := tlog.ParseHash(s)
	if err != nil {
		log.Fatal(err)
	}
	return h
}

func cmdInit(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary init [-db file] name\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	needDB()
	flag.CommandLine.Parse(args)
	if flag.NArg() != 1 {
		flag.Usage()
	}
	_, err := notedb.Create(dbname)
	if err != nil {
		log.Fatal(err)
	}
	db := mustOpenDB()
	defer db.Close()
	name := flag.Arg(0)
	skey, vkey, err := note.GenerateKey(rand.Reader, name)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.SetConfig("secret-key", skey); err != nil {
		log.Fatal(err)
	}
	if err := db.SetConfig("public-key", vkey); err != nil {
		log.Fatal(err)
	}
}

func cmdAdd(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary add [-db file] [-k key] <file>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	needDB()
	var key string
	flag.StringVar(&key, "k", "", "make record available for lookup by `key`")
	flag.CommandLine.Parse(args)
	if flag.NArg() != 1 {
		flag.Usage()
	}
	db := mustOpenDB()
	defer db.Close()
	data := mustReadFile(flag.Arg(0))
	index, err := db.Add(data, key)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d\n", index)
}

func cmdConfig(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary config key [value]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	needDB()
	flag.CommandLine.Parse(args)
	if flag.NArg() != 1 && flag.NArg() != 2 {
		flag.Usage()
	}
	db := mustOpenDB()
	defer db.Close()

	key := flag.Arg(0)
	if flag.NArg() == 1 {
		fmt.Printf("%s\n", db.Config(key))
	} else {
		err := db.SetConfig(key, flag.Arg(1))
		if err != nil {
			log.Fatal(err)
		}
	}
}

func cmdLog(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary log [-db file] [<start-id> [<end-id>]]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	needDB()
	flag.CommandLine.Parse(args)
	if flag.NArg() > 2 {
		flag.Usage()
	}
	db := mustOpenDB()
	defer db.Close()

	start := int64(0)
	end := int64(1e18)
	if flag.NArg() >= 1 {
		start = mustParseInt(flag.Arg(0))
	}
	if flag.NArg() == 2 {
		end = mustParseInt(flag.Arg(1))
	}
	if max := db.NumRecords() - 1; end > max {
		end = max
	}
	for id := start; id <= end; id++ {
		r, err := db.ReadContent(id)
		if err != nil {
			log.Fatal(err)
		}
		data, err := ioutil.ReadAll(r)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d %d\n", id, len(data))
		os.Stdout.Write(data)
	}
}

func cmdShow(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary show [-db file] [-h] [-t] <id-or-hash-or-key>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	needDB()
	var hflag, tflag, sflag bool
	flag.BoolVar(&hflag, "h", false, "show hash instead of data")
	flag.BoolVar(&sflag, "s", false, "sign output")
	flag.BoolVar(&tflag, "t", false, "look up tree instead of record")
	flag.CommandLine.Parse(args)
	if flag.NArg() != 1 {
		flag.Usage()
	}
	db := mustOpenDB()
	defer db.Close()

	arg := flag.Arg(0)
	id, err := strconv.ParseInt(arg, 0, 64)
	if err != nil && !tflag {
		if h, err1 := tlog.ParseHash(arg); err1 == nil {
			id, err = db.FindHash(h)
		} else {
			id, err = db.FindKey(arg)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
	var data []byte
	if tflag {
		h, err := tlog.TreeHash(id, db)
		if err != nil {
			log.Fatal(err)
		}
		if hflag {
			fmt.Printf("%v\n", h)
			return
		}
		data = note.FormatTree(id, h)
	} else {
		if hflag {
			h, err := db.ReadHash(0, id)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%v\n", h)
			return
		}
		r, err := db.ReadContent(id)
		if err != nil {
			log.Fatal(err)
		}
		data, err = ioutil.ReadAll(r)
		if err != nil {
			log.Fatal(err)
		}
	}

	if sflag {
		skey := db.Config("secret-key")
		if skey == "" {
			log.Fatal("missing secret-key configuration")
		}
		signer, err := note.NewSigner(db.Config("secret-key"))
		if err != nil {
			log.Fatal(err)
		}
		data, err = note.Sign(&note.Note{Text: string(data)}, signer)
		if err != nil {
			log.Fatal(err)
		}
	}
	os.Stdout.Write(data)
}

func cmdFind(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary find [-db file] [-k] <hash>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	needDB()
	var kflag bool
	flag.BoolVar(&kflag, "k", false, "look up by key instead of hash")
	flag.CommandLine.Parse(args)
	if flag.NArg() != 1 {
		flag.Usage()
	}
	db := mustOpenDB()
	defer db.Close()

	arg := flag.Arg(0)
	var id int64
	var err error
	if kflag {
		id, err = db.FindKey(arg)
	} else {
		id, err = db.FindHash(mustParseHash(arg))
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d\n", id)
}

func cmdProve(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary prove [-db file] [-t] <tid> <id>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	needDB()
	var tflag bool
	flag.BoolVar(&tflag, "t", false, "prove tree extension instead of record inclusion")
	flag.CommandLine.Parse(args)
	if flag.NArg() != 2 {
		flag.Usage()
	}
	db := mustOpenDB()
	defer db.Close()
	tid := mustParseInt(flag.Arg(0))
	id := mustParseInt(flag.Arg(1))
	var p []tlog.Hash
	var err error
	if tflag {
		p, err = tlog.ProveTree(tid, id, db)
	} else {
		p, err = tlog.ProveRecord(tid, id, db)
	}
	if err != nil {
		log.Fatal(err)
	}
	for _, h := range p {
		fmt.Printf("%s\n", h)
	}
}

func cmdCheck(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary check [-t] <tid> <thash> <id> <hash> <proof-file>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	var tflag bool
	flag.BoolVar(&tflag, "t", false, "check tree extension instead of record inclusion")
	flag.CommandLine.Parse(args)
	if flag.NArg() != 5 {
		flag.Usage()
	}

	tid := mustParseInt(flag.Arg(0))
	thash := mustParseHash(flag.Arg(1))
	id := mustParseInt(flag.Arg(2))
	hash := mustParseHash(flag.Arg(3))
	data := mustReadFile(flag.Arg(4))

	var p []tlog.Hash
	for _, s := range strings.Fields(string(data)) {
		h, err := tlog.ParseHash(s)
		if err != nil {
			log.Fatalf("parsing proof: %v", h)
		}
		p = append(p, h)
	}

	var err error
	if tflag {
		err = tlog.CheckTree(p, tid, thash, id, hash)
	} else {
		err = tlog.CheckRecord(p, tid, thash, id, hash)
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("proof ok\n")
}

func cmdHash(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary hash <file>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	var tflag bool
	flag.BoolVar(&tflag, "t", false, "check tree extension instead of record inclusion")
	flag.CommandLine.Parse(args)
	if flag.NArg() != 1 {
		flag.Usage()
	}

	fmt.Printf("%s\n", tlog.RecordHash(mustReadFile(flag.Arg(0))))
}

func cmdTree(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary tree [-db file] [-h] [<id>]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	needDB()
	var hflag bool
	flag.BoolVar(&hflag, "h", false, "show hash of tree instead of id")
	flag.CommandLine.Parse(args)
	if flag.NArg() > 1 {
		flag.Usage()
	}
	db := mustOpenDB()
	defer db.Close()

	var id int64
	var h tlog.Hash
	if flag.NArg() == 0 {
		id = db.NumRecords()
		if hflag {
			var err error
			h, err = tlog.TreeHash(id, db)
			if err != nil {
				log.Fatal(err)
			}
		}
	} else {
		var err error
		id = mustParseInt(flag.Arg(0))
		h, err = tlog.TreeHash(id, db)
		if err != nil {
			log.Fatal(err)
		}
	}
	if hflag {
		fmt.Printf("%v\n", h)
		return
	}
	fmt.Printf("%d\n", id)
}

func cmdSign(args []string) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: notary sign [-db file] <module>@<version>...\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	needDB()
	flag.CommandLine.Parse(args)
	if flag.NArg() > 1 {
		flag.Usage()
	}
	db := mustOpenDB()
	defer db.Close()

	for _, arg := range flag.Args() {
		cmd := exec.Command("go", "mod", "download", "-json", arg)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), "GO111MODULE=on")
		cmd.Dir = "/"
		err := cmd.Run()
		if err != nil {
			log.Printf("go mod download %s: %v\n%s%s", arg, err, stdout.Bytes(), stderr.Bytes())
			continue
		}
		var info struct {
			Path     string
			Version  string
			Error    string
			Sum      string
			GoModSum string
		}
		if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
			log.Printf("go mod download %s: invalid json: %v [%s]\n", arg, err, stdout.String())
			continue
		}
		if info.Error != "" {
			log.Printf("go mod download %s: %s\n", arg, info.Error)
			continue
		}
		text := fmt.Sprintf("go notary sum\n%s %s\n%s\n/go.mod %s\n", info.Path, info.Version, info.Sum, info.GoModSum)
		key := info.Path + "@" + info.Version
		id, err := db.Add([]byte(text), key)
		if err != nil {
			log.Printf("%s: %v", arg, err)
			continue
		}
		fmt.Printf("%s %d\n%s\n", info.Path+"@"+info.Version, id, text)
	}
}
