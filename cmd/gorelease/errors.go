// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.12

package main

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/mod/module"
)

type helpError struct {
	err error
}

func helpErrorf(format string, args ...interface{}) error {
	return &helpError{err: fmt.Errorf(format, args...)}
}

func (e *helpError) Error() string {
	if e.err == flag.ErrHelp {
		return helpText
	}
	return fmt.Sprintf("%v\nFor more information, run gorelease -h", e.err)
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
	return fmt.Sprintf("error downloading module %s@%s to temporary directory: %s", e.m.Path, e.m.Version, msg)
}
