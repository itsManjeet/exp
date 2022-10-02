package hashset_test

import (
	"testing"

	"golang.org/x/exp/hashset"
	"golang.org/x/exp/slices"
)

func TestHashSetString(t *testing.T) {
	h := hashset.New("foo")
	h.Add("bar")
	testContains(t, h, "foo", true)
	testContains(t, h, "bar", true)
	testContains(t, h, "baz", false)
	testToSlice(t, h, []string{"foo", "bar"})

	h.Delete("foo")
	testContains(t, h, "foo", false)
	testContains(t, h, "bar", true)
	testContains(t, h, "baz", false)
	testToSlice(t, h, []string{"bar"})
}

func TestHashSetUint64(t *testing.T) {
	h := hashset.New(uint64(1))
	h.Add(uint64(2))
	testContains(t, h, uint64(1), true)
	testContains(t, h, uint64(2), true)
	testContains(t, h, uint64(3), false)
	testToSlice(t, h, []uint64{1, 2})

	h.Delete(uint64(1))
	testContains(t, h, uint64(1), false)
	testContains(t, h, uint64(2), true)
	testContains(t, h, uint64(3), false)
	testToSlice(t, h, []uint64{2})
}

func TestHashSetStringEqual(t *testing.T) {
	testCases := []struct {
		h, o hashset.HashSet[string]
		want bool
	}{
		{h: hashset.New[string](), o: hashset.New[string](), want: true},
		{h: hashset.New("foo"), o: hashset.New("foo"), want: true},
		{h: hashset.New("foo", "bar"), o: hashset.New("bar", "foo"), want: true},
		{h: hashset.New("foo"), o: hashset.New[string](), want: false},
		{h: hashset.New[string](), o: hashset.New("foo"), want: false},
		{h: hashset.New("foo"), o: hashset.New("bar"), want: false},
		{h: hashset.New("foo"), o: hashset.New("foo", "bar"), want: false},
	}
	for _, tc := range testCases {
		testEqual(t, tc.h, tc.o, tc.want)
	}
}

func TestHashSetUint64Equal(t *testing.T) {
	testCases := []struct {
		h, o hashset.HashSet[uint64]
		want bool
	}{
		{h: hashset.New[uint64](), o: hashset.New[uint64](), want: true},
		{h: hashset.New(uint64(1)), o: hashset.New(uint64(1)), want: true},
		{h: hashset.New(uint64(1), uint64(2)), o: hashset.New(uint64(2), uint64(1)), want: true},
		{h: hashset.New(uint64(1)), o: hashset.New[uint64](), want: false},
		{h: hashset.New[uint64](), o: hashset.New(uint64(1)), want: false},
		{h: hashset.New(uint64(1)), o: hashset.New(uint64(2)), want: false},
		{h: hashset.New(uint64(1)), o: hashset.New(uint64(1), uint64(2)), want: false},
	}
	for _, tc := range testCases {
		testEqual(t, tc.h, tc.o, tc.want)
	}
}

func testContains[T comparable](t *testing.T, h hashset.HashSet[T], v T, want bool) {
	t.Helper()
	if got := h.Contains(v); got != want {
		t.Errorf("result mismatch, got=%v, want=%v", got, want)
	}
}

func testToSlice[T comparable](t *testing.T, h hashset.HashSet[T], want []T) {
	t.Helper()
	if got := h.ToSlice(); !slicesEqualAsSet(got, want) {
		t.Errorf("result mismatch, got=%v, want=%v", got, want)
	}
}

func testEqual[T comparable](t *testing.T, h, o hashset.HashSet[T], want bool) {
	t.Helper()
	if got := h.Equal(o); got != want {
		t.Errorf("result mismatch, h=%v, o=%v, got=%v, want=%v", h, o, got, want)
	}
}

func slicesEqualAsSet[E comparable](s, t []E) bool {
	return slicesContainsAll(s, t) && slicesContainsAll(t, s)
}

func slicesContainsAll[E comparable](s, values []E) bool {
	for _, v := range values {
		if !slices.Contains(s, v) {
			return false
		}
	}
	return true
}
