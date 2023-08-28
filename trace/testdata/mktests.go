// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/exp/trace/internal/raw"
	"golang.org/x/exp/trace/internal/version"
	"golang.org/x/tools/txtar"
)

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	generators, err := filepath.Glob("./generators/*.go")
	if err != nil {
		return fmt.Errorf("reading generators: %v", err)
	}
	genroot := "./staging"

	// Grab a pattern, if any.
	var re *regexp.Regexp
	if pattern := os.Getenv("GOTRACETEST"); pattern != "" {
		re, err = regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("compiling regexp %q for GOTRACETEST: %v", pattern, err)
		}
	}

	if err := os.MkdirAll(genroot, 0777); err != nil {
		return fmt.Errorf("creating generated root: %v", err)
	}
	for _, path := range generators {
		name := filepath.Base(path)
		name = name[:len(name)-len(filepath.Ext(name))]
		isProg := strings.HasPrefix(name, "prog")

		// Skip if we have a pattern and this test doesn't match.
		if re != nil && !re.MatchString(name) {
			continue
		}

		fmt.Fprintf(os.Stderr, "generating %s... ", name)

		start := time.Now()
		f, err := os.CreateTemp("", fmt.Sprintf("trace-test-gen-%s-", name))
		if err != nil {
			return fmt.Errorf("creating temporary trace.out: %v", err)
		}
		traceTmp := f.Name()
		f.Close()

		// Run generator.
		cmd := exec.Command("go", "run", path, traceTmp)
		cmd.Env = append(os.Environ(), "GOTRACEBACK=crash", "GOMAXPROCS=4")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("running generator %s: %v:\n%s", name, err, out)
		}

		// Get the test result into the right place.
		testPath := filepath.Join(genroot, fmt.Sprintf("%s.test", name))
		if isProg {
			// This test is a Go program generating a trace. We need to turn
			// the result into a test.
			if err := traceBytesToTestData(traceTmp, testPath); err != nil {
				return fmt.Errorf("creating test for generator %s: %v", name, err)
			}
		} else {
			// It's already a complete test, so just move it into place.
			if err := os.Rename(traceTmp, testPath); err != nil {
				return fmt.Errorf("copying test %s from %s to %s: %v", name, traceTmp, testPath, err)
			}
		}

		// Print progress.
		fmt.Fprintln(os.Stderr, time.Since(start))
	}
	return nil
}

func traceBytesToTestData(tracePath, testPath string) error {
	in, err := os.Open(tracePath)
	if err != nil {
		return fmt.Errorf("opening trace at %q: %v", tracePath, err)
	}
	defer in.Close()

	// Create text.
	var outBuf bytes.Buffer
	tr, err := raw.NewReader(bufio.NewReader(in))
	if err != nil {
		return fmt.Errorf("creating trace reader for %q: %v", tracePath, err)
	}
	tw, err := raw.NewTextWriter(&outBuf, version.Go122)
	if err != nil {
		return fmt.Errorf("creating trace writer: %v", err)
	}
	for {
		ev, err := tr.ReadEvent()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("parsing trace %q: %v", tracePath, err)
		}
		if err := tw.WriteEvent(ev); err != nil {
			return fmt.Errorf("writing trace: %v", err)
		}
	}

	// Write out test.
	testBytes := txtar.Format(&txtar.Archive{
		Files: []txtar.File{
			{Name: "expect", Data: []byte("SUCCESS\n")},
			{Name: "trace", Data: outBuf.Bytes()},
		},
	})
	out, err := os.Create(testPath)
	if err != nil {
		return fmt.Errorf("creating test at %q: %v", testPath, err)
	}
	if _, err := out.Write(testBytes); err != nil {
		return fmt.Errorf("writing test to %q: %v", testPath, err)
	}
	defer out.Close()
	return nil
}
