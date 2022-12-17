// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog_test

import (
	"fmt"
	"os"

	"golang.org/x/exp/slog"
)

func Infof(format string, args ...any) {
	// Use LogDepth to adjust source line information to point to the caller of Infof.
	// The 0 passed to LogDepth refers to the caller of LogDepth, namely this function.
	slog.Default().LogDepth(0, slog.LevelInfo, fmt.Sprintf(format, args...))
}

func ExampleLogger_LogDepth() {
	defer func(l *slog.Logger) { slog.SetDefault(l) }(slog.Default())

	removeTime := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey && len(groups) == 0 {
			a.Key = ""
		}
		return a
	}
	logger := slog.New(slog.HandlerOptions{AddSource: true, ReplaceAttr: removeTime}.NewTextHandler(os.Stdout))
	slog.SetDefault(logger)
	Infof("message, %s", "formatted")

	// Output will refer to the line above.
}
