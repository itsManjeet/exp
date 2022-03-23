// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.18

package main

import (
	"context"
	"io"

	"golang.org/x/exp/vulncheck"
)

func vulncheckBinary(ctx context.Context, r io.ReaderAt, cfg *vulncheck.Config) (*vulncheck.Result, error) {
	return vulncheck.Binary(ctx, r, cfg)
}
