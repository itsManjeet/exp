package slog

// A list[T] is a functional sequence.
// It supports three operations: append, len and indexing (at).
//
// Repeated calls to append happen in amortized O(1) space and time. (Appending
// an element allocates one node directly, and the normalize operation always
// doubles the front slice, so we can charge two slots to each element.)
//
// The len method takes constant time.
//
// The at method requires a normalized list, and then takes constant time.
//
// It is possible to obtain quadratic behavior by alternating append and at:
// the normalize required by at is called for each appended element, causing
// front to be copied each time.
type list[T any] struct {
	front   []T
	back    *node[T] // reversed
	lenBack int
}

type node[T any] struct {
	el   T
	next *node[T]
}

func (l list[T]) append(x T) list[T] {
	if l.front == nil {
		return list[T]{
			front:   []T{x},
			back:    nil,
			lenBack: 0,
		}
	}
	if l.lenBack == len(l.front) {
		l = l.normalize()
	}
	return list[T]{
		front:   l.front,
		back:    &node[T]{el: x, next: l.back},
		lenBack: l.lenBack + 1,
	}
}

func (l list[T]) len() int {
	return len(l.front) + l.lenBack
}

func (l list[T]) at(i int) T {
	if l.back != nil {
		panic("not normalized")
	}
	return l.front[i]
}

func (l list[T]) normalize() list[T] {
	if l.back == nil {
		return l
	}
	newFront := make([]T, len(l.front)+l.lenBack)
	copy(newFront, l.front)
	i := len(newFront) - 1
	for b := l.back; b != nil; b = b.next {
		newFront[i] = b.el
		i--
	}
	return list[T]{
		front:   newFront,
		back:    nil,
		lenBack: 0,
	}
}
