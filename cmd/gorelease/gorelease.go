// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gorelease is an experimental tool that helps module authors avoid common
// problems before releasing a new version of a module.
//
// gorelease suggests a new version tag that satisfies semantic versioning
// requirements by comparing the public API of a module at two revisions:
// a base version and the currently checked out revision.
//
// If there are no differences in the module's public API, gorelease will
// suggest a new version that increments the base version's patch version
// number.  For example, if the base version is "v2.3.1", gorelease would
// suggest "v2.3.2" as the new version.
//
// If there are only compatible differences in the module's public API,
// gorelease will suggest a new version that increments the base version's
// minor version number. For example, if the base version is "v2.3.1",
// gorelease will suggest "v2.4.0".
//
// If there are incompatible differences, gorelease will exit with a non-zero
// status. Incompatible differences may only be released in a new major
// version, which involves creating a module with a different path. For
// example, if incompatible changes are made in the module "rsc.io/quote", a
// new major version must be released as a new module, "rsc.io/quote/v2".
//
// gorelease accepts the following flags:
//
// -base=version: The version that the current version of the module will be
// compared against. The version must be a semantic version (for example,
// "v2.3.4") or "none". If the version is "none", gorelease will not compare the
// current version against any previous version; it will only validate the
// current version. This is useful for checking the first release of a new major
// version.
//
// -version=version: The proposed version to be released. If specified,
// gorelease will confirm whether this version is consistent with changes made
// to the module's public API. gorelease will exit with a non-zero status if the
// version is not valid.
//
// gorelease is eventually intended to be merged into the go command
// as "go release". See golang.org/issues/26420.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/zip"

	"golang.org/x/exp/apidiff"
	"golang.org/x/tools/go/packages"
)

// IDEAS:
// * Should we suggest versions at all or should -release be mandatory?
// * 'gorelease path1@version1 path2@version2' should compare two arbitrary
//   modules. Useful for comparing differences in forks.
// * Verify downstream modules have licenses. May need an API or library
//   for this. Be clear that we can't provide legal advice.
// * Should updating the go version in go.mod be an incompatible change?
//   should we detect usage of new language or standard library features
//   and recommend updating the go version?
// * Internal packages may be relevant to submodules (for example,
//   golang.org/x/tools/internal/lsp is imported by golang.org/x/tools).
//   gorelease should detect whether this is the case and include internal
//   directories in comparison. It should be possible to opt out or specify
//   a different list of submodules.

// TODO(jayconrod):
// * Report errors when packages can't be loaded without replace / exclude.
// * Clean up overuse of fmt.Errorf.
// * Support -json output.
// * Don't suggest a release tag that already exists.
// * Suggest a minor release if dependency has been bumped by minor version.
// * Support migration to modules after v2.x.y+incompatible. Requires comparing
//   packages with different module paths.
// * Error when packages import from earlier major version of same module.
// * Check that proposed prerelease will not sort below pseudo-versions.
// * Error messages point to HTML documentation.
// * Positional arguments should specify which packages to check. Without
//   these, we check all non-internal packages in the module.
// * Mechanism to suppress error messages.
// * Check that the main module does not transitively require a newer version
//   of itself.
// * Invalid file names and import paths should be reported sensibly.
//   golang.org/x/mod/zip should return structured errors for this.

func main() {
	log.SetFlags(0)
	log.SetPrefix("gorelease: ")
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	success, err := runRelease(os.Stdout, wd, os.Args[1:])
	if err != nil {
		if _, ok := err.(*helpError); ok {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		} else {
			log.Fatal(err)
		}
	}
	if !success {
		os.Exit(1)
	}
}

// runRelease is the main function of gorelease. It's called by tests, so
// it writes to w instead of os.Stdout and returns an error instead of
// exiting.
func runRelease(w io.Writer, dir string, args []string) (success bool, err error) {
	// Validate arguments and flags. We'll print our own errors, since we want to
	// test without printing to stderr.
	fs := flag.NewFlagSet("gorelease", flag.ContinueOnError)
	fs.Usage = func() {}
	fs.SetOutput(ioutil.Discard)
	var baseVersion, releaseVersion string
	fs.StringVar(&baseVersion, "base", "", "previous version to compare against")
	fs.StringVar(&releaseVersion, "version", "", "proposed version to be released")
	if err := fs.Parse(args); err != nil {
		return false, &helpError{err: err}
	}

	if len(fs.Args()) > 0 {
		return false, helpErrorf("no arguments allowed")
	}
	if baseVersion == "" {
		return false, helpErrorf("-base flag must be specified.\nUse -base=none if there is no previous version.")
	}
	if baseVersion != "none" {
		if c := semver.Canonical(baseVersion); c != baseVersion {
			return false, helpErrorf("base version %q is not a valid semantic version", baseVersion)
		}
	}
	if releaseVersion != "" {
		if c := semver.Canonical(releaseVersion); c != releaseVersion {
			return false, helpErrorf("release version %q is not a valid semantic version", releaseVersion)
		}
	}
	if baseVersion != "none" && releaseVersion != "" {
		if cmp := semver.Compare(baseVersion, releaseVersion); cmp == 0 {
			return false, helpErrorf("-base and -version must be different")
		} else if cmp > 0 {
			return false, helpErrorf("base version (%q) must be lower than release version (%q)", baseVersion, releaseVersion)
		}
	}

	modRoot, err := findModuleRoot(dir)
	if err != nil {
		return false, err
	}
	repoRoot := findRepoRoot(modRoot)
	if repoRoot == "" {
		repoRoot = modRoot
	}

	report, err := makeReleaseReport(modRoot, repoRoot, baseVersion, releaseVersion)
	if err != nil {
		return false, err
	}
	if err := report.Text(w); err != nil {
		return false, err
	}
	return report.isSuccessful(), nil
}

// makeReleaseReport returns a report comparing the current version of a
// module with a previously released version. The report notes any backward
// compatible and incompatible changes in the module's public API. It also
// diagnoses common problems, such as go.mod or go.sum being incomplete.
// The report recommends or validates a release version and suggests a
// version control tag to use.
//
// modRoot is the directory containing the module's go.mod file.
//
// repoRoot the root directory of the version control repository containing
// modRoot.
//
// baseVersion is a previously released version of the module to compare.
// If baseVersion is "none", no comparison will be performed, and
// the returned report will only describe problems with the release version.
//
// releaseVersion is the proposed version for the module in dir.
// If releaseVersion is "", the report will suggest a release version based on
// changes to the public API.
func makeReleaseReport(modRoot, repoRoot, baseVersion, releaseVersion string) (report, error) {
	if !hasFilePathPrefix(modRoot, repoRoot) {
		// runRelease should always make sure this is true.
		return report{}, fmt.Errorf("module root %q is not in repository root %q", modRoot, repoRoot)
	}

	// Read the module path from the go.mod file.
	goModPath := filepath.Join(modRoot, "go.mod")
	goModData, err := ioutil.ReadFile(goModPath)
	if err != nil {
		return report{}, err
	}
	modFile, err := modfile.ParseLax(goModPath, goModData, nil)
	if err != nil {
		return report{}, err
	}
	if modFile.Module == nil {
		return report{}, fmt.Errorf("no module statement in %s", goModPath)
	}

	modPath := modFile.Module.Mod.Path
	if err := checkModPath(modPath); err != nil {
		return report{}, err
	}
	_, modPathMajor, ok := module.SplitPathVersion(modPath)
	if !ok {
		// we just validated the path above.
		panic(fmt.Sprintf("could not find version suffix in module path %q", modPath))
	}

	if baseVersion != "none" {
		if err := module.Check(modPath, baseVersion); err != nil {
			return report{}, fmt.Errorf("can't compare major versions: base version %s does not belong to module %s", baseVersion, modPath)
		}
	}
	// releaseVersion is checked by report.validateVersion.

	// Determine the version tag prefix for the module within the repository.
	tagPrefix := ""
	if modRoot != repoRoot {
		if strings.HasPrefix(modPathMajor, ".") {
			return report{}, fmt.Errorf("%s: module path starts with gopkg.in and must be declared in the root directory of the repository", modPath)
		}
		codeDir := filepath.ToSlash(modRoot[len(repoRoot)+1:])
		if modPathMajor == "" {
			// module has no major version suffix.
			// codeDir must be a suffix of modPath.
			// tagPrefix is codeDir with a trailing slash.
			if !strings.HasSuffix(modPath, "/"+codeDir) {
				return report{}, fmt.Errorf("%s: module path must end with %[2]q, since it is in subdirectory %[2]q", modPath, codeDir)
			}
			tagPrefix = codeDir + "/"
		} else {
			if strings.HasSuffix(modPath, "/"+codeDir) {
				// module has a major version suffix and is in a major version subdirectory.
				// codeDir must be a suffix of modPath.
				// tagPrefix must not include the major version.
				tagPrefix = codeDir[:len(codeDir)-len(modPathMajor)+1]
			} else if strings.HasSuffix(modPath, "/"+codeDir+modPathMajor) {
				// module has a major version suffix and is not in a major version subdirectory.
				// codeDir + modPathMajor is a suffix of modPath.
				// tagPrefix is codeDir with a trailing slash.
				tagPrefix = codeDir + "/"
			} else {
				return report{}, fmt.Errorf("%s: module path must end with %[2]q or %q, since it is in subdirectory %[2]q", modPath, codeDir, codeDir+modPathMajor)
			}
		}
	}

	// Load the base version of the module.
	var basePkgs []*packages.Package
	if baseVersion != "none" {
		baseMod := module.Version{Path: modPath, Version: baseVersion}
		baseDir, goModPath, err := downloadModule(baseMod)
		if err != nil {
			return report{}, err
		}
		if basePkgs, _, err = loadPackages(modPath, baseDir, goModPath, false); err != nil {
			return report{}, err
		}
	}

	// Copy the current version to a temporary directory and load it from there.
	// TODO(jayconrod): this won't be necessary when the oldest supported version
	// of the go command supports -modfile (golang.org/issue/34506).
	releaseDir, err := copyModuleToTempDir(modPath, modRoot)
	if err != nil {
		return report{}, err
	}
	defer os.RemoveAll(releaseDir)
	releaseGoModPath := filepath.Join(releaseDir, "go.mod")
	releasePkgs, diagnostics, err := loadPackages(modPath, releaseDir, releaseGoModPath, true)
	if err != nil {
		return report{}, err
	}

	// Compare each pair of packages.
	// Ignore internal packages.
	// If we don't have a base version to compare against,
	// just check the new packages for errors.
	shouldCompare := baseVersion != "none"
	isInternal := func(pkgPath string) bool {
		if !hasPathPrefix(pkgPath, modPath) {
			panic(fmt.Sprintf("package %s not in module %s", pkgPath, modPath))
		}
		for pkgPath != modPath {
			if path.Base(pkgPath) == "internal" {
				return true
			}
			pkgPath = path.Dir(pkgPath)
		}
		return false
	}
	baseIndex, releaseIndex := 0, 0
	r := report{
		modulePath:     modPath,
		baseVersion:    baseVersion,
		releaseVersion: releaseVersion,
		tagPrefix:      tagPrefix,
		diagnostics:    diagnostics,
	}
	for baseIndex < len(basePkgs) || releaseIndex < len(releasePkgs) {
		if baseIndex < len(basePkgs) && (releaseIndex == len(releasePkgs) || basePkgs[baseIndex].PkgPath < releasePkgs[releaseIndex].PkgPath) {
			// Package removed
			basePkg := basePkgs[baseIndex]
			baseIndex++
			if !isInternal(basePkg.PkgPath) || len(basePkg.Errors) > 0 {
				pr := packageReport{
					path:      basePkg.PkgPath,
					oldErrors: basePkg.Errors,
				}
				if !isInternal(basePkg.PkgPath) {
					pr.Report = apidiff.Report{
						Changes: []apidiff.Change{{
							Message:    "package removed",
							Compatible: false,
						}},
					}
				}
				r.addPackage(pr)
			}
		} else if releaseIndex < len(releasePkgs) && (baseIndex == len(basePkgs) || releasePkgs[releaseIndex].PkgPath < basePkgs[baseIndex].PkgPath) {
			// Package added
			releasePkg := releasePkgs[releaseIndex]
			releaseIndex++
			if !isInternal(releasePkg.PkgPath) && shouldCompare || len(releasePkg.Errors) > 0 {
				pr := packageReport{
					path:      releasePkg.PkgPath,
					newErrors: releasePkg.Errors,
				}
				if !isInternal(releasePkg.PkgPath) && shouldCompare {
					// If we aren't comparing against a base version, don't say
					// "package added". Only report packages with errors.
					pr.Report = apidiff.Report{
						Changes: []apidiff.Change{{
							Message:    "package added",
							Compatible: true,
						}},
					}
				}
				r.addPackage(pr)
			}
		} else {
			// Matched packages.
			oldPkg := basePkgs[baseIndex]
			newPkg := releasePkgs[releaseIndex]
			baseIndex++
			releaseIndex++
			if !isInternal(oldPkg.PkgPath) && oldPkg.Name != "main" && newPkg.Name != "main" {
				pr := packageReport{
					path:      oldPkg.PkgPath,
					oldErrors: oldPkg.Errors,
					newErrors: newPkg.Errors,
				}
				if len(oldPkg.Errors) == 0 && len(newPkg.Errors) == 0 {
					pr.Report = apidiff.Changes(oldPkg.Types, newPkg.Types)
				}
				r.addPackage(pr)
			}
		}
	}

	return r, nil
}

// findRepoRoot finds the root directory of the repository that contains dir.
// findRepoRoot returns "" if it can't find the repository root.
func findRepoRoot(dir string) string {
	vcsDirs := []string{".git", ".hg", ".svn", ".bzr"}
	d := filepath.Clean(dir)
	for {
		for _, vcsDir := range vcsDirs {
			if _, err := os.Stat(filepath.Join(d, vcsDir)); err == nil {
				return d
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// findModuleRoot finds the root directory of the module that contains dir.
func findModuleRoot(dir string) (string, error) {
	d := filepath.Clean(dir)
	for {
		if fi, err := os.Stat(filepath.Join(d, "go.mod")); err == nil && !fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", fmt.Errorf("%s: cannot find go.mod file", dir)
}

// checkModPath is like golang.org/x/mod/module.CheckPath, but it returns
// friendlier error messages for common mistakes.
//
// TODO(jayconrod): update module.CheckPath and delete this function.
func checkModPath(modPath string) error {
	if path.IsAbs(modPath) || filepath.IsAbs(modPath) {
		// TODO(jayconrod): improve error message in x/mod instead of checking here.
		return fmt.Errorf("module path %q must not be an absolute path.\nIt must be an address where your module may be found.", modPath)
	}
	if suffix := dirMajorSuffix(modPath); suffix == "v0" || suffix == "v1" {
		return fmt.Errorf("module path %q has major version suffix %q.\nA major version suffix is only allowed for v2 or later.", modPath, suffix)
	} else if strings.HasPrefix(suffix, "v0") {
		return fmt.Errorf("module path %q has major version suffix %q.\nA major version may not have a leading zero.", modPath, suffix)
	} else if strings.ContainsRune(suffix, '.') {
		return fmt.Errorf("module path %q has major version suffix %q.\nA major version may not contain dots.", modPath, suffix)
	}
	return module.CheckPath(modPath)
}

// dirMajorSuffix returns a major version suffix for a slash-separated path.
// For example, for the path "foo/bar/v2", dirMajorSuffix would return "v2".
// If no major version suffix is found, "" is returned.
//
// dirMajorSuffix is less strict than module.SplitPathVersion so that incorrect
// suffixes like "v0", "v02", "v1.2" can be detected. It doesn't handle
// special cases for gopkg.in paths.
func dirMajorSuffix(path string) string {
	i := len(path)
	for i > 0 && ('0' <= path[i-1] && path[i-1] <= '9') || path[i-1] == '.' {
		i--
	}
	if i <= 1 || i == len(path) || path[i-1] != 'v' || (i > 1 && path[i-2] != '/') {
		return ""
	}
	return path[i-1:]
}

func hasPathPrefix(p, prefix string) bool {
	return p == prefix || len(p) > len(prefix) && p[:len(prefix)] == prefix && p[len(prefix)] == '/'
}

func hasFilePathPrefix(p, prefix string) bool {
	return p == prefix || len(p) > len(prefix) && p[:len(prefix)] == prefix && p[len(prefix)] == os.PathSeparator
}

// splitVersionNumbers returns the major, minor, and patch numbers for a given
// version.
//
// TODO(jayconrod): extend semver to do this and delete this function.
func splitVersionNumbers(vers string) (major, minor, patch string, err error) {
	if !strings.HasPrefix(vers, "v") {
		return "", "", "", fmt.Errorf("version %q does not start with 'v'", vers)
	}
	base := vers[1:]
	if i := strings.IndexByte(vers, '-'); i >= 0 {
		base = base[:i] // trim prerelease
	}
	if i := strings.IndexByte(vers, '+'); i >= 0 {
		base = base[:i] // trim build
	}
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("version %q should have three numbers", vers)
	}
	return parts[0], parts[1], parts[2], nil
}

// copyModuleToTempDir copies module files from modRoot to a subdirectory of
// scratchDir. Submodules, vendor directories, and irregular files are excluded.
// An error is returned if the module contains any files or directories that
// can't be included in a module zip file (due to special characters,
// excessive sizes, etc.).
func copyModuleToTempDir(modPath, modRoot string) (dir string, err error) {
	// Generate a fake version consistent with modPath. We need a valid version
	// to create a zip file.
	version := "v0.0.0-gorelease"
	_, majorPathSuffix, _ := module.SplitPathVersion(modPath)
	if majorPathSuffix != "" {
		version = majorPathSuffix[1:] + ".0.0-gorelease"
	}
	m := module.Version{Path: modPath, Version: version}

	zipFile, err := ioutil.TempFile("", "gorelease-*.zip")
	if err != nil {
		return "", err
	}
	defer zipFile.Close()
	defer os.Remove(zipFile.Name())

	dir, err = ioutil.TempDir("", "gorelease")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
			dir = ""
		}
	}()

	if err := zip.CreateFromDir(zipFile, m, modRoot); err != nil {
		return "", err
	}
	if err := zipFile.Close(); err != nil {
		return "", err
	}
	if err := zip.Unzip(dir, m, zipFile.Name()); err != nil {
		return "", err
	}
	return dir, nil
}

// downloadModule downloads a specific version of a module to the
// module cache using 'go mod download'.
func downloadModule(m module.Version) (modRoot, goModPath string, err error) {
	defer func() {
		if err != nil {
			err = &downloadError{m: m, err: err}
		}
	}()
	cmd := exec.Command("go", "mod", "download", "-json", "--", m.Path+"@"+m.Version)
	out, err := cmd.Output()
	var xerr *exec.ExitError
	if err != nil {
		var ok bool
		if xerr, ok = err.(*exec.ExitError); !ok {
			return "", "", err
		}
	}

	parsed := struct{ Dir, GoMod, Error string }{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return "", "", err
	}
	if parsed.Error != "" {
		return "", "", errors.New(parsed.Error)
	}
	if xerr != nil {
		return "", "", xerr
	}
	return parsed.Dir, parsed.GoMod, nil
}

// loadPackages returns a list of all packages in the module m
// in directory modRoot, sorted by package path. goModPath is the path to the
// go.mod file; for downloaded modules, this may be outside modRoot.
// modRootIsTemp indicates whether modRoot is a temporary directory that can be
// modified.
//
// Package loading errors will be returned in the Errors field of each package.
// Other diagnostics (such as the go.sum file being incomplete) will be
// returned through diagnostics.
// err will be non-nil in case of a fatal error that prevented packages
// from being loaded.
func loadPackages(modPath, modRoot, goModPath string, modRootIsTemp bool) (pkgs []*packages.Package, diagnostics []string, err error) {
	// Copy modRoot to a temporary directory if it's not in one already.
	// We may need to make some modifications to go.mod and go.sum.
	loadDir := modRoot
	if !modRootIsTemp {
		loadDir, err = ioutil.TempDir("", "gorelease-"+path.Base(modPath))
		if err != nil {
			return nil, nil, err
		}
		defer os.RemoveAll(loadDir)
		if err := copyTree(loadDir, modRoot); err != nil {
			return nil, nil, err
		}
	}

	// Load go.mod.
	// Add a go version if one is missing.
	// Remove all replace and exclude directives.
	// Write it back to the temporary directory.
	goModData, err := ioutil.ReadFile(goModPath)
	if err != nil {
		return nil, nil, err
	}
	goMod, err := modfile.Parse(goModPath, goModData, nil)
	if err != nil {
		return nil, nil, err
	}
	if goMod.Module == nil {
		return nil, nil, fmt.Errorf("%s: module directive is missing", goModPath)
	}
	if goMod.Go == nil {
		diagnostics = append(diagnostics, fmt.Sprintf("go.mod: go directive is missing"))
		goVersion := build.Default.ReleaseTags[len(build.Default.ReleaseTags)-1][len("go"):]
		goMod.AddGoStmt(goVersion)
	}
	goMod.Replace = nil
	goMod.Exclude = nil
	goModData, err = goMod.Format()
	if err != nil {
		return nil, nil, err
	}
	loadGoModPath := filepath.Join(loadDir, "go.mod")
	info, err := os.Stat(goModPath)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&0200 == 0 {
		// zip.Unzip create read-only files, since they're intended to be extracted
		// into the module cache. We want go.mod to be writable though.
		if err := os.Chmod(goModPath, info.Mode()|0200); err != nil {
			return nil, nil, err
		}
	}
	if err := ioutil.WriteFile(loadGoModPath, goModData, 0666); err != nil {
		return nil, nil, err
	}

	// Load go.sum. It's okay if it doesn't exist. Make it writable if it does.
	goSumPath := filepath.Join(loadDir, "go.sum")
	var goSumData []byte
	info, err = os.Stat(goSumPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, nil, err
		}
	} else {
		if info.Mode()&0200 == 0 {
			if err := os.Chmod(goSumPath, info.Mode()|0200); err != nil {
				return nil, nil, err
			}
		}
		goSumData, err = ioutil.ReadFile(goSumPath)
		if err != nil {
			return nil, nil, err
		}
	}

	// Load all packages in the module.
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps,
		Dir:  loadDir,
	}
	pkgs, err = packages.Load(cfg, "./...")
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].PkgPath < pkgs[j].PkgPath })

	// Trim loadDir from file paths in errors.
	prefix := loadDir + string(os.PathSeparator)
	for _, pkg := range pkgs {
		for i := range pkg.Errors {
			pkg.Errors[i].Pos = strings.TrimPrefix(pkg.Errors[i].Pos, prefix)
		}
	}

	// Report changes in go.mod and go.sum as diagnostics.
	newGoModData, err := ioutil.ReadFile(loadGoModPath)
	if err != nil {
		return nil, nil, err
	}
	goModChanged := !bytes.Equal(goModData, newGoModData)
	if goModChanged {
		// TODO(jayconrod): report which requirements changed.
		diagnostics = append(diagnostics, "go.mod: requirements are incomplete.\nRun 'go mod tidy' to add missing requirements.")
	}

	if !goModChanged {
		newGoSumData, err := ioutil.ReadFile(goSumPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, err
		}
		if !bytes.Equal(goSumData, newGoSumData) {
			diagnostics = append(diagnostics, "go.sum: one or more sums are missing.\nRun 'go mod tidy' to add missing sums.")
		}
	}

	return pkgs, diagnostics, nil
}

// copyTree recursively copies a directory tree at srcRoot to dstRoot.
func copyTree(dstRoot, srcRoot string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("error copying from %q to %q: %v", srcRoot, dstRoot, err)
		}
	}()

	dstRoot = filepath.Clean(dstRoot)
	srcRoot = filepath.Clean(srcRoot)
	if err := os.MkdirAll(dstRoot, 0777); err != nil {
		return err
	}

	return filepath.Walk(srcRoot, func(srcPath string, info os.FileInfo, err error) error {
		// TODO(jayconrod): skip files that don't affect package loading,
		// for example, large data files.
		if err != nil {
			return err
		}

		rel := ""
		if srcPath != srcRoot {
			rel = srcPath[len(srcRoot)+1:]
		}
		dstPath := filepath.Join(dstRoot, rel)

		if info.IsDir() {
			if srcPath == srcRoot {
				// we created dstRoot earlier
				return nil
			}
			// Don't attempt to skip testdata and hidden directories. They won't be
			// matched by wildcards, but they can still be imported explicitly.
			// Submodules and vendor directories will have already been filtered out
			// by module zip creation / extraction.
			return os.Mkdir(dstPath, 0777)
		}

		r, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer r.Close()
		w, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer w.Close()
		if _, err := io.Copy(w, r); err != nil {
			return err
		}
		return w.Close()
	})
}
