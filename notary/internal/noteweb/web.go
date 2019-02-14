// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package noteweb serves the notary web endpoints from a notary database.
package noteweb

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/exp/notary/internal/tlog"
)

// Server is a connection to a notary server.
type Server interface {
	// NewContext returns the context to use for the request r.
	NewContext(r *http.Request) (context.Context, error)

	// Signed returns the signed hash of the latest tree.
	Signed(ctx context.Context) ([]byte, error)

	// ReadContent returns the content for the given record.
	ReadContent(ctx context.Context, id int64) ([]byte, error)

	// FindKey looks up a record by its associated key ("module@version"),
	// returning the record ID.
	FindKey(ctx context.Context, key string) (int64, error)

	// ReadTileData reads the content of tile t.
	ReadTileData(ctx context.Context, t tlog.Tile) ([]byte, error)
}

// Handler is the notary endpoint handler,
// which should be used for the paths listed in Paths.
// The client is responsible for initializing Server.
type Handler struct {
	Server Server
}

// Paths are the URL paths for which Handler should be invoked.
//
// Typically a client will do:
//
//	handler := &noteweb.Handler{Server: srv}
//	for _, path := range noteweb.Paths {
//		http.HandleFunc(path, handler)
//	}
//
var Paths = []string{
	"/lookup/",
	"/record/",
	"/signed",
	"/tile/",
}

var modVerRE = regexp.MustCompile(`^[^@]+@v[0-9]+\.[0-9]+\.[0-9]+(-[^@]*)?$`)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, err := h.Server.NewContext(r)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	switch {
	default:
		http.NotFound(w, r)

	case strings.HasPrefix(r.URL.Path, "/lookup/"):
		mod := strings.TrimPrefix(r.URL.Path, "/lookup/")
		if !modVerRE.MatchString(mod) {
			http.Error(w, "invalid module@version syntax", http.StatusBadRequest)
			return
		}
		id, err := h.Server.FindKey(ctx, mod)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data, err := h.Server.ReadContent(ctx, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		fmt.Fprintf(w, "%d\n", id)
		w.Write(data)

	case strings.HasPrefix(r.URL.Path, "/record/"):
		arg := strings.TrimPrefix(r.URL.Path, "/record/")
		id, err := strconv.ParseInt(arg, 10, 64)
		if err != nil || id < 0 || strconv.FormatInt(id, 10) != arg {
			http.Error(w, "invalid record number syntax", http.StatusBadRequest)
			return
		}
		data, err := h.Server.ReadContent(ctx, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.Write(data)

	case r.URL.Path == "/latest":
		data, err := h.Server.Signed(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.Write(data)

	case strings.HasPrefix(r.URL.Path, "/tile/"):
		t, err := tlog.ParseTilePath(r.URL.Path[1:])
		if err != nil {
			http.Error(w, "invalid tile syntax", http.StatusBadRequest)
			return
		}
		data, err := h.Server.ReadTileData(ctx, t)
		if err != nil {
			if os.IsNotExist(data) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(data)
	}
}
