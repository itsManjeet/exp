// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package binscan contains methods for parsing Go binary files for the purpose
// of extracting module dependency and symbol table information.
package binscan

// Code in this package is dervied from src/cmd/go/internal/version/version.go
// and cmd/go/internal/version/exe.go.

import (
	"errors"
	"io"
	"runtime/debug"

	"golang.org/x/tools/go/packages"
)

//lint:ignore U1000 this is a utility used when compiled with go1.18+.
func debugModulesToPackagesModules(debugModules []*debug.Module) []*packages.Module {
	packagesModules := make([]*packages.Module, len(debugModules))
	for i, mod := range debugModules {
		packagesModules[i] = &packages.Module{
			Path:    mod.Path,
			Version: mod.Version,
		}
		if mod.Replace != nil {
			packagesModules[i].Replace = &packages.Module{
				Path:    mod.Replace.Path,
				Version: mod.Replace.Version,
			}
		}
	}
	return packagesModules
}

// ExtractPackagesAndSymbols extracts the symbols, packages, and their associated module versions
// from a Go binary. Stripped binaries are not supported.
//
// TODO(#51412): detect inlined symbols too
func ExtractPackagesAndSymbols(bin io.ReaderAt) ([]*packages.Module, map[string][]string, error) {
	return extractPackagesAndSymbols(bin)
}

var extractPackagesAndSymbols = func(bin io.ReaderAt) ([]*packages.Module, map[string][]string, error) {
	return nil, nil, errors.New("ExtractPackagesAndSymbols requires go1.18 or newer")
}
