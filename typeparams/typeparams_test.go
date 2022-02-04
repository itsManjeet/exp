// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.18
// +build go1.18

package typeparams_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func TestAPIConsistency(t *testing.T) {
	// TODO: write an API consistency test that doesn't use x/tools/internal
}

func typeCheck(t *testing.T, filenames []string) *types.Package {
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range filenames {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	conf := types.Config{
		Importer: importer.Default(),
	}
	pkg, err := conf.Check("", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}
