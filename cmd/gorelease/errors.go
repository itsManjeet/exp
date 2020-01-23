// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/xerrors"
)

type usageError struct {
	err error
}

func usageErrorf(format string, args ...interface{}) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

const usageText = `usage: gorelease [-base=version] [-version=version]`

func (e *usageError) Error() string {
	msg := ""
	if !xerrors.Is(e.err, flag.ErrHelp) {
		msg = e.err.Error()
	}
	return usageText + "\n" + msg + "\nFor more information, run go doc golang.org/x/exp/cmd/gorelease"
}

type baseVersionError struct {
	err error
}

func (e *baseVersionError) Error() string {
	return fmt.Sprintf("could not find base version: %v", e.err)
}

func (e *baseVersionError) Unwrap() error {
	return e.err
}

type downloadError struct {
	m   module.Version
	err error
}

func (e *downloadError) Error() string {
	msg := stderrFromExitError(e.err).Error()
	sep := " "
	if strings.Contains(msg, "\n") {
		sep = "\n"
	}
	return fmt.Sprintf("error downloading module %s@%s:%s%s", e.m.Path, e.m.Version, sep, msg)
}

func stderrFromExitError(err error) error {
	if xerr, ok := err.(*exec.ExitError); ok {
		return errors.New(strings.TrimSpace(string(xerr.Stderr)))
	} else {
		return err
	}
}
