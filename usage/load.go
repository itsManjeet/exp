// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.16
// +build go1.16

package usage

import "io/fs"

// Load scans the file system for all the help files and converts them to pages.
func Load(help fs.FS) ([]Page, error) {
	var pages []Page
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
