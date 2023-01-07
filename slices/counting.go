// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

// solve x * log(x) = x*2 + 2**8; this is a very rough estimation
const countingThreshold256 = 99

// countingSort256 is a non comparative sort that runs in O(N) time implemented
// for 8 bits wide integer type.
// If the array is not sorted it will perform n * 2 + 256 operations, if it is
// sorted it will perform n operations.
// It returns true if the array was already sorted.
func countingSort256[E ~uint8 | ~int8](x []E) {
	const permutations = 1 << 8
	var counts [permutations]uint

	// You might be tempted to to add a comparision with a fast path in order to
	// skip the array rebuild if the array is already sorted.
	// Turns out, on my machine (Ryzen 3600, 3200Mhz ram) rebuilding the array
	// is 1.6x faster than the extra comparisions we have for the already sorted
	// fast path.
	for _, v := range x {
		counts[uint8(v)]++
	}

	// folded by the compiler
	var signTest E
	signTest--
	signed := signTest < 0

	// rebuild the output
	var i, v uint
	var max uint = permutations
	if signed {
		// if we deal with a signed number we need to rebuild the slice
		// discontinully because negative number starts in the middle of the range
		v = permutations / 2
	}
rebuild:
	for ; v < uint(len(counts[:max])); v++ {
		count := counts[v]
		// We do not need any BCE here because there is no way the sum of elements
		// is bigger than the length of the slice, however theaching the compiler
		// this is too much work, so let's settle for 256 BCE instead.
		newI := i + count
		if uint(len(x)) < newI {
			panic("unreachable")
		}
		for ; i < newI; i++ {
			x[i] = E(v)
		}
	}
	if signed && max == permutations {
		max = permutations / 2
		v = 0
		goto rebuild
	}

	// done!
}

// solve x * log(x) = x*2 + 2**16; this is a very rough estimation
const countingThreshold65536 = 9196

// countingSort65536 is a non comparative sort that runs in O(N) time
// implemented for 16 bits wide integer type.
// If the array is not sorted it will perform n * 2 + 65536 operations, if it is
// sorted it will perform n operations.
// It returns true if the array is already sorted.
func countingSort65536[E ~uint16 | ~int16](x []E) {
	const permutations = 1 << 16
	var counts [permutations]uint

	// You might be tempted to to add a comparision with a fast path in order to
	// skip the array rebuild if the array is already sorted.
	// Turns out, on my machine (Ryzen 3600, 3200Mhz ram) rebuilding the array
	// is 1.6x faster than the extra comparisions we have for the already sorted
	// fast path.
	for _, v := range x {
		counts[uint16(v)]++
	}

	// test signess, folded by the compiler
	var signTest E
	signTest--
	signed := signTest < 0

	// rebuild the output
	var i, v uint
	var max uint = permutations
	if signed {
		// if we deal with a signed number we need to rebuild the slice
		// discontinully because negative number starts in the middle of the range
		v = permutations / 2
	}
rebuild:
	for ; v < uint(len(counts[:max])); v++ {
		count := counts[v]
		// We do not need any BCE here because there is no way the sum of element
		// counts is bigger than the length of the slice, however theaching the
		// compiler this is too much work, so let's settle for 65536 BCE instead.
		newI := i + count
		if uint(len(x)) < newI {
			panic("unreachable")
		}
		for ; i < newI; i++ {
			x[i] = E(v)
		}
	}
	if signed && max == permutations {
		max = permutations / 2
		v = 0
		goto rebuild
	}

	// done!
}
