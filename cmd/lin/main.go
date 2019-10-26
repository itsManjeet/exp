// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Linguini (lin) displays “go mod graph” output as an interactable graph. Lin
// allows you to see how your dependencies would change if you deleted one or
// more edges.

// While existing tools (e.g. golang.org/x/cmd/digraph) are good at answering
// questions about the actual graph, they're not so helpful with the kind of
// hypothetical questions you have when you're about to undertake a refactoring.
// Linguini can help you see the scope of the work ahead of you.
//
// Usage:
//
//	go mod graph | lin
//	go mod graph | digraph transpose | lin
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type edge struct {
	From string
	To   string
}

var graph []edge

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: go mod graph | lin")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("lin: ")

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 0 {
		usage()
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		l := scanner.Text()
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) != 2 {
			log.Fatalf("expected 2 words in line, but got %d: %s", len(parts), l)
		}
		graph = append(graph, edge{From: parts[0], To: parts[1]})
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles(filepath.Join("static", "index.html"))
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "page", nil); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/graph", func(w http.ResponseWriter, r *http.Request) {
		graphString := "digraph {\n"
		for _, e := range graph {
			graphString += fmt.Sprintf("\t%q -> %q\n", e.From, e.To)
		}
		graphString += "}\n"
		if _, err := w.Write([]byte(graphString)); err != nil {
			log.Fatal(err)
		}
	})

	http.HandleFunc("/edges", func(w http.ResponseWriter, r *http.Request) {
		b, err := json.Marshal(graph)
		if err != nil {
			fmt.Println("error:", err)
		}
		if _, err := w.Write(b); err != nil {
			log.Fatal(err)
		}
	})

	log.Println("Listening on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
