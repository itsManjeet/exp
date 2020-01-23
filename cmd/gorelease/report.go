// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
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
	// It may be "none" if there is no base version (for example, if this is
	// the first release). It may not be "".
	baseVersion string

	// baseVersionInferred is true if the base version was determined
	// automatically (not specified with -base).
	baseVersionInferred bool

	// releaseVersion is the proposed version of the module. It may be empty
	// if no version was proposed.
	releaseVersion string

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

	// haveCompatibleChanges is true if there are any backward-compatible
	// changes in non-internal packages.
	haveCompatibleChanges bool

	// haveIncompatibleChanges is true if there are any backward-incompatible
	// changes in non-internal packages.
	haveIncompatibleChanges bool

	// haveErrors is true if there were any errors loading packages in
	// the release version.
	haveErrors bool
}

// Text formats and writes a report to w. The report lists errors, compatible
// changes, and incompatible changes in each package. If releaseVersion is set,
// it states whether releaseVersion is valid (and why). If releaseVersion is not
// set, it suggests a new version.
func (r *report) Text(w io.Writer) error {
	for _, p := range r.packages {
		if err := p.Text(w); err != nil {
			return err
		}
	}

	if r.baseVersionInferred {
		if _, err := fmt.Fprintf(w, "Inferred base version: %s\n", r.baseVersion); err != nil {
			return err
		}
	}

	var summary string
	if len(r.diagnostics) > 0 {
		summary = strings.Join(r.diagnostics, "\n")
	} else if r.releaseVersion != "" {
		if err := r.validateVersion(); err != nil {
			summary = err.Error()
		} else {
			if r.tagPrefix == "" {
				summary = fmt.Sprintf("%s is a valid semantic version for this release.", r.releaseVersion)
			} else {
				summary = fmt.Sprintf("%[1]s (with tag %[2]s%[1]s) is a valid semantic version for this release", r.releaseVersion, r.tagPrefix)
			}
		}
	} else if r.haveErrors {
		summary = "Errors were detected, so no version will be suggested."
	} else if r.haveIncompatibleChanges && r.baseVersion != "" && semver.Major(r.baseVersion) != "v0" {
		suggestedVersion := r.suggestVersion()
		summary = fmt.Sprintf(`Incompatible changes detected, so no version will be suggested.
Use -version=%s to verify a new major version.
Avoid creating new major versions if possible though.
`, suggestedVersion)
		// TODO(jayconrod): link to documentation on releasing major versions
	} else {
		suggestedVersion := r.suggestVersion()
		if r.tagPrefix == "" {
			summary = fmt.Sprintf("Suggested version: %s", suggestedVersion)
		} else {
			summary = fmt.Sprintf("Suggested version: %[1]s (with tag %[2]s%[1]s)", suggestedVersion, r.tagPrefix)
		}
	}
	if _, err := fmt.Fprintln(w, summary); err != nil {
		return err
	}

	return nil
}

func (r *report) addPackage(p packageReport) {
	r.packages = append(r.packages, p)
	for _, c := range p.Changes {
		if c.Compatible {
			r.haveCompatibleChanges = true
		} else {
			r.haveIncompatibleChanges = true
		}
	}
	if len(p.newErrors) > 0 {
		r.haveErrors = true
	}
}

// validateVersion checks whether r.releaseVersion is valid.
// If r.releaseVersion is not valid, an error is returned explaining why.
// r.releaseVersion must be set.
func (r *report) validateVersion() error {
	if r.releaseVersion == "" {
		panic("validateVersion called without version")
	}
	if r.haveErrors {
		return fmt.Errorf(`%s is not a valid semantic version for this release.
Errors were found in one or more packages.`, r.releaseVersion)
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
			return fmt.Errorf(`%s is not a valid semantic version for this release.
The major version %s does not match the major version suffix
in the module path: %s`, r.releaseVersion, r.modulePath, major)
		}
	} else if major := semver.Major(r.releaseVersion); major != "v0" && major != "v1" {
		return fmt.Errorf(`%s is not a valid semantic version for this release.
The module path does not end with the major version suffix /%s,
which is required for major versions v2 or greater.`, r.releaseVersion, major)
	}

	// Check that compatible / incompatible changes are consistent.
	if semver.Major(r.baseVersion) == "v0" {
		return nil
	}
	if r.haveIncompatibleChanges {
		return fmt.Errorf(`%s is not a valid semantic version for this release.
There are incompatible changes.`, r.releaseVersion)
	}
	if r.haveCompatibleChanges && semver.MajorMinor(r.baseVersion) == semver.MajorMinor(r.releaseVersion) {
		return fmt.Errorf(`%s is not a valid semantic version for this release.
There are compatible changes, but the major and minor version numbers
are the same as the base version %s.`, r.releaseVersion, r.baseVersion)
	}

	return nil
}

// suggestVersion suggests a new version consistent with observed changes.
func (r *report) suggestVersion() string {
	if r.baseVersion == "none" {
		if _, pathMajor, ok := module.SplitPathVersion(r.modulePath); !ok {
			panic(fmt.Sprintf("could not parse module path %q", r.modulePath))
		} else if pathMajor == "" {
			return "v0.0.1"
		} else {
			return pathMajor[1:] + ".0.0"
		}
	}

	major, minor, patch, err := splitVersionNumbers(r.baseVersion)
	if err != nil {
		panic(fmt.Sprintf("could not parse base version: %v", err))
	}

	if r.haveIncompatibleChanges && major != "0" {
		major = incDecimal(major)
		minor = "0"
		patch = "0"
	} else if r.haveCompatibleChanges || (r.haveIncompatibleChanges && major == "0") {
		minor = incDecimal(minor)
		patch = "0"
	} else {
		patch = incDecimal(patch)
	}
	return fmt.Sprintf("v%s.%s.%s", major, minor, patch)
}

// isSuccessful returns whether observed changes are consistent with
// r.releaseVersion. If r.releaseVersion is set, isSuccessful tests whether
// r.validateVersion() returns an error. If r.releaseVersion is not set,
// isSuccessful returns true if there were no incompatible changes or if
// a new version could be released without changing the module path.
func (r *report) isSuccessful() bool {
	if r.haveErrors || len(r.diagnostics) > 0 {
		return false
	}
	if r.releaseVersion != "" {
		return r.validateVersion() == nil
	}
	return !r.haveIncompatibleChanges || semver.Major(r.baseVersion) == "v0"
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

// decDecimal returns the decimal string decremented by 1, or the empty string
// if the decimal is all zeroes.
func decDecimal(decimal string) string {
	// Scan right to left turning 0s to 9s until you find a digit to decrement.
	digits := []byte(decimal)
	i := len(digits) - 1
	for ; i >= 0 && digits[i] == '0'; i-- {
		digits[i] = '9'
	}
	if i < 0 {
		// decimal is all zeros
		return ""
	}
	if i == 0 && digits[i] == '1' && len(digits) > 1 {
		digits = digits[1:]
	} else {
		digits[i]--
	}
	return string(digits)
}

type packageReport struct {
	apidiff.Report
	path                 string
	oldErrors, newErrors []packages.Error
}

func (p *packageReport) Text(w io.Writer) error {
	if len(p.Changes) == 0 && len(p.oldErrors) == 0 && len(p.newErrors) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "%s\n%s\n", p.path, strings.Repeat("-", len(p.path))); err != nil {
		return err
	}
	if len(p.oldErrors) > 0 {
		if _, err := fmt.Fprintf(w, "errors in old version:\n"); err != nil {
			return err
		}
		for _, e := range p.oldErrors {
			if _, err := fmt.Fprintf(w, "\t%v\n", e); err != nil {
				return err
			}
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
	}
	if len(p.newErrors) > 0 {
		if _, err := fmt.Fprintf(w, "errors in new version:\n"); err != nil {
			return err
		}
		for _, e := range p.newErrors {
			if _, err := fmt.Fprintf(w, "\t%v\n", e); err != nil {
				return err
			}
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
	}
	if len(p.Changes) > 0 {
		if err := p.Report.Text(w); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
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
