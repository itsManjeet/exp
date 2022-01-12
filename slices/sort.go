// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

import "constraints"

//func Sort[Elem constraints.Ordered](x []Elem)
//func SortFunc[Elem any](x []Elem, less func(a, b Elem) bool)
//func SortStable[Elem constraints.Ordered](x []Elem)
//func SortStableFunc[Elem any](x []Elem, less func(a, b Elem) bool)
//func IsSorted[Elem constraints.Ordered](x []Elem)
//func IsSortedFunc[Elem any](x []Elem, less func(a, b Elem) bool)
//func BinarySearch[Elem constraints.Ordered](x Slice, target Elem) int
//func BinarySearchFunc[Elem any](x []Elem, ok func(Elem) bool) int

// Sort sorts the slice x. The sort is not guaranteed to be stable.
func Sort[Elem constraints.Ordered](x []Elem) {
	n := len(x)
	quickSort_ordered(x, 0, n, maxDepth(n))
}

// SortStable sorts the slice x while keeping the original order of equal
// elements. It was added to this package for consistency.
func SortStable[Elem constraints.Ordered](x []Elem) {
	stable_ordered(x, len(x))
}

// IsSorted reports whether x is sorted.
func IsSorted[Elem constraints.Ordered](x []Elem) bool {
	for i := len(x) - 1; i > 0; i-- {
		if x[i] < x[i-1] {
			return false
		}
	}
	return true
}

// IsSortedFunc reports whether x is sorted, with less as the comparison
// function.
func IsSortedFunc[Elem any](x []Elem, less func(a, b Elem) bool) bool {
	for i := len(x) - 1; i > 0; i-- {
		if less(x[i], x[i-1]) {
			return false
		}
	}
	return true
}

// BinarySearch searches for target in a sorted slice and returns the smallest
// index at which target is found. If there is no such index, returns len(x).
func BinarySearch[Elem constraints.Ordered](x []Elem, target Elem) int {
	return search(len(x), func(i int) bool { return x[i] >= target })
}

// BinarySearchFunc searches in a sorted slice and returns the samllest index
// at which ok(x[i]) is true. If there is no such index, returns len(x).
func BinarySearchFunc[Elem any](x []Elem, ok func(Elem) bool) int {
	return search(len(x), func(i int) bool { return ok(x[i]) })
}

// maxDepth returns a threshold at which quicksort should switch
// to heapsort. It returns 2*ceil(lg(n+1)).
func maxDepth(n int) int {
	var depth int
	for i := n; i > 0; i >>= 1 {
		depth++
	}
	return depth * 2
}

func search(n int, f func(int) bool) int {
	// Define f(-1) == false and f(n) == true.
	// Invariant: f(i-1) == false, f(j) == true.
	i, j := 0, n
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		// i ≤ h < j
		if !f(h) {
			i = h + 1 // preserves f(i-1) == false
		} else {
			j = h // preserves f(j) == true
		}
	}
	// i == j, f(i-1) == false, and f(j) (= f(i)) == true  =>  answer is i.
	return i
}
