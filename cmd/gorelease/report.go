// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"
	"text/template"

	"golang.org/x/exp/apidiff"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/tools/go/packages"
)

const defaultFormat = `
{{- range .Packages}}{{.}}{{end -}}
{{- if .Release.Diagnostics -}}
# diagnostics
{{range .Release.Diagnostics}}{{.}}
{{end}}
{{end -}}
{{- if canVerifyReleaseVersion . -}}
# summary
{{$base := .Base.Version -}}
{{- if ne .Base.ModPath .Release.ModPath -}}
{{- $base = printf "%s@%s" .Base.ModPath .Base.Version -}}
{{- end -}}
{{- if .Base.VersionInferred -}}
Inferred base version: {{$base}}
{{else if .Base.VersionQuery -}}
Base version: {{$base}} ({{.Base.VersionQuery}})
{{end -}}
{{- if .InvalidReleaseVersion -}}
{{.InvalidReleaseVersion}}
{{else if .Release.VersionInferred -}}
Suggested version: {{.Release.Version}}
{{- if .Release.TagPrefix}} (with tag {{.Release.TagPrefix}}{{.Release.Version}}){{end}}
{{else if .Release.Version -}}
{{.Release.Version}} {{if .Release.TagPrefix}}(with tag {{.Release.TagPrefix}}{{.Release.Version}}) {{end}}is a valid semantic version for this release.
{{if lowerThanPseudo .Release.Version -}}
Note: {{.Release.Version}} sorts lower in MVS than pseudo-versions, which may be
unexpected for users. So, it may be better to choose a different suffix.
{{end -}}
{{- end -}}
{{- if and (not .InvalidReleaseVersion) (haveBaseErrors .) -}}
Errors were found in the base version. Some API changes may be omitted.
{{ end -}}
{{- end -}}
`

var defaultFuncs = template.FuncMap{
	"canVerifyReleaseVersion": func(r Report) bool {
		return r.canVerifyReleaseVersion()
	},
	"haveBaseErrors": func(r Report) bool {
		return r.haveBaseErrors
	},
	"lowerThanPseudo": func(v string) bool {
		return semver.Compare(v, "v0.0.0-99999999999999-zzzzzzzzzzzz") < 0
	},
}

// Report describes the differences in the public API between two versions
// of a module.
type Report struct {
	// Base contains information about the "old" module version being compared
	// against. Base.version may be "none", indicating there is no Base version
	// (for example, if this is the first release). Base.version may not be "".
	Base VersionInfo

	// Release contains information about the version of the module to Release.
	// The version may be set explicitly with -version or suggested using
	// suggestVersion, in which case Release.versionInferred is true.
	Release VersionInfo

	// Packages is a list of package reports, describing the differences
	// for individual Packages, sorted by package path.
	Packages []PackageReport

	// InvalidReleaseVersion explains why the proposed or suggested version is not valid.
	InvalidReleaseVersion string

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

// String returns a human-readable report that lists errors, compatible changes,
// and incompatible changes in each package. If releaseVersion is set, the
// report states whether releaseVersion is valid (and why). If releaseVersion is
// not set, it suggests a new version.
func (r *Report) String() string {
	buf := &strings.Builder{}
	for _, p := range r.Packages {
		buf.WriteString(p.String())
	}

	if !r.canVerifyReleaseVersion() {
		return buf.String()
	}

	if len(r.Release.Diagnostics) > 0 {
		buf.WriteString("# diagnostics\n")
		for _, d := range r.Release.Diagnostics {
			fmt.Fprintln(buf, d)
		}
		buf.WriteByte('\n')
	}

	buf.WriteString("# summary\n")
	baseVersion := r.Base.Version
	if r.Base.ModPath != r.Release.ModPath {
		baseVersion = r.Base.ModPath + "@" + baseVersion
	}
	if r.Base.VersionInferred {
		fmt.Fprintf(buf, "Inferred base version: %s\n", baseVersion)
	} else if r.Base.VersionQuery != "" {
		fmt.Fprintf(buf, "Base version: %s (%s)\n", baseVersion, r.Base.VersionQuery)
	}

	if r.InvalidReleaseVersion != "" {
		fmt.Fprintln(buf, r.InvalidReleaseVersion)
	} else if r.Release.VersionInferred {
		if r.Release.TagPrefix == "" {
			fmt.Fprintf(buf, "Suggested version: %s\n", r.Release.Version)
		} else {
			fmt.Fprintf(buf, "Suggested version: %[1]s (with tag %[2]s%[1]s)\n", r.Release.Version, r.Release.TagPrefix)
		}
	} else if r.Release.Version != "" {
		if r.Release.TagPrefix == "" {
			fmt.Fprintf(buf, "%s is a valid semantic version for this release.\n", r.Release.Version)

			if semver.Compare(r.Release.Version, "v0.0.0-99999999999999-zzzzzzzzzzzz") < 0 {
				fmt.Fprintf(buf, `Note: %s sorts lower in MVS than pseudo-versions, which may be
unexpected for users. So, it may be better to choose a different suffix.`, r.Release.Version)
			}
		} else {
			fmt.Fprintf(buf, "%[1]s (with tag %[2]s%[1]s) is a valid semantic version for this release\n", r.Release.Version, r.Release.TagPrefix)
		}
	}

	if r.InvalidReleaseVersion == "" && r.haveBaseErrors {
		fmt.Fprintln(buf, "Errors were found in the base version. Some API changes may be omitted.")
	}

	return buf.String()
}

func (r *Report) addPackage(p PackageReport) {
	r.Packages = append(r.Packages, p)
	if len(p.baseErrors) == 0 && len(p.releaseErrors) == 0 {
		// Only count compatible and incompatible changes if there were no errors.
		// When there are errors, definitions may be missing, and fixes may appear
		// incompatible when they are not. Changes will still be reported, but
		// they won't affect version validation or suggestions.
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

// validateReleaseVersion checks whether r.release.version is valid.
// If r.release.version is not valid, an error is returned explaining why.
// r.release.version must be set.
func (r *Report) validateReleaseVersion() {
	if r.Release.Version == "" {
		panic("validateVersion called without version")
	}
	setNotValid := func(format string, args ...interface{}) {
		prefix := fmt.Sprintf("%s is not a valid semantic version for this release.\n", r.Release.Version)
		r.InvalidReleaseVersion = prefix + fmt.Sprintf(format, args...)
	}

	if r.haveReleaseErrors {
		if r.haveReleaseErrors {
			setNotValid("Errors were found in one or more packages.")
			return
		}
	}

	// TODO(jayconrod): link to documentation for all of these errors.

	// Check that the major version matches the module path.
	_, suffix, ok := module.SplitPathVersion(r.Release.ModPath)
	if !ok {
		setNotValid("%s: could not find version suffix in module path", r.Release.ModPath)
		return
	}
	if suffix != "" {
		if suffix[0] != '/' && suffix[0] != '.' {
			setNotValid("%s: unknown module path version suffix: %q", r.Release.ModPath, suffix)
			return
		}
		pathMajor := suffix[1:]
		major := semver.Major(r.Release.Version)
		if pathMajor != major {
			setNotValid(`The major version %s does not match the major version suffix
in the module path: %s`, major, r.Release.ModPath)
			return
		}
	} else if major := semver.Major(r.Release.Version); major != "v0" && major != "v1" {
		setNotValid(`The module path does not end with the major version suffix /%s,
which is required for major versions v2 or greater.`, major)
		return
	}

	for _, v := range r.Base.existingVersions {
		if semver.Compare(v, r.Release.Version) == 0 {
			setNotValid("version %s already exists", v)
		}
	}

	// Check that compatible / incompatible changes are consistent.
	if semver.Major(r.Base.Version) == "v0" || r.Base.ModPath != r.Release.ModPath {
		return
	}
	if r.haveIncompatibleChanges {
		setNotValid("There are incompatible changes.")
		return
	}
	if r.haveCompatibleChanges && semver.MajorMinor(r.Base.Version) == semver.MajorMinor(r.Release.Version) {
		setNotValid(`There are compatible changes, but the minor version is not incremented
over the base version (%s).`, r.Base.Version)
		return
	}

	if r.Release.highestTransitiveVersion != "" && semver.Compare(r.Release.highestTransitiveVersion, r.Release.Version) > 0 {
		setNotValid(`Module indirectly depends on a higher version of itself (%s).
		`, r.Release.highestTransitiveVersion)
	}
}

// suggestReleaseVersion suggests a new version consistent with observed
// changes.
func (r *Report) suggestReleaseVersion() {
	setNotValid := func(format string, args ...interface{}) {
		r.InvalidReleaseVersion = "Cannot suggest a release version.\n" + fmt.Sprintf(format, args...)
	}
	setVersion := func(v string) {
		r.Release.Version = v
		r.Release.VersionInferred = true
	}

	if r.Base.ModPath != r.Release.ModPath {
		setNotValid("Base module path is different from release.")
		return
	}

	if r.haveReleaseErrors || r.haveBaseErrors {
		setNotValid("Errors were found.")
		return
	}

	var major, minor, patch, pre string
	if r.Base.Version != "none" {
		minVersion := r.Base.Version
		if r.Release.highestTransitiveVersion != "" && semver.Compare(r.Release.highestTransitiveVersion, minVersion) > 0 {
			setNotValid("Module indirectly depends on a higher version of itself (%s) than the base version (%s).", r.Release.highestTransitiveVersion, r.Base.Version)
			return
		}

		var err error
		major, minor, patch, pre, _, err = parseVersion(minVersion)
		if err != nil {
			panic(fmt.Sprintf("could not parse base version: %v", err))
		}
	}

	if r.haveIncompatibleChanges && r.Base.Version != "none" && pre == "" && major != "0" {
		setNotValid("Incompatible changes were detected.")
		return
		// TODO(jayconrod): briefly explain how to prepare major version releases
		// and link to documentation.
	}

	// Check whether we're comparing to the latest version of base.
	//
	// This could happen further up, but we want the more pressing errors above
	// to take precedence.
	var latestForBaseMajor string
	for _, v := range r.Base.existingVersions {
		if semver.Major(v) != semver.Major(r.Base.Version) {
			continue
		}
		if latestForBaseMajor == "" || semver.Compare(latestForBaseMajor, v) < 0 {
			latestForBaseMajor = v
		}
	}
	if latestForBaseMajor != "" && latestForBaseMajor != r.Base.Version {
		setNotValid(fmt.Sprintf("Can only suggest a release version when compared against the most recent version of this major: %s.", latestForBaseMajor))
		return
	}

	if r.Base.Version == "none" {
		if _, pathMajor, ok := module.SplitPathVersion(r.Release.ModPath); !ok {
			panic(fmt.Sprintf("could not parse module path %q", r.Release.ModPath))
		} else if pathMajor == "" {
			setVersion("v0.1.0")
		} else {
			setVersion(pathMajor[1:] + ".0.0")
		}
		return
	}

	if pre != "" {
		// suggest non-prerelease version
	} else if r.haveCompatibleChanges || (r.haveIncompatibleChanges && major == "0") || r.requirementsChanged() {
		minor = incDecimal(minor)
		patch = "0"
	} else {
		patch = incDecimal(patch)
	}
	setVersion(fmt.Sprintf("v%s.%s.%s", major, minor, patch))
	return
}

// canVerifyReleaseVersion returns true if we can safely suggest a new version
// or if we can verify the version passed in with -version is safe to tag.
func (r *Report) canVerifyReleaseVersion() bool {
	// For now, return true if the base and release module paths are the same,
	// ignoring the major version suffix.
	// TODO(#37562, #39192, #39666, #40267): there are many more situations when
	// we can't verify a new version.
	basePath := strings.TrimSuffix(r.Base.ModPath, r.Base.modPathMajor)
	releasePath := strings.TrimSuffix(r.Release.ModPath, r.Release.modPathMajor)
	return basePath == releasePath
}

// requirementsChanged reports whether requirements have changed from base to
// version.
//
// requirementsChanged reports true for,
//   - A requirement was upgraded to a higher minor version.
//   - A requirement was added.
//   - The version of Go was incremented.
//
// It does not report true when, for example, a requirement was downgraded or
// remove. We care more about the former since that might force dependent
// modules that have the same dependency to upgrade.
func (r *Report) requirementsChanged() bool {
	if r.Base.goModFile == nil {
		// There wasn't a modfile before, and now there is.
		return true
	}

	// baseReqs is a map of module path to MajorMinor of the base module
	// requirements.
	baseReqs := make(map[string]string)
	for _, r := range r.Base.goModFile.Require {
		baseReqs[r.Mod.Path] = r.Mod.Version
	}

	for _, r := range r.Release.goModFile.Require {
		if _, ok := baseReqs[r.Mod.Path]; !ok {
			// A module@version was added to the "require" block between base
			// and release.
			return true
		}
		if semver.Compare(semver.MajorMinor(r.Mod.Version), semver.MajorMinor(baseReqs[r.Mod.Path])) > 0 {
			// The version of r.Mod.Path increased from base to release.
			return true
		}
	}

	if r.Release.goModFile.Go != nil && r.Base.goModFile.Go != nil {
		if r.Release.goModFile.Go.Version > r.Base.goModFile.Go.Version {
			// The Go version increased from base to release.
			return true
		}
	}

	return false
}

// isSuccessful returns true the module appears to be safe to release at the
// proposed or suggested version.
func (r *Report) isSuccessful() bool {
	return len(r.Release.Diagnostics) == 0 && r.InvalidReleaseVersion == ""
}

type versionMessage struct {
	message, reason string
}

func (m versionMessage) String() string {
	return m.message + "\n" + m.reason + "\n"
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

type PackageReport struct {
	apidiff.Report
	path                      string
	baseErrors, releaseErrors []packages.Error
}

func (p *PackageReport) String() string {
	if len(p.Changes) == 0 && len(p.baseErrors) == 0 && len(p.releaseErrors) == 0 {
		return ""
	}
	buf := &strings.Builder{}
	fmt.Fprintf(buf, "# %s\n", p.path)
	if len(p.baseErrors) > 0 {
		fmt.Fprintf(buf, "## errors in base version:\n")
		for _, e := range p.baseErrors {
			fmt.Fprintln(buf, e)
		}
		buf.WriteByte('\n')
	}
	if len(p.releaseErrors) > 0 {
		fmt.Fprintf(buf, "## errors in release version:\n")
		for _, e := range p.releaseErrors {
			fmt.Fprintln(buf, e)
		}
		buf.WriteByte('\n')
	}
	if len(p.Changes) > 0 {
		var compatible, incompatible []apidiff.Change
		for _, c := range p.Changes {
			if c.Compatible {
				compatible = append(compatible, c)
			} else {
				incompatible = append(incompatible, c)
			}
		}
		if len(incompatible) > 0 {
			fmt.Fprintf(buf, "## incompatible changes\n")
			for _, c := range incompatible {
				fmt.Fprintln(buf, c.Message)
			}
		}
		if len(compatible) > 0 {
			fmt.Fprintf(buf, "## compatible changes\n")
			for _, c := range compatible {
				fmt.Fprintln(buf, c.Message)
			}
		}
		buf.WriteByte('\n')
	}
	return buf.String()
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
