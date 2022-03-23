// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !go1.18

package main

import (
	"context"
	"errors"
	"io"

	"golang.org/x/exp/vulncheck"
)

func vulncheckBinary(context.Context, io.ReaderAt, *vulncheck.Config) (*vulncheck.Result, error) {
	return nil, errors.New("Binary not available for Go 1.17; upgrade to 1.18")
}
