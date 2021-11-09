// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.16
// +build go1.16

package usage

import (
	"io/fs"
)

// Load scans the file system for all the help files and converts them to pages.
func Load(help fs.FS) (Pages, error) {
	var pages Pages
	err := fs.WalkDir(help, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(help, path)
		if err != nil {
			return err
		}
		pages = append(pages, Page{Name: d.Name(), Path: path, Content: string(data)})
		return nil
	})
	return pages, err
}

// Process is a helper that implements the common flow for simple applications.
// It compiles the help text from the file system, scans the options for fields,
// matches the grammar against the command line arguments, and then applies the
// results to the fields that it found.
func Process(helpFS fs.FS, options interface{}, args []string) error {
	help, err := Load(helpFS)
	if err != nil {
		return err
	}
	grammar, err := help.Compile()
	if err != nil {
		return err
	}
	fields := &Fields{}
	if err := fields.Scan(options); err != nil {
		return err
	}
	bindings, err := grammar.Bind(fields)
	if err != nil {
		return err
	}

	return bindings.Process(args)
}
