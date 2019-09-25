// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/zip"
	"golang.org/x/tools/txtar"
)

// buildProxyDir constructs a temporary directory suitable for use as a
// module proxy with a file:// URL. The caller is responsible for deleting
// the directory when it's no longer needed.
func buildProxyDir() (proxyDir, proxyURL string, err error) {
	proxyDir, err = ioutil.TempDir("", "gorelease-proxy")
	if err != nil {
		return "", "", err
	}
	defer func(proxyDir string) {
		if err != nil {
			os.RemoveAll(proxyDir)
		}
	}(proxyDir)

	txtarPaths, err := filepath.Glob(filepath.FromSlash("testdata/mod/*.txt"))
	if err != nil {
		return "", "", err
	}
	versionLists := make(map[string][]string)
	for _, txtarPath := range txtarPaths {
		base := filepath.Base(txtarPath)
		stem := base[:len(base)-len(".txt")]
		i := strings.LastIndexByte(base, '_')
		if i < 0 {
			return "", "", fmt.Errorf("invalid module archive: %s", base)
		}
		modPath := strings.ReplaceAll(stem[:i], "_", "/")
		version := stem[i+1:]
		versionLists[modPath] = append(versionLists[modPath], version)

		modDir := filepath.Join(proxyDir, modPath, "@v")
		if err := os.MkdirAll(modDir, 0777); err != nil {
			return "", "", err
		}

		arc, err := txtar.ParseFile(txtarPath)
		if err != nil {
			return "", "", err
		}

		var zipContents []zip.File
		var haveInfo, haveMod bool
		var goMod txtar.File
		for _, af := range arc.Files {
			if af.Name == ".info" || af.Name == ".mod" {
				if af.Name == ".info" {
					haveInfo = true
				} else {
					haveMod = true
				}
				outPath := filepath.Join(modDir, version+af.Name)
				if err := ioutil.WriteFile(outPath, af.Data, 0666); err != nil {
					return "", "", err
				}
				continue
			}
			if af.Name == "go.mod" {
				goMod = af
			}

			zipContents = append(zipContents, txtarFile{af})
		}

		if !haveInfo {
			outPath := filepath.Join(modDir, version+".info")
			outContent := fmt.Sprintf(`{"Version":"%s"}`, version)
			if err := ioutil.WriteFile(outPath, []byte(outContent), 0666); err != nil {
				return "", "", err
			}
		}
		if !haveMod {
			if goMod.Name == "" {
				return "", "", fmt.Errorf("%s: no .mod or go.mod file found", txtarPath)
			}
			outPath := filepath.Join(modDir, version+".mod")
			if err := ioutil.WriteFile(outPath, goMod.Data, 0666); err != nil {
				return "", "", err
			}
		}

		zipPath := filepath.Join(modDir, version+".zip")
		zipFile, err := os.Create(zipPath)
		if err != nil {
			return "", "", err
		}
		defer zipFile.Close()
		if err := zip.Create(zipFile, module.Version{Path: modPath, Version: version}, zipContents); err != nil {
			return "", "", err
		}
		if err := zipFile.Close(); err != nil {
			return "", "", err
		}
	}

	buf := &bytes.Buffer{}
	for modPath, versions := range versionLists {
		outPath := filepath.Join(proxyDir, modPath, "@v", "list")
		sort.Slice(versions, func(i, j int) bool {
			return semver.Compare(versions[i], versions[j]) < 0
		})
		for _, v := range versions {
			fmt.Fprintln(buf, v)
		}
		if err := ioutil.WriteFile(outPath, buf.Bytes(), 0666); err != nil {
			return "", "", err
		}
		buf.Reset()
	}

	// Make sure the URL path starts with a slash on Windows. Absolute paths
	// normally start with a drive letter.
	// TODO(golang.org/issue/32456): use url.FromFilePath when implemented.
	if strings.HasPrefix(proxyDir, "/") {
		proxyURL = "file://" + proxyDir
	} else {
		proxyURL = "file:///" + filepath.FromSlash(proxyDir)
	}
	return proxyDir, proxyURL, nil
}

type txtarFile struct {
	f txtar.File
}

func (f txtarFile) Path() string                { return f.f.Name }
func (f txtarFile) Lstat() (os.FileInfo, error) { return txtarFileInfo{f.f}, nil }
func (f txtarFile) Open() (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader(f.f.Data)), nil
}

type txtarFileInfo struct {
	f txtar.File
}

func (f txtarFileInfo) Name() string       { return f.f.Name }
func (f txtarFileInfo) Size() int64        { return int64(len(f.f.Data)) }
func (f txtarFileInfo) Mode() os.FileMode  { return 0444 }
func (f txtarFileInfo) ModTime() time.Time { return time.Time{} }
func (f txtarFileInfo) IsDir() bool        { return false }
func (f txtarFileInfo) Sys() interface{}   { return nil }

func extractTxtar(destDir string, arc *txtar.Archive) error {
	for _, f := range arc.Files {
		outPath := filepath.Join(destDir, f.Name)
		if err := os.MkdirAll(filepath.Dir(outPath), 0777); err != nil {
			return err
		}
		if err := ioutil.WriteFile(outPath, f.Data, 0666); err != nil {
			return err
		}
	}
	return nil
}
