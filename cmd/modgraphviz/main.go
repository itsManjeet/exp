// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The modgraphviz command translates the output for `go mod graph` into .dot
// notation, which can then be parsed by `dot` into visual graphs.
package main

import (
	"bytes"
	"flag"
	"log"
	"os"

	"golang.org/x/exp/cmd/modgraphviz/internal"
)

func main() {
	flag.Usage = func() {
		log.Println("Usage: GO111MODULE=on go mod graph | modgraphviz | dot -Tpng -o outfile.png")
	}
	flag.Parse()

	var out bytes.Buffer

	if err := internal.Run(os.Stdin, &out); err != nil {
		log.Fatal(err)
	}

	if _, err := out.WriteTo(os.Stdout); err != nil {
		log.Fatal(err)
	}
}
