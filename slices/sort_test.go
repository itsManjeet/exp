// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var ints = [...]int{74, 59, 238, -784, 9845, 959, 905, 0, 0, 42, 7586, -5467984, 7586}
var float64s = [...]float64{74.3, 59.0, math.Inf(1), 238.2, -784.0, 2.3, math.Inf(-1), 9845.768, -959.7485, 905, 7.8, 7.8, 74.3, 59.0, math.Inf(1), 238.2, -784.0, 2.3}
var float64sWithNaNs = [...]float64{74.3, 59.0, math.Inf(1), 238.2, -784.0, 2.3, math.NaN(), math.NaN(), math.Inf(-1), 9845.768, -959.7485, 905, 7.8, 7.8}
var strs = [...]string{"", "Hello", "foo", "bar", "foo", "f00", "%*&^*&^&", "***"}

func TestSortIntSlice(t *testing.T) {
	data := Clone(ints[:])
	Sort(data)
	if !IsSorted(data) {
		t.Errorf("sorted %v", ints)
		t.Errorf("   got %v", data)
	}
}

func TestSortFuncIntSlice(t *testing.T) {
	data := Clone(ints[:])
	SortFunc(data, func(a, b int) bool { return a < b })
	if !IsSorted(data) {
		t.Errorf("sorted %v", ints)
		t.Errorf("   got %v", data)
	}
}

func TestSortFloat64Slice(t *testing.T) {
	data := Clone(float64s[:])
	Sort(data)
	if !IsSorted(data) {
		t.Errorf("sorted %v", float64s)
		t.Errorf("   got %v", data)
	}
}

func TestSortFloat64SliceWithNaNs(t *testing.T) {
	data := float64sWithNaNs[:]
	input := Clone(data)

	// Make sure Sort doesn't panic when the slice contains NaNs.
	Sort(data)
	// Check whether the result is a permutation of the input.
	sort.Float64s(data)
	sort.Float64s(input)
	for i, v := range input {
		if data[i] != v && !(math.IsNaN(data[i]) && math.IsNaN(v)) {
			t.Fatalf("the result is not a permutation of the input\ngot %v\nwant %v", data, input)
		}
	}
}

func TestSortStringSlice(t *testing.T) {
	data := Clone(strs[:])
	Sort(data)
	if !IsSorted(data) {
		t.Errorf("sorted %v", strs)
		t.Errorf("   got %v", data)
	}
}

func TestSortLarge_Random(t *testing.T) {
	n := 1000000
	if testing.Short() {
		n /= 100
	}
	data := make([]int, n)
	for i := 0; i < len(data); i++ {
		data[i] = rand.Intn(100)
	}
	if IsSorted(data) {
		t.Fatalf("terrible rand.rand")
	}
	Sort(data)
	if !IsSorted(data) {
		t.Errorf("sort didn't sort - 1M ints")
	}
}

type intPair struct {
	a, b int
}

type intPairs []intPair

// Pairs compare on a only.
func intPairLess(x, y intPair) bool {
	return x.a < y.a
}

// Record initial order in B.
func (d intPairs) initB() {
	for i := range d {
		d[i].b = i
	}
}

// InOrder checks if a-equal elements were not reordered.
func (d intPairs) inOrder() bool {
	lastA, lastB := -1, 0
	for i := 0; i < len(d); i++ {
		if lastA != d[i].a {
			lastA = d[i].a
			lastB = d[i].b
			continue
		}
		if d[i].b <= lastB {
			return false
		}
		lastB = d[i].b
	}
	return true
}

func TestStability(t *testing.T) {
	n, m := 100000, 1000
	if testing.Short() {
		n, m = 1000, 100
	}
	data := make(intPairs, n)

	// random distribution
	for i := 0; i < len(data); i++ {
		data[i].a = rand.Intn(m)
	}
	if IsSortedFunc(data, intPairLess) {
		t.Fatalf("terrible rand.rand")
	}
	data.initB()
	SortStableFunc(data, intPairLess)
	if !IsSortedFunc(data, intPairLess) {
		t.Errorf("Stable didn't sort %d ints", n)
	}
	if !data.inOrder() {
		t.Errorf("Stable wasn't stable on %d ints", n)
	}

	// already sorted
	data.initB()
	SortStableFunc(data, intPairLess)
	if !IsSortedFunc(data, intPairLess) {
		t.Errorf("Stable shuffled sorted %d ints (order)", n)
	}
	if !data.inOrder() {
		t.Errorf("Stable shuffled sorted %d ints (stability)", n)
	}

	// sorted reversed
	for i := 0; i < len(data); i++ {
		data[i].a = len(data) - i
	}
	data.initB()
	SortStableFunc(data, intPairLess)
	if !IsSortedFunc(data, intPairLess) {
		t.Errorf("Stable didn't sort %d ints", n)
	}
	if !data.inOrder() {
		t.Errorf("Stable wasn't stable on %d ints", n)
	}
}

func TestMinMax(t *testing.T) {
	intLess := func(a, b int) bool { return a < b }

	tests := []struct {
		data    []int
		wantMin int
		wantMax int
	}{
		{[]int{7}, 7, 7},
		{[]int{1, 2}, 1, 2},
		{[]int{2, 1}, 1, 2},
		{[]int{1, 2, 3}, 1, 3},
		{[]int{3, 2, 1}, 1, 3},
		{[]int{2, 1, 3}, 1, 3},
		{[]int{2, 2, 3}, 2, 3},
		{[]int{3, 2, 3}, 2, 3},
		{[]int{0, 2, -9}, -9, 2},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.data), func(t *testing.T) {
			gotMin := Min(tt.data)
			if gotMin != tt.wantMin {
				t.Errorf("Min got %v, want %v", gotMin, tt.wantMin)
			}

			gotMinFunc := MinFunc(tt.data, intLess)
			if gotMinFunc != tt.wantMin {
				t.Errorf("MinFunc got %v, want %v", gotMinFunc, tt.wantMin)
			}

			gotMax := Max(tt.data)
			if gotMax != tt.wantMax {
				t.Errorf("Max got %v, want %v", gotMax, tt.wantMax)
			}

			gotMaxFunc := Max(tt.data)
			if gotMaxFunc != tt.wantMax {
				t.Errorf("MaxFunc got %v, want %v", gotMaxFunc, tt.wantMax)
			}
		})
	}
}

func TestMinMaxNaNs(t *testing.T) {
	fs := []float64{math.NaN(), 999.9, math.Inf(1), -400.4, math.Inf(-1)}
	fmin := Min(fs)
	if !math.IsNaN(fmin) {
		t.Errorf("got min %v, want NaN", fmin)
	}

	fmax := Max(fs)
	if !math.IsNaN(fmax) {
		t.Errorf("got max %v, want NaN", fmax)
	}
}

func TestMinMaxPanics(t *testing.T) {
	intLess := func(a, b int) bool { return a < b }
	emptySlice := []int{}

	if !panics(func() { Min(emptySlice) }) {
		t.Errorf("Min([]): got no panic, want panic")
	}

	if !panics(func() { Max(emptySlice) }) {
		t.Errorf("Max([]): got no panic, want panic")
	}

	if !panics(func() { MinFunc(emptySlice, intLess) }) {
		t.Errorf("MinFunc([]): got no panic, want panic")
	}

	if !panics(func() { MaxFunc(emptySlice, intLess) }) {
		t.Errorf("MaxFunc([]): got no panic, want panic")
	}
}

func TestBinarySearch(t *testing.T) {
	str1 := []string{"foo"}
	str2 := []string{"ab", "ca"}
	str3 := []string{"mo", "qo", "vo"}
	str4 := []string{"ab", "ad", "ca", "xy"}

	// slice with repeating elements
	strRepeats := []string{"ba", "ca", "da", "da", "da", "ka", "ma", "ma", "ta"}

	// slice with all element equal
	strSame := []string{"xx", "xx", "xx"}

	tests := []struct {
		data      []string
		target    string
		wantPos   int
		wantFound bool
	}{
		{[]string{}, "foo", 0, false},
		{[]string{}, "", 0, false},

		{str1, "foo", 0, true},
		{str1, "bar", 0, false},
		{str1, "zx", 1, false},

		{str2, "aa", 0, false},
		{str2, "ab", 0, true},
		{str2, "ad", 1, false},
		{str2, "ca", 1, true},
		{str2, "ra", 2, false},

		{str3, "bb", 0, false},
		{str3, "mo", 0, true},
		{str3, "nb", 1, false},
		{str3, "qo", 1, true},
		{str3, "tr", 2, false},
		{str3, "vo", 2, true},
		{str3, "xr", 3, false},

		{str4, "aa", 0, false},
		{str4, "ab", 0, true},
		{str4, "ac", 1, false},
		{str4, "ad", 1, true},
		{str4, "ax", 2, false},
		{str4, "ca", 2, true},
		{str4, "cc", 3, false},
		{str4, "dd", 3, false},
		{str4, "xy", 3, true},
		{str4, "zz", 4, false},

		{strRepeats, "da", 2, true},
		{strRepeats, "db", 5, false},
		{strRepeats, "ma", 6, true},
		{strRepeats, "mb", 8, false},

		{strSame, "xx", 0, true},
		{strSame, "ab", 0, false},
		{strSame, "zz", 3, false},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			{
				pos, found := BinarySearch(tt.data, tt.target)
				if pos != tt.wantPos || found != tt.wantFound {
					t.Errorf("BinarySearch got (%v, %v), want (%v, %v)", pos, found, tt.wantPos, tt.wantFound)
				}
			}

			{
				pos, found := BinarySearchFunc(tt.data, tt.target, strings.Compare)
				if pos != tt.wantPos || found != tt.wantFound {
					t.Errorf("BinarySearchFunc got (%v, %v), want (%v, %v)", pos, found, tt.wantPos, tt.wantFound)
				}
			}
		})
	}
}

func TestBinarySearchInts(t *testing.T) {
	data := []int{20, 30, 40, 50, 60, 70, 80, 90}
	tests := []struct {
		target    int
		wantPos   int
		wantFound bool
	}{
		{20, 0, true},
		{23, 1, false},
		{43, 3, false},
		{80, 6, true},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.target), func(t *testing.T) {
			{
				pos, found := BinarySearch(data, tt.target)
				if pos != tt.wantPos || found != tt.wantFound {
					t.Errorf("BinarySearch got (%v, %v), want (%v, %v)", pos, found, tt.wantPos, tt.wantFound)
				}
			}

			{
				cmp := func(a, b int) int {
					return a - b
				}
				pos, found := BinarySearchFunc(data, tt.target, cmp)
				if pos != tt.wantPos || found != tt.wantFound {
					t.Errorf("BinarySearchFunc got (%v, %v), want (%v, %v)", pos, found, tt.wantPos, tt.wantFound)
				}
			}
		})
	}
}

func TestBinarySearchFunc(t *testing.T) {
	data := []int{1, 10, 11, 2} // sorted lexicographically
	cmp := func(a int, b string) int {
		return strings.Compare(strconv.Itoa(a), b)
	}
	pos, found := BinarySearchFunc(data, "2", cmp)
	if pos != 3 || !found {
		t.Errorf("BinarySearchFunc(%v, %q, cmp) = %v, %v, want %v, %v", data, "2", pos, found, 3, true)
	}
}
