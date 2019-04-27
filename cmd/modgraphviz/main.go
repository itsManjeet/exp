// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The modgraphviz command translates the output for go mod graph into .dot
// notation, which can then be parsed by graphviz into visual graphs.
//
// Requires graphviz and the graphviz dot cli to be installed.
//
// Usage: GO111MODULE=on go mod graph | modgraphviz [--pathsTo] | dot -Tpng -o outfile.png
package main

import (
	"bytes"
	"flag"
	"log"
	"os"
)

var pathsTo = flag.String("pathsTo", "", "Only show the graph of the path(s) to a module. ex: --pathsTo foo.com/bar@1.2.3")
var simple = flag.Bool("simple", false, "Only show the modules without their versions.")

func main() {
	flag.Usage = func() {
		log.Println("Usage: GO111MODULE=on go mod graph | modgraphviz [--pathsTo] | dot -Tpng -o outfile.png")
	}
	flag.Parse()

	var out bytes.Buffer

	g, err := newGraph(os.Stdin, *simple)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := out.Write([]byte("digraph gomodgraph {\n")); err != nil {
		log.Fatal(err)
	}

	if *pathsTo != "" {
		if err := g.printPathsTo(&out, *pathsTo); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := g.print(&out); err != nil {
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
