// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

import (
	"math/rand"
	"sort"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/exp/constraints"
)

// These benchmarks compare sorting a large slice of int with sort.Ints vs.
// slices.Sort
func makeRandomInts[T constraints.Integer](n int) []T {
	rand.Seed(42)
	ints := make([]T, n)
	for i := 0; i < n; i++ {
		ints[i] = T(rand.Intn(n))
	}
	return ints
}

func makeSortedInts[T constraints.Integer](n int) []T {
	ints := make([]T, n)
	for i := 0; i < n; i++ {
		ints[i] = T(i)
	}

	var signTest T
	signTest--
	signed := signTest < 0
	max := uint64(1 << unsafe.Sizeof(signTest) * 8)
	if signed {
		max = max>>1 - 1
	}
	if uint64(n) > max {
		Sort(ints)
	}

	return ints
}

func makeReversedInts[T constraints.Integer](n int) []T {
	ints := make([]T, n)
	for i := 0; i < n; i++ {
		ints[i] = T(n - i)
	}

	var signTest T
	signTest--
	signed := signTest < 0
	max := uint64(1 << unsafe.Sizeof(signTest) * 8)
	if signed {
		max = max>>1 - 1
	}
	if uint64(n) > max {
		Sort(ints)
		// Reverse
		for i := 0; i < len(ints)/2; i++ {
			j := len(ints) - 1 - i
			ints[i], ints[j] = ints[j], ints[i]
		}
	}

	return ints
}

const N = 100_000

func BenchmarkSortInts(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ints := makeRandomInts[int](N)
		b.StartTimer()
		sort.Ints(ints)
	}
}

func BenchmarkSlicesSortInts(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ints := makeRandomInts[int](N)
		b.StartTimer()
		Sort(ints)
	}
}

func BenchmarkSlicesSortInts_Sorted(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ints := makeSortedInts[int](N)
		b.StartTimer()
		Sort(ints)
	}
}

func BenchmarkSlicesSortInts_Reversed(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ints := makeReversedInts[int](N)
		b.StartTimer()
		Sort(ints)
	}
}

// Since we're benchmarking these sorts against each other, make sure that they
// generate similar results.
func TestIntSorts(t *testing.T) {
	ints := makeRandomInts[int](200)
	ints2 := Clone(ints)

	sort.Ints(ints)
	Sort(ints2)

	for i := range ints {
		if ints[i] != ints2[i] {
			t.Fatalf("ints2 mismatch at %d; %d != %d", i, ints[i], ints2[i])
		}
	}
}

// The following is a benchmark for sorting strings.

// makeRandomStrings generates n random strings with alphabetic runes of
// varying lenghts.
func makeRandomStrings(n int) []string {
	rand.Seed(42)
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	ss := make([]string, n)
	for i := 0; i < n; i++ {
		var sb strings.Builder
		slen := 2 + rand.Intn(50)
		for j := 0; j < slen; j++ {
			sb.WriteRune(letters[rand.Intn(len(letters))])
		}
		ss[i] = sb.String()
	}
	return ss
}

func TestStringSorts(t *testing.T) {
	ss := makeRandomStrings(200)
	ss2 := Clone(ss)

	sort.Strings(ss)
	Sort(ss2)

	for i := range ss {
		if ss[i] != ss2[i] {
			t.Fatalf("ss2 mismatch at %d; %s != %s", i, ss[i], ss2[i])
		}
	}
}

func BenchmarkSortStrings(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ss := makeRandomStrings(N)
		b.StartTimer()
		sort.Strings(ss)
	}
}

func BenchmarkSlicesSortStrings(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ss := makeRandomStrings(N)
		b.StartTimer()
		Sort(ss)
	}
}

// These benchmarks compare sorting a slice of structs with sort.Sort vs.
// slices.SortFunc.
type myStruct struct {
	a, b, c, d string
	n          int
}

type myStructs []*myStruct

func (s myStructs) Len() int           { return len(s) }
func (s myStructs) Less(i, j int) bool { return s[i].n < s[j].n }
func (s myStructs) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func makeRandomStructs(n int) myStructs {
	rand.Seed(42)
	structs := make([]*myStruct, n)
	for i := 0; i < n; i++ {
		structs[i] = &myStruct{n: rand.Intn(n)}
	}
	return structs
}

func TestStructSorts(t *testing.T) {
	ss := makeRandomStructs(200)
	ss2 := make([]*myStruct, len(ss))
	for i := range ss {
		ss2[i] = &myStruct{n: ss[i].n}
	}

	sort.Sort(ss)
	SortFunc(ss2, func(a, b *myStruct) bool { return a.n < b.n })

	for i := range ss {
		if *ss[i] != *ss2[i] {
			t.Fatalf("ints2 mismatch at %d; %v != %v", i, *ss[i], *ss2[i])
		}
	}
}

func BenchmarkSortStructs(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ss := makeRandomStructs(N)
		b.StartTimer()
		sort.Sort(ss)
	}
}

func BenchmarkSortFuncStructs(b *testing.B) {
	lessFunc := func(a, b *myStruct) bool { return a.n < b.n }
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ss := makeRandomStructs(N)
		b.StartTimer()
		SortFunc(ss, lessFunc)
	}
}

func benchmarkSlicesSortIntsGeneric[T constraints.Integer](b *testing.B, ints []T) {
	ints2 := make([]T, N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		copy(ints2, ints)
		b.StartTimer()
		Sort(ints2)
	}
}

func benchmarkSlicesSortIntsGenericNotCounting[T constraints.Integer](b *testing.B, ints []T) {
	ints2 := make([]T, N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		copy(ints2, ints)
		b.StartTimer()
		sortNotCounting(ints2)
	}
}

func BenchmarkSlicesSortUint8_Random(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeRandomInts[uint8](N))
}
func BenchmarkSlicesSortUint8_Random_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeRandomInts[uint8](N))
}
func BenchmarkSlicesSortUint16_Random(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeRandomInts[uint16](N))
}
func BenchmarkSlicesSortUint16_Random_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeRandomInts[uint16](N))
}
func BenchmarkSlicesSortInt8_Random(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeRandomInts[int8](N))
}
func BenchmarkSlicesSortInt8_Random_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeRandomInts[int8](N))
}
func BenchmarkSlicesSortInt16_Random(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeRandomInts[int16](N))
}
func BenchmarkSlicesSortInt16_Random_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeRandomInts[int16](N))
}
func BenchmarkSlicesSortUint8_Sorted(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeSortedInts[uint8](N))
}
func BenchmarkSlicesSortUint8_Sorted_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeSortedInts[uint8](N))
}
func BenchmarkSlicesSortUint16_Sorted(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeSortedInts[uint16](N))
}
func BenchmarkSlicesSortUint16_Sorted_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeSortedInts[uint16](N))
}
func BenchmarkSlicesSortInt8_Sorted(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeSortedInts[int8](N))
}
func BenchmarkSlicesSortInt8_Sorted_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeSortedInts[int8](N))
}
func BenchmarkSlicesSortInt16_Sorted(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeSortedInts[int16](N))
}
func BenchmarkSlicesSortInt16_Sorted_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeSortedInts[int16](N))
}
func BenchmarkSlicesSortUint8_Reversed(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeReversedInts[uint8](N))
}
func BenchmarkSlicesSortUint8_Reversed_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeReversedInts[uint8](N))
}
func BenchmarkSlicesSortUint16_Reversed(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeReversedInts[uint16](N))
}
func BenchmarkSlicesSortUint16_Reversed_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeReversedInts[uint16](N))
}
func BenchmarkSlicesSortInt8_Reversed(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeReversedInts[int8](N))
}
func BenchmarkSlicesSortInt8_Reversed_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeReversedInts[int8](N))
}
func BenchmarkSlicesSortInt16_Reversed(b *testing.B) {
	benchmarkSlicesSortIntsGeneric(b, makeReversedInts[int16](N))
}
func BenchmarkSlicesSortInt16_Reversed_NotCounting(b *testing.B) {
	benchmarkSlicesSortIntsGenericNotCounting(b, makeReversedInts[int16](N))
}

// Ensure that counting sort isn't slower at the threshold for some reandom input

func benchmarkCountingThreshold_Counting8[T ~uint8 | ~int8](b *testing.B, n int) {
	ints := makeRandomInts[T](n)
	ints2 := make([]T, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		copy(ints2, ints)
		b.StartTimer()
		countingSort256(ints2)
	}
}

func benchmarkCountingThreshold_Counting16[T ~uint16 | ~int16](b *testing.B, n int) {
	ints := makeRandomInts[T](n)
	ints2 := make([]T, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		copy(ints2, ints)
		b.StartTimer()
		countingSort65536(ints2)
	}
}

func benchmarkCountingThreshold_NotCounting[T ~uint8 | ~int8 | ~uint16 | ~int16](b *testing.B, n int) {
	ints := makeRandomInts[T](n)
	ints2 := make([]T, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		copy(ints2, ints)
		b.StartTimer()
		sortNotCounting(ints2)
	}
}

func BenchmarkCountingThreshold_Counting_Uint8(b *testing.B) {
	benchmarkCountingThreshold_Counting8[uint8](b, countingThreshold256)
}
func BenchmarkCountingThreshold_NotCounting_Uint8(b *testing.B) {
	benchmarkCountingThreshold_NotCounting[uint8](b, countingThreshold256)
}
func BenchmarkCountingThreshold_Counting_Int8(b *testing.B) {
	benchmarkCountingThreshold_Counting8[int8](b, countingThreshold256)
}
func BenchmarkCountingThreshold_NotCounting_Int8(b *testing.B) {
	benchmarkCountingThreshold_NotCounting[int8](b, countingThreshold256)
}
func BenchmarkCountingThreshold_Counting_Uint16(b *testing.B) {
	benchmarkCountingThreshold_Counting16[uint16](b, countingThreshold65536)
}
func BenchmarkCountingThreshold_NotCounting_Uint16(b *testing.B) {
	benchmarkCountingThreshold_NotCounting[uint16](b, countingThreshold65536)
}
func BenchmarkCountingThreshold_Counting_Int16(b *testing.B) {
	benchmarkCountingThreshold_Counting16[int16](b, countingThreshold65536)
}
func BenchmarkCountingThreshold_NotCounting_Int16(b *testing.B) {
	benchmarkCountingThreshold_NotCounting[int16](b, countingThreshold65536)
}
