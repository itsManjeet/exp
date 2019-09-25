// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.12

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/exp/cmd/gorelease/internal/base"
	"golang.org/x/exp/cmd/gorelease/internal/cfg"
)

// mkenv creates the environment used to run Go tools.
// It is copied from cmd/go/internal/envcmd, which imports
// packages not suitable for the standalone gorelease.
func mkenv() []cfg.EnvVar {
	env := []cfg.EnvVar{
		{Name: "GOARCH", Value: cfg.Goarch},
		{Name: "GOBIN", Value: cfg.GOBIN},
		{Name: "GOCACHE", Value: defaultCacheDir()},
		{Name: "GOEXE", Value: cfg.ExeSuffix},
		{Name: "GOFLAGS", Value: os.Getenv("GOFLAGS")},
		{Name: "GOHOSTARCH", Value: runtime.GOARCH},
		{Name: "GOHOSTOS", Value: runtime.GOOS},
		{Name: "GOOS", Value: cfg.Goos},
		{Name: "GOPATH", Value: cfg.BuildContext.GOPATH},
		{Name: "GOPROXY", Value: os.Getenv("GOPROXY")},
		{Name: "GORACE", Value: os.Getenv("GORACE")},
		{Name: "GOROOT", Value: cfg.GOROOT},
		{Name: "GOTMPDIR", Value: os.Getenv("GOTMPDIR")},
		{Name: "GOTOOLDIR", Value: base.ToolDir},
	}

	switch cfg.Goarch {
	case "arm":
		env = append(env, cfg.EnvVar{Name: "GOARM", Value: cfg.GOARM})
	case "386":
		env = append(env, cfg.EnvVar{Name: "GO386", Value: cfg.GO386})
	case "mips", "mipsle":
		env = append(env, cfg.EnvVar{Name: "GOMIPS", Value: cfg.GOMIPS})
	case "mips64", "mips64le":
		env = append(env, cfg.EnvVar{Name: "GOMIPS64", Value: cfg.GOMIPS64})
	}

	cc := cfg.DefaultCC(cfg.Goos, cfg.Goarch)
	if env := strings.Fields(os.Getenv("CC")); len(env) > 0 {
		cc = env[0]
	}
	cxx := cfg.DefaultCXX(cfg.Goos, cfg.Goarch)
	if env := strings.Fields(os.Getenv("CXX")); len(env) > 0 {
		cxx = env[0]
	}
	env = append(env, cfg.EnvVar{Name: "CC", Value: cc})
	env = append(env, cfg.EnvVar{Name: "CXX", Value: cxx})

	if cfg.BuildContext.CgoEnabled {
		env = append(env, cfg.EnvVar{Name: "CGO_ENABLED", Value: "1"})
	} else {
		env = append(env, cfg.EnvVar{Name: "CGO_ENABLED", Value: "0"})
	}

	return env
}

var (
	defaultDirOnce sync.Once
	defaultDir     string
	defaultDirErr  error
)

// defaultCacheDir returns the effective GOCACHE setting.
// It returns "off" if the cache is disabled.
// It is based on cmd/go/internal/cache.DefaultDir, which is not
// appropriate for the standalone gorelease to import.
func defaultCacheDir() string {
	// Save the result of the first call to DefaultDir for later use in
	// initDefaultCache. cmd/go/main.go explicitly sets GOCACHE so that
	// subprocesses will inherit it, but that means initDefaultCache can't
	// otherwise distinguish between an explicit "off" and a UserCacheDir error.

	defaultDirOnce.Do(func() {
		defaultDir = os.Getenv("GOCACHE")
		if filepath.IsAbs(defaultDir) || defaultDir == "off" {
			return
		}
		if defaultDir != "" {
			defaultDir = "off"
			defaultDirErr = fmt.Errorf("GOCACHE is not an absolute path")
			return
		}

		// Compute default location.
		dir, err := os.UserCacheDir()
		if err != nil {
			defaultDir = "off"
			defaultDirErr = fmt.Errorf("GOCACHE is not defined and %v", err)
			return
		}
		defaultDir = filepath.Join(dir, "go-build")
	})

	return defaultDir
}
