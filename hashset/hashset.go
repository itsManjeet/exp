package hashset

import "golang.org/x/exp/maps"

type HashSet[T comparable] struct {
	inner map[T]struct{}
}

func New[T comparable](values ...T) HashSet[T] {
	h := HashSet[T]{
		inner: make(map[T]struct{}, len(values)),
	}
	for _, v := range values {
		h.inner[v] = struct{}{}
	}
	return h
}

func (h HashSet[T]) Add(v T) {
	h.inner[v] = struct{}{}
}

func (h HashSet[T]) Delete(v T) {
	delete(h.inner, v)
}

func (h HashSet[T]) Contains(v T) bool {
	_, ok := h.inner[v]
	return ok
}

func (h HashSet[T]) ContainsAll(values ...T) bool {
	for _, v := range values {
		if _, ok := h.inner[v]; !ok {
			return false
		}
	}
	return true
}

func (h HashSet[T]) Equal(o HashSet[T]) bool {
	return h.ContainsAll(o.ToSlice()...) && o.ContainsAll(h.ToSlice()...)
}

func (h HashSet[T]) ToSlice() []T {
	return maps.Keys(h.inner)
}
