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

var graph map[string]map[string]struct{} = make(map[string]map[string]struct{})
var shoppingCart map[string]map[string]struct{} = make(map[string]map[string]struct{})

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
		from := parts[0]
		to := parts[1]
		if _, ok := graph[from]; !ok {
			graph[from] = make(map[string]struct{})
		}
		graph[from][to] = struct{}{}
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
		if err := json.NewEncoder(w).Encode(graph); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/shoppingCart", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(shoppingCart); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/edge", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		from := in["from"]
		to := in["to"]

		if r.Method == http.MethodDelete {
			if _, ok := shoppingCart[from]; !ok {
				shoppingCart[from] = make(map[string]struct{})
			}
			shoppingCart[from][to] = struct{}{}
			delete(graph[from], to)
		} else if r.Method == http.MethodPost {
			if _, ok := graph[from]; !ok {
				graph[from] = make(map[string]struct{})
			}
			graph[from][to] = struct{}{}
			delete(shoppingCart[from], to)
		}

		m := make(map[string]interface{})
		m["graph"] = graph
		m["shoppingCart"] = shoppingCart
		if err := json.NewEncoder(w).Encode(m); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	log.Println("Listening on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
