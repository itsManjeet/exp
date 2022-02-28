// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.18
// +build go1.18

package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"log"
)

const src = `
package p

type Pair[L, R comparable] struct {
	left  L
	right R
}

func (p Pair[L, _]) Left() L {
	return p.left
}

var X Pair[int, string]
var Y Pair[string, int]
`

func Instantiate(pkg *types.Package) error {
	Pair := pkg.Scope().Lookup("Pair").Type()
	X := pkg.Scope().Lookup("X").Type()
	Y := pkg.Scope().Lookup("Y").Type()

	Compare(X, Y)

	ctxt := types.NewContext()
	Int := types.Typ[types.Int]
	String := types.Typ[types.String]
	I1, _ := types.Instantiate(ctxt, Pair, []types.Type{Int, String}, true)
	Compare(X, I1)
	Compare(Y, I1)

	I2, _ := types.Instantiate(ctxt, Pair, []types.Type{Int, String}, true)
	Compare(I1, I2)

	Any := types.Universe.Lookup("any").Type()
	_, err := types.Instantiate(ctxt, Pair, []types.Type{Int, Any}, true)
	var argErr *types.ArgumentError
	if errors.As(err, &argErr) {
		fmt.Printf("Argument %d: %v\n", argErr.Index, argErr.Err)
	}

	return nil
}

func Compare(left, right types.Type) {
	fmt.Println()
	fmt.Printf("Identical(%s, %s) : %t\n", left, right, types.Identical(left, right))
	fmt.Printf("%s == %s : %t\n", left, right, left == right)
}

func TypeCheckWithInstances(fset *token.FileSet, file *ast.File) (*types.Package, *types.Info, error) {
	conf := types.Config{}
	info := &types.Info{
		Instances: make(map[*ast.Ident]types.Instance),
	}
	pkg, err := conf.Check("p", fset, []*ast.File{file}, info)
	return pkg, info, err
}

func PrintInstances(fset *token.FileSet, info *types.Info) {
	for id, inst := range info.Instances {
		posn := fset.Position(id.Pos())
		fmt.Printf("%s: %s instantiated with %s\n", posn, id.Name, FormatTypeList(inst.TypeArgs))
	}
}

func FormatTypeList(list *types.TypeList) string {
	var buf bytes.Buffer
	buf.WriteRune('[')
	for i := 0; i < list.Len(); i++ {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(list.At(i).String())
	}
	buf.WriteRune(']')
	return buf.String()
}

func main() {
	// Parse one file.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "hello.go", src, 0)
	if err != nil {
		log.Fatal(err) // parse error
	}
	pkg, info, err := TypeCheckWithInstances(fset, f)
	if err != nil {
		log.Fatal(err)
	}
	PrintInstances(fset, info)
	if err := Instantiate(pkg); err != nil {
		log.Fatal(err) // type error
	}
}
