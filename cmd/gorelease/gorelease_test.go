// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.12

package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var workDir string

var (
	testwork     = flag.Bool("testwork", false, "if true, work directory will be preserved")
	updateGolden = flag.Bool("updategolden", false, "if true, expected text in test files will be updated")
)

func TestMain(m *testing.M) {
	status := 1
	defer func() {
		if !*testwork && workDir != "" {
			os.RemoveAll(workDir)
		}
		os.Exit(status)
	}()

	flag.Parse()

	var err error
	workDir, err = ioutil.TempDir("", "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	if *testwork {
		fmt.Fprintf(os.Stderr, "test work dir: %s\n", workDir)
	}

	// The testdata directory should contain a subdirectory for each
	// test repository. Here, we extract the repository .zip files to our
	// work directory.
	infos, err := ioutil.ReadDir("testdata")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	for _, info := range infos {
		if !info.IsDir() {
			continue
		}
		name := info.Name()
		zipPath := filepath.Join("testdata", name, name+".zip")
		if _, err := os.Stat(zipPath); os.IsNotExist(err) {
			continue
		}
		if err := extractZip(workDir, zipPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
	}

	status = m.Run()
}

func TestRelease(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("could not locate git")
	}

	var testPaths []string
	err := filepath.Walk("testdata", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".test") {
			testPaths = append(testPaths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(testPaths) == 0 {
		t.Error("no .test files found in testdata directory")
	}

	for _, testPath := range testPaths {
		testName := filepath.ToSlash(testPath)[len("testdata/") : len(testPath)-len(".test")]
		t.Run(testName, func(t *testing.T) {
			// Read the test file, and find a line that contains "---".
			// Above this are key=value configuration settings.
			// Below this is the expected output.
			var f *os.File
			var err error
			if *updateGolden {
				f, err = os.OpenFile(testPath, os.O_RDWR, 0666)
			} else {
				f, err = os.Open(testPath)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := f.Close(); err != nil && *updateGolden {
					t.Fatalf("error closing golden file: %v", err)
				}
			}()

			data, err := ioutil.ReadAll(f)
			if err != nil {
				t.Fatal(err)
			}
			var wantOffset int64
			sep := []byte("\n---\n")
			sepOffset := bytes.Index(data, sep)
			if sepOffset < 0 {
				t.Fatalf("%s: could not find separator", testPath)
			}
			wantOffset = int64(sepOffset + len(sep))
			configData := data[:sepOffset]
			want := bytes.TrimSpace(data[wantOffset:])

			var dir, baseVersion, releaseVersion, skip string
			var short, wantErr bool
			revision := "master"
			wantSuccess := true
			for n, line := range bytes.Split(configData, []byte("\n")) {
				lineNum := n + 1
				if i := bytes.IndexByte(line, '#'); i >= 0 {
					line = line[:i]
				}
				line = bytes.TrimSpace(line)
				if len(line) == 0 {
					continue
				}
				var key, value string
				if i := bytes.IndexByte(line, '='); i < 0 {
					t.Fatalf("%s:%d: no '=' found", testPath, lineNum)
				} else {
					key = string(line[:i])
					value = string(line[i+1:])
				}
				switch key {
				case "dir":
					dir = value
				case "revision":
					revision = value
				case "error":
					wantErr, err = strconv.ParseBool(value)
					if err != nil {
						t.Fatalf("%s:%d: %v", testPath, lineNum, err)
					}
				case "success":
					wantSuccess, err = strconv.ParseBool(value)
					if err != nil {
						t.Fatalf("%s:%d: %v", testPath, lineNum, err)
					}
				case "short":
					short, err = strconv.ParseBool(value)
					if err != nil {
						t.Fatalf("%s:%d: %v", testPath, lineNum, err)
					}
				case "skip":
					skip = value
				case "base":
					baseVersion = value
				case "version":
					releaseVersion = value
				default:
					t.Fatalf("%s:%d: unknown key: %q", testPath, lineNum+1, key)
				}
			}
			if short && testing.Short() {
				// Go builders disable networking in some configurations.
				// We need networking for a few tests that report errors when go.mod
				// and go.sum should refer to external modules but don't.
				// As with cmd/go/script_test.go, we use -short to indicate that
				// networking is disabled.
				t.Skip("short")
			}
			if skip != "" {
				if b, err := strconv.ParseBool(skip); err == nil {
					if b {
						t.Skip(string(want))
					}
				} else {
					t.Skip(skip)
				}
			}

			// Checkout the release version.
			// Rename the repo first to defeat caching. If the repo is cached, the
			// commit for HEAD will be saved in memory, even though we change it
			// on disk.
			repo := filepath.Base(filepath.Dir(testPath))
			origRepoDir := filepath.Join(workDir, repo)
			testSuffix := strings.Replace(testName, "/", "_", -1)
			repoDir := origRepoDir + "-TestRelease." + testSuffix
			if err := os.Rename(origRepoDir, repoDir); err != nil {
				t.Fatalf("error renaming repo: %v", err)
			}
			defer func() {
				if err := os.Rename(repoDir, origRepoDir); err != nil {
					t.Fatalf("error restoring repo: %v", err)
				}
			}()

			cmd := exec.Command("git", "checkout", "--quiet", revision)
			cmd.Dir = repoDir
			if _, err := cmd.Output(); err != nil {
				t.Fatalf("could not checkout revision %q: %v", revision, err)
			}

			testDir := repoDir
			if dir != "" {
				testDir = filepath.Join(testDir, dir)
			}

			// Generate the report and compare it against the expected text.
			r, err := makeReleaseReport(testDir, baseVersion, releaseVersion)
			if wantErr {
				if err == nil {
					t.Fatalf("got success; want error:\n%s", want)
				}
				got := []byte(err.Error())
				if !bytes.Equal(got, want) {
					if *updateGolden {
						updateGoldenFile(t, f, wantOffset, got)
					} else {
						t.Errorf("got error:\n%s\n\nwant error:\n%s", got, want)
					}
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				buf := &bytes.Buffer{}
				if err := r.Text(buf); err != nil {
					t.Fatal(err)
				}
				got := bytes.TrimSpace(buf.Bytes())
				if !bytes.Equal(got, want) {
					if *updateGolden {
						updateGoldenFile(t, f, wantOffset, got)
					} else {
						t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
					}
				}
				success := r.isSuccessful()
				if success != wantSuccess {
					t.Errorf("success: got %t, want %t", success, wantSuccess)
				}
			}
		})
	}
}

func extractZip(destDir, zipPath string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	extractFile := func(f *zip.File) (err error) {
		outPath := filepath.Join(destDir, f.Name)
		if strings.HasSuffix(f.Name, "/") {
			return os.MkdirAll(outPath, 0777)
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0777); err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		defer r.Close()
		w, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := w.Close(); err == nil && cerr != nil {
				err = cerr
			}
		}()
		if _, err := io.Copy(w, r); err != nil {
			return err
		}
		return nil
	}

	for _, f := range zr.File {
		if err := extractFile(f); err != nil {
			return err
		}
	}
	return nil
}

func updateGoldenFile(t *testing.T, f *os.File, offset int64, got []byte) {
	if err := f.Truncate(offset); err != nil {
		t.Fatalf("error truncating golden file: %v", err)
	}
	if _, err := f.Seek(0, 2); err != nil {
		t.Fatalf("error seeking golden file: %v", err)
	}
	if _, err := f.Write(got); err != nil {
		t.Fatalf("error writing golden file: %v", err)
	}
}
