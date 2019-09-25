// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/exp/apidiff"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/tools/go/packages"
)

// report describes the differences in the public API between two versions
// of a module.
type report struct {
	// modulePath is the name of the module.
	modulePath string

	// baseVersion is the "old" version of the module to compare against.
	// It may be empty if there is no base version (for example, if this is
	// the first release).
	baseVersion string

	// releaseVersion is the version of the module to release, either
	// proposed with -version or inferred with suggestVersion.
	releaseVersion string

	// releaseVersionInferred is true if the release version was suggested
	// (not specified with -version).
	releaseVersionInferred bool

	// tagPrefix is the prefix for VCS tags for this module. For example,
	// if the module is defined in "foo/bar/v2/go.mod", tagPrefix will be
	// "foo/bar/".
	tagPrefix string

	// packages is a list of package reports, describing the differences
	// for individual packages, sorted by package path.
	packages []packageReport

	// diagnostics is a list of problems unrelated to the module API.
	// For example, if go.mod is missing some requirements, that will be
	// reported here.
	diagnostics []string

	// versionErr is set if the proposed release version is not valid, or if
	// no release version could be suggested.
	versionErr error

	// haveCompatibleChanges is true if there are any backward-compatible
	// changes in non-internal packages.
	haveCompatibleChanges bool

	// haveIncompatibleChanges is true if there are any backward-incompatible
	// changes in non-internal packages.
	haveIncompatibleChanges bool

	// haveBaseErrors is true if there were errors loading packages
	// in the base version.
	haveBaseErrors bool

	// haveReleaseErrors is true if there were errors loading packages
	// in the release version.
	haveReleaseErrors bool
}

// Text formats and writes a report to w. The report lists errors, compatible
// changes, and incompatible changes in each package. If releaseVersion is set,
// it states whether releaseVersion is valid (and why). If releaseVersion is not
// set, it suggests a new version.
func (r *report) Text(w io.Writer) error {
	buf := &bytes.Buffer{}
	for _, p := range r.packages {
		if err := p.Text(buf); err != nil {
			return err
		}
	}

	if len(r.diagnostics) > 0 {
		for _, d := range r.diagnostics {
			fmt.Fprintln(buf, d)
		}
	} else if r.versionErr != nil {
		fmt.Fprintln(buf, r.versionErr)
	} else if r.releaseVersionInferred {
		if r.tagPrefix == "" {
			fmt.Fprintf(buf, "Suggested version: %s\n", r.releaseVersion)
		} else {
			fmt.Fprintf(buf, "Suggested version: %[1]s (with tag %[2]s%[1]s)\n", r.releaseVersion, r.tagPrefix)
		}
	} else {
		if r.tagPrefix == "" {
			fmt.Fprintf(buf, "%s is a valid semantic version for this release.\n", r.releaseVersion)
		} else {
			fmt.Fprintf(buf, "%[1]s (with tag %[2]s%[1]s) is a valid semantic version for this release\n", r.releaseVersion, r.tagPrefix)
		}
	}

	if r.versionErr == nil && r.haveBaseErrors {
		fmt.Fprintln(buf, "Errors were found in the base version. Not all API changes may be reported.")
	}

	_, err := io.Copy(w, buf)
	return err
}

func (r *report) addPackage(p packageReport) {
	r.packages = append(r.packages, p)
	if len(p.baseErrors) == 0 && len(p.releaseErrors) == 0 {
		// Don't count compatible and incompatible changes if there were errors.
		// Definitions may be missing, and fixes may appear incompatible when
		// they are not. Changes will still be reported, but they won't affect
		// version validation or suggestions.
		for _, c := range p.Changes {
			if !c.Compatible && len(p.releaseErrors) == 0 {
				r.haveIncompatibleChanges = true
			} else if c.Compatible && len(p.baseErrors) == 0 && len(p.releaseErrors) == 0 {
				r.haveCompatibleChanges = true
			}
		}
	}
	if len(p.baseErrors) > 0 {
		r.haveBaseErrors = true
	}
	if len(p.releaseErrors) > 0 {
		r.haveReleaseErrors = true
	}
}

// validateVersion checks whether r.releaseVersion is valid.
// If r.releaseVersion is not valid, an error is returned explaining why.
// r.releaseVersion must be set.
func (r *report) validateVersion() (err error) {
	if r.releaseVersion == "" {
		panic("validateVersion called without version")
	}
	defer func() {
		if err != nil {
			err = &versionError{releaseVersion: r.releaseVersion, err: err}
		}
	}()
	if r.haveReleaseErrors {
		return errors.New("Errors were found in one or more packages.")
	}

	// TODO(jayconrod): link to documentation for all of these errors.

	// Check that the major version matches the module path.
	_, suffix, ok := module.SplitPathVersion(r.modulePath)
	if !ok {
		return fmt.Errorf("%s: could not find version suffix in module path", r.modulePath)
	}
	if suffix != "" {
		if suffix[0] != '/' && suffix[0] != '.' {
			return fmt.Errorf("%s: unknown module path version suffix: %q", r.modulePath, suffix)
		}
		pathMajor := suffix[1:]
		major := semver.Major(r.releaseVersion)
		if pathMajor != major {
			return fmt.Errorf(`The major version %s does not match the major version suffix
in the module path: %s`, major, r.modulePath)
		}
	} else if major := semver.Major(r.releaseVersion); major != "v0" && major != "v1" {
		return fmt.Errorf(`The module path does not end with the major version suffix /%s,
which is required for major versions v2 or greater.`, major)
	}

	// Check that compatible / incompatible changes are consistent.
	if semver.Major(r.baseVersion) == "v0" {
		return nil
	}
	if r.haveIncompatibleChanges {
		return errors.New("There are incompatible changes.")
	}
	if r.haveCompatibleChanges && semver.MajorMinor(r.baseVersion) == semver.MajorMinor(r.releaseVersion) {
		return fmt.Errorf(`There are compatible changes, but the major and minor version numbers
are the same as the base version %s.`, r.baseVersion)
	}

	return nil
}

// suggestVersion suggests a new version consistent with observed changes.
func (r *report) suggestVersion() (v string, err error) {
	defer func() {
		if err != nil {
			err = &versionError{err: err}
		}
	}()

	if r.haveReleaseErrors || r.haveBaseErrors {
		return "", errors.New("Errors were found.")
	}

	var major, minor, patch, pre string
	if r.baseVersion != "none" {
		major, minor, patch, pre, _, err = parseVersion(r.baseVersion)
		if err != nil {
			panic(fmt.Sprintf("could not parse base version: %v", err))
		}
	}

	if r.haveIncompatibleChanges && r.baseVersion != "none" && pre == "" && major != "0" {
		return "", errors.New("Incompatible changes were detected.")
		// TODO(jayconrod): briefly explain how to prepare major version releases
		// and link to documentation.
	}

	if r.baseVersion == "none" {
		if _, pathMajor, ok := module.SplitPathVersion(r.modulePath); !ok {
			panic(fmt.Sprintf("could not parse module path %q", r.modulePath))
		} else if pathMajor == "" {
			return "v0.1.0", nil
		} else {
			return pathMajor[1:] + ".0.0", nil
		}
	}

	if pre != "" {
		// suggest non-prerelease version
	} else if r.haveCompatibleChanges || (r.haveIncompatibleChanges && major == "0") {
		minor = incDecimal(minor)
		patch = "0"
	} else {
		patch = incDecimal(patch)
	}
	return fmt.Sprintf("v%s.%s.%s", major, minor, patch), nil
}

// isSuccessful returns true the module appears to be safe to release at the
// proposed or suggested version.
func (r *report) isSuccessful() bool {
	return len(r.diagnostics) == 0 && r.versionErr == nil
}

// incDecimal returns the decimal string incremented by 1.
func incDecimal(decimal string) string {
	// Scan right to left turning 9s to 0s until you find a digit to increment.
	digits := []byte(decimal)
	i := len(digits) - 1
	for ; i >= 0 && digits[i] == '9'; i-- {
		digits[i] = '0'
	}
	if i >= 0 {
		digits[i]++
	} else {
		// digits is all zeros
		digits[0] = '1'
		digits = append(digits, '0')
	}
	return string(digits)
}

type packageReport struct {
	apidiff.Report
	path                      string
	baseErrors, releaseErrors []packages.Error
}

func (p *packageReport) Text(w io.Writer) error {
	if len(p.Changes) == 0 && len(p.baseErrors) == 0 && len(p.releaseErrors) == 0 {
		return nil
	}
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "%s\n%s\n", p.path, strings.Repeat("-", len(p.path)))
	if len(p.baseErrors) > 0 {
		fmt.Fprintf(buf, "errors in base version:\n")
		for _, e := range p.baseErrors {
			fmt.Fprintf(buf, "\t%v\n", e)
		}
		buf.WriteByte('\n')
	}
	if len(p.releaseErrors) > 0 {
		fmt.Fprintf(buf, "errors in release version:\n")
		for _, e := range p.releaseErrors {
			fmt.Fprintf(buf, "\t%v\n", e)
		}
		buf.WriteByte('\n')
	}
	if len(p.Changes) > 0 {
		if err := p.Report.Text(buf); err != nil {
			return err
		}
		buf.WriteByte('\n')
	}
	_, err := io.Copy(w, buf)
	return err
}

// parseVersion returns the major, minor, and patch numbers, prerelease text,
// and metadata for a given version.
//
// TODO(jayconrod): extend semver to do this and delete this function.
func parseVersion(vers string) (major, minor, patch, pre, meta string, err error) {
	if !strings.HasPrefix(vers, "v") {
		return "", "", "", "", "", fmt.Errorf("version %q does not start with 'v'", vers)
	}
	base := vers[1:]
	if i := strings.IndexByte(base, '+'); i >= 0 {
		meta = base[i+1:]
		base = base[:i]
	}
	if i := strings.IndexByte(base, '-'); i >= 0 {
		pre = base[i+1:]
		base = base[:i]
	}
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return "", "", "", "", "", fmt.Errorf("version %q should have three numbers", vers)
	}
	major, minor, patch = parts[0], parts[1], parts[2]
	return major, minor, patch, pre, meta, nil
}
