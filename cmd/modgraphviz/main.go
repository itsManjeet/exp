// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The modgraphviz command translates the output for go mod graph into .dot
// notation, which can then be parsed by `dot` into visual graphs.
//
// Usage: GO111MODULE=on go mod graph | modgraphviz | dot -Tpng -o outfile.png
package main

import (
	"bytes"
	"flag"
	"log"
	"os"
)

var pathTo = flag.String("pathTo", "", "Only show the graph of the path(s) to a module. ex: --pathTo foo.com/bar@1.2.3")

func main() {
	flag.Usage = func() {
		// TODO add usage instructions for pathTo
		log.Println("Usage: GO111MODULE=on go mod graph | modgraphviz [--pathTo] | dot -Tpng -o outfile.png")
	}
	flag.Parse()

	var out bytes.Buffer

	g, err := newGraph(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := out.Write([]byte("digraph gomodgraph {\n")); err != nil {
		log.Fatal(err)
	}

	if *pathTo != "" {
		if err := g.printPathTo(&out, nil, nil, nil, *pathTo); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := g.print(&out, nil, nil); err != nil {
			log.Fatal(err)
		}
	}

	if _, err := out.Write([]byte("}\n")); err != nil {
		log.Fatal(err)
	}

	if _, err := out.WriteTo(os.Stdout); err != nil {
		log.Fatal(err)
	}
}
