// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fakemodfetch

import (
	"os"
	"path/filepath"
)

// Checkout creates a zip of a specific module version, then extracts it
// in the given directory.
// based on cmd/go/internal/modfetch.Download
func Checkout(repo Repo, vers, scratchDir string) (dir string, err error) {
	// Create a zip file for the module at the specific version.
	// This should match the zip that cmd/go would create.
	info, err := repo.Stat(vers)
	if err != nil {
		return "", err
	}
	statVers := info.Version

	zipPath := filepath.Join(scratchDir, statVers+".zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}

	if err := repo.Zip(zipFile, statVers); err != nil {
		zipFile.Close()
		return "", err
	}
	if err := zipFile.Close(); err != nil {
		return "", err
	}

	dir = filepath.Join(scratchDir, vers)
	prefix := repo.ModulePath() + "@" + statVers
	if err := Unzip(dir, zipPath, prefix, 0); err != nil {
		return "", err
	}
	return dir, nil
}
