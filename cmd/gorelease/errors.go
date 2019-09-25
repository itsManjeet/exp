// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/xerrors"
)

type helpError struct {
	err error
}

func helpErrorf(format string, args ...interface{}) error {
	return &helpError{err: fmt.Errorf(format, args...)}
}

const usageText = `usage: gorelease [-base=version] [-version=version]`

func (e *helpError) Error() string {
	var msg string
	if xerrors.Is(e.err, flag.ErrHelp) {
		msg = usageText
	} else {
		msg = e.err.Error()
	}
	return msg + "\nFor more information, run go doc golang.org/x/exp/cmd/gorelease"
}

type downloadError struct {
	m   module.Version
	err error
}

func (e *downloadError) Error() string {
	var msg string
	if xerr, ok := e.err.(*exec.ExitError); ok {
		msg = strings.TrimSpace(string(xerr.Stderr))
	} else {
		msg = e.err.Error()
	}
	return fmt.Sprintf("error downloading module %s@%s: %s", e.m.Path, e.m.Version, msg)
}
