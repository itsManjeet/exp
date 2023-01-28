// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by 'go generate'. DO NOT EDIT.
//go:generate ./mkdescs.sh

package slogtest

var descriptions = []string{
	`l.Info("message", "k", "v")`,
	`l.Info("msg", "a", "b", "", "ignore", "c", "d")`,
	`l.Info("msg", "k", "v")`,
	`l.With("a", "b").Info("msg", "k", "v")`,
	`l.Info("msg", "a", "b", slog.Group("G", slog.String("c", "d")), "e", "f")`,
	`l.Info("msg", "a", "b", slog.Group("G"), "e", "f")`,
	`l.Info("msg", "a", "b", slog.Group("", slog.String("c", "d")), "e", "f")`,
	`l.WithGroup("G").Info("msg", "a", "b")`,
	`l.With("a", "b").WithGroup("G").With("c", "d").WithGroup("H").Info("msg", "e", "f")`,
}
