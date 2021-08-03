// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audit

import (
	"fmt"
	"strings"
)

// FindingCompare compares two findings in terms of their estimated value to the user.
// Findings of shorter traces generally come earlier in the ordering.
//
// Two findings produced by audit call graph search are lexicographically ordered by:
// 1) their estimated level of confidence in being a true positive, 2) the length of
// their traces, and 3) the number of unresolved call sites in the traces.
func FindingCompare(finding1, finding2 Finding) bool {
	if finding1.confidence < finding2.confidence {
		return true
	} else if finding2.confidence < finding1.confidence {
		return false
	}
	if len(finding1.Trace) < len(finding2.Trace) {
		return true
	} else if len(finding2.Trace) < len(finding1.Trace) {
		return false
	}
	if finding1.weight < finding2.weight {
		return true
	} else if finding2.weight < finding1.weight {
		return false
	}

	// At this point we just need to make sure the ordering is deterministic.
	// TODO(zpavlinovic): is there a more meaningful ordering?
	return findingStrCompare(finding1, finding2)
}

// findingStrCompare compares string representation of findings pointwise by their fields.
func findingStrCompare(finding1, finding2 Finding) bool {
	symCmp := strings.Compare(finding1.Symbol, finding2.Symbol)
	if symCmp == -1 {
		return true
	} else if symCmp == 1 {
		return false
	}

	typeCmp := strings.Compare(fmt.Sprintf("%v", finding1.Type), fmt.Sprintf("%v", finding2.Type))
	if typeCmp == -1 {
		return true
	} else if typeCmp == 1 {
		return false
	}

	posCmp := strings.Compare(fmt.Sprintf("%v", finding1.Position), fmt.Sprintf("%v", finding2.Position))
	if posCmp == -1 {
		return true
	} else if posCmp == 1 {
		return false
	}

	traceCmp := strings.Compare(fmt.Sprintf("%v", finding1.Trace), fmt.Sprintf("%v", finding2.Trace))
	if traceCmp == 1 {
		return false
	}
	return true
}
