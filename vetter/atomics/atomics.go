// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package atomics is a vet style checks for consistent usage of atomics.

// Highly experimental code. Use at your own risk.

package atomics

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const Doc = `check for inconsistent usage of sync/atomic package

The atomics checker looks for inconsistent references of the form:

	atomic.AddUint64(&x, 1)
	return x

There is both an atomic and a non-atomic usage of x.

Limitations: Reports variables and fields usage within one package.
`

var Analyzer = &analysis.Analyzer{
	Name:     "atomics",
	Doc:      Doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var debug bool = false

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo

	// looking for both use of x.f and a atomics.AddInt32(&x.f, ...) use.

	// Optimization: skip if atomics is not directly imported.
	if !imports(pass.Pkg, "sync/atomic") {
		return nil, nil
	}

	// Note(taking): lattice sketch:
	//
	//            unknown
	//               |
	//    {atomic,non-atomic} use
	//     /                \
	//  {atomic} use  {non-atomic} use
	//          \      /
	//           {} use
	//
	// We are looking for a pair of an atomic and a non-atomic usage.

	// Collect for atomics.AddInt32(&x.f, ...)
	// Collect all of the field and variables in first argument of calls
	var calls int
	atomics := make(map[*types.Var][]*ast.CallExpr)
	callFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}
	inspect.Preorder(callFilter, func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if len(call.Args) == 0 {
			return
		}
		if isAtomic(info, call) {
			// Look for e: x, &x, x.f or &x.f.
			e := deparen(call.Args[0]) // arg0 in atomic.AddInt32(arg0, arg1)
			if addr, ok := e.(*ast.UnaryExpr); ok && addr.Op == token.AND {
				e = deparen(addr.X)
			}
			switch e := e.(type) {
			case *ast.Ident:
				if v, ok := info.Uses[e].(*types.Var); ok {
					atomics[v] = append(atomics[v], call)
					calls++
				}
			case *ast.SelectorExpr:
				if sel, ok := info.Selections[e]; ok {
					if v, ok := sel.Obj().(*types.Var); ok {
						atomics[v] = append(atomics[v], call)
						calls++
					}
				}
			}

		}
	})

	rem := calls
	defer func() {
		if calls > 0 && debug {
			fmt.Println("PPP ", rem, " , ", calls, " calls")
		}
	}()

	if len(atomics) == 0 {
		return nil, nil // optimization: no atomics to match against
	}

	// Collect non-atomic uses of selector and idents for vars.
	nas := make(map[*types.Var][]ast.Node) // invariant: keys(nas) subset keys(atomics)
	naFilter := []ast.Node{
		(*ast.CallExpr)(nil),
		(*ast.SelectorExpr)(nil),
		(*ast.Ident)(nil),
	}
	inspect.Nodes(naFilter, func(n ast.Node, push bool) bool {
		if !push {
			return false
		}
		switch n := n.(type) {
		case *ast.CallExpr:
			if isAtomic(info, n) {
				return false
			}
		case *ast.SelectorExpr:
			// inv: not in the scope of an atomic call.
			// TODO(taking): scope of an atomic call is not quite correct.
			//   atomic.AddInt(&x, x) should be reported.
			if sel, ok := info.Selections[n]; ok {
				if obj, ok := sel.Obj().(*types.Var); ok {
					if _, ok := atomics[obj]; ok {
						nas[obj] = append(nas[obj], n)
					}
				}
			}
		case *ast.Ident:
			if obj, ok := info.Uses[n].(*types.Var); ok {
				if !obj.IsField() { // skip fields as these are can be part of Composite literals keys.
					if _, ok := atomics[obj]; ok {
						nas[obj] = append(nas[obj], n)
					}
				}
			}
		}
		return true
	})

	for v := range nas {
		auses, nauses := atomics[v], nas[v]
		if len(auses) == 0 || len(nauses) == 0 || v == nil {
			return nil, fmt.Errorf("internal error in atomics checker on %v", v.Name())
		}

		var prefix string
		if v.IsField() {
			prefix = "field"
		} else {
			prefix = "variable"
		}
		nauPos := pass.Fset.Position(nauses[0].Pos())

		rem -= len(auses)

		// for _, au := range auses {
		// 	fmt.Println("au @", pass.Fset.Position(au.Pos()))
		// }
		// for _, nau := range nauses {
		// 	fmt.Println("nau @", pass.Fset.Position(nau.Pos()))
		// }

		pass.ReportRangef(auses[0], "%s %s is inconsistently used in sync/atomic functions (%d locs) and non-atomic operations ( %d locs, e.g. @ %s )",
			prefix, v.Name(), len(auses), len(nauses), nauPos)
	}

	return nil, nil
}

func isAtomic(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgIdent, _ := sel.X.(*ast.Ident)
	pkgName, ok := info.Uses[pkgIdent].(*types.PkgName)
	if !ok || pkgName.Imported().Path() != "sync/atomic" {
		return false
	}
	return atomicFnNames[sel.Sel.Name]
}

func imports(p *types.Package, path string) bool {
	for _, q := range p.Imports() {
		if q.Path() == path {
			return true
		}
	}
	return false
}

func deparen(e ast.Expr) ast.Expr {
	p, ok := e.(*ast.ParenExpr)
	for ok {
		e = p.X
		p, ok = e.(*ast.ParenExpr)
	}
	return e
}

var atomicFnNames map[string]bool = map[string]bool{
	"AddInt32":              true,
	"AddInt64":              true,
	"AddUint32":             true,
	"AddUint64":             true,
	"AddUintptr":            true,
	"CompareAndSwapInt32":   true,
	"CompareAndSwapInt64":   true,
	"CompareAndSwapPointer": true,
	"CompareAndSwapUint32":  true,
	"CompareAndSwapUint64":  true,
	"CompareAndSwapUintptr": true,
	"LoadInt32":             true,
	"LoadInt64":             true,
	"LoadPointer":           true,
	"LoadUint32":            true,
	"LoadUint64":            true,
	"LoadUintptr":           true,
	"StoreInt32":            true,
	"StoreInt64":            true,
	"StorePointer":          true,
	"StoreUint32":           true,
	"StoreUint64":           true,
	"StoreUintptr":          true,
	"SwapInt32":             true,
	"SwapInt64":             true,
	"SwapPointer":           true,
	"SwapUint32":            true,
	"SwapUint64":            true,
	"SwapUintptr":           true,
}
