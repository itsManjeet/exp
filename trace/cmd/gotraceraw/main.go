// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/exp/trace/internal/raw"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [mode]\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Supported modes:")
		fmt.Fprintf(flag.CommandLine.Output(), "\n")
		fmt.Fprintf(flag.CommandLine.Output(), "* text2bytes - converts a text format trace to bytes\n")
		fmt.Fprintf(flag.CommandLine.Output(), "* bytes2text - converts a byte format trace to text\n")
		fmt.Fprintf(flag.CommandLine.Output(), "* strip      - remove comments and whitespace from text\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\n")
		flag.PrintDefaults()
	}
	log.SetFlags(0)
}

func main() {
	flag.Parse()
	if narg := flag.NArg(); narg != 1 {
		log.Fatal("expected exactly one positional argument: the mode to operate in; see -h output")
	}

	r := bufio.NewReader(os.Stdin)
	w := bufio.NewWriter(os.Stdout)

	var tr traceReader
	var tw traceWriter
	var err error

	switch flag.Arg(0) {
	case "text2bytes":
		tr, err = raw.NewTextReader(r)
		if err != nil {
			log.Fatal(err)
		}
		tw, err = raw.NewWriter(w)
		if err != nil {
			log.Fatal(err)
		}
	case "bytes2text":
		tr, err = raw.NewReader(r)
		if err != nil {
			log.Fatal(err)
		}
		tw, err = raw.NewTextWriter(w)
		if err != nil {
			log.Fatal(err)
		}
	case "strip":
		tr, err = raw.NewTextReader(r)
		if err != nil {
			log.Fatal(err)
		}
		tw, err = raw.NewTextWriter(w)
		if err != nil {
			log.Fatal(err)
		}
	}
	for {
		ev, err := tr.NextEvent()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if err := tw.WriteEvent(ev); err != nil {
			log.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}

type traceReader interface {
	NextEvent() (raw.Event, error)
}

type traceWriter interface {
	WriteEvent(raw.Event) error
}
