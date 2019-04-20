// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gosumcheck checks a go.sum file against a go.sum database server.
//
// Usage:
//
//	gosumcheck [-h H] [-k key] [-u url] [-v] go.sum
//
// The -h flag changes the tile height (default 8).
//
// The -k flag changes the go.sum database server key.
//
// The -u flag overrides the URL of the server (usually set from the key name).
//
// The -v flag enables verbose output.
// In particular, it causes gosumcheck to report
// the URL and elapsed time for each server request.
//
// WARNING! WARNING! WARNING!
//
// Gosumcheck is meant as a proof of concept demo and should not be
// used in production scripts or continuous integration testing.
// It does not cache any downloaded information from run to run,
// making it expensive and also keeping it from detecting server
// misbehavior or successful HTTPS man-in-the-middle timeline forks.
//
// To discourage misuse in automated settings, gosumcheck does not
// set any exit status to report whether any problems were found.
//
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/notary/internal/sumweb"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: gosumcheck [-h H] [-k key] [-u url] [-v] go.sum...\n")
	os.Exit(2)
}

var (
	height = flag.Int("h", 8, "tile height")
	vkey   = flag.String("k", "rsc-goog.appspot.com+eecb1dec+AbTy1QXWdqYd1TTpuaUqsk6u7p+n4AqLiLB8SBwoB831", "key") // TODO: Replace with real key.
	url    = flag.String("u", "", "url to server (overriding name)")
	vflag  = flag.Bool("v", false, "enable verbose output")
)

func main() {
	log.SetPrefix("notecheck: ")
	log.SetFlags(0)

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 {
		usage()
	}

	conn := sumweb.NewConn(new(client))

	// Look in environment explicitly, so that if 'go env' is old and
	// doesn't know about GONOVERIFY, we at least get anything
	// set in the environment.
	env := os.Getenv("GONOVERIFY")
	if env == "" {
		out, err := exec.Command("go", "env", "GONOVERIFY").CombinedOutput()
		if err != nil {
			log.Fatalf("go env GONOVERIFY: %v\n%s", err, out)
		}
		env = strings.TrimSpace(string(out))
	}
	conn.SetGONOVERIFY(env)

	for _, arg := range flag.Args() {
		data, err := ioutil.ReadFile(arg)
		if err != nil {
			log.Fatal(err)
		}
		checkGoSum(conn, arg, data)
	}
}

func checkGoSum(conn *sumweb.Conn, name string, data []byte) {
	lines := strings.Split(string(data), "\n")
	if lines[len(lines)-1] != "" {
		log.Printf("error: final line missing newline")
		return
	}
	lines = lines[:len(lines)-1]

	errs := make([]string, len(lines))
	var wg sync.WaitGroup
	for i, line := range lines {
		wg.Add(1)
		go func(i int, line string) {
			defer wg.Done()
			f := strings.Fields(line)
			if len(f) != 3 {
				errs[i] = "invalid number of fields"
				return
			}

			dbLines, err := conn.Lookup(f[0], f[1])
			if err != nil {
				if err == sumweb.ErrGONOVERIFY {
					errs[i] = fmt.Sprintf("%s@%s: %v", f[0], f[1], err)
				} else {
					// Otherwise Lookup properly adds the prefix itself.
					errs[i] = err.Error()
				}
				return
			}
			hashAlgPrefix := f[0] + " " + f[1] + " " + f[2][:strings.Index(f[2], ":")+1]
			for _, dbLine := range dbLines {
				if dbLine == line {
					return
				}
				if strings.HasPrefix(dbLine, hashAlgPrefix) {
					errs[i] = fmt.Sprintf("%s@%s hash mismatch: have %s, want %s", f[0], f[1], line, dbLine)
					return
				}
			}
			errs[i] = fmt.Sprintf("%s@%s hash algorithm mismatch: have %s, want one of:\n\t%s", f[0], f[1], line, strings.Join(dbLines, "\n\t"))
		}(i, line)
	}
	wg.Wait()

	for i, err := range errs {
		if err != "" {
			fmt.Printf("%s:%d: %s\n", name, i+1, err)
		}
	}
}

type client struct{}

func (*client) ReadConfig(file string) ([]byte, error) {
	if file == "key" {
		return []byte(*vkey + "\n" + *url), nil
	}
	if strings.HasSuffix(file, "/latest") {
		// Looking for cached latest tree head.
		// Empty result means empty tree.
		return []byte{}, nil
	}
	return nil, fmt.Errorf("unknown config %s", file)
}

func (*client) WriteConfig(file string, old, new []byte) error {
	// Ignore writes.
	return nil
}

func (*client) ReadCache(file string) ([]byte, error) {
	return nil, fmt.Errorf("no cache")
}

func (*client) WriteCache(file string, data []byte) {
	// Ignore writes.
}

func (*client) Log(msg string) {
	log.Print(msg)
}

func (*client) SecurityError(msg string) {
	log.Fatal(msg)
}

func init() {
	http.DefaultClient.Timeout = 1 * time.Minute
}

func (*client) GetURL(url string) ([]byte, error) {
	start := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GET %v: %v", url, resp.Status)
	}
	data, err := ioutil.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if *vflag {
		fmt.Fprintf(os.Stderr, "%.3fs %s\n", time.Since(start).Seconds(), url)
	}
	return data, nil
}
