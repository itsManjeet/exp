package syncutil

import "sync"

// Once returns function that invokes fn only once and returns the values
// returned by fn. The returned function is safe for concurrent use.
func Once[T any](fn func() (T, error)) func() (T, error) {
	var (
		once  sync.Once
		value T
		err   error
	)
	return func() (T, error) {
		once.Do(func() {
			value, err = fn()
		})
		return value, err
	}
}
