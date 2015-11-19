package queue

import (
	"testing"
	"time"
)

func TestGrowth(t *testing.T) {
	events := Make()

	for i := 0; i < 20; i++ {
		events.Send(i)
	}

	for want := 0; want < 20; want++ {
		got := events.NextEvent()
		if got != want {
			t.Errorf("NextEvent()=%d, want %d", got, want)
		}
	}
}

func TestRelease(t *testing.T) {
	events := Make()

	events.Send(6)
	events.Send(7)
	events.Send(8)
	events.NextEvent()

	const want = -1
	events.Release(want)

	if got := events.NextEvent(); got != want {
		t.Errorf("NextEvent()=%d, want %d", got, want)
	}
}

func TestReleaseSignal(t *testing.T) {
	events := Make()

	events.Send(6)
	events.NextEvent()

	const want = -1
	go func() {
		time.Sleep(10 * time.Millisecond)
		events.Release(want)
	}()

	if got := events.NextEvent(); got != want {
		t.Errorf("NextEvent()=%d, want %d", got, want)
	}

	if !panics(events.NextEvent) {
		t.Error("NextEvent() after close did not panic")
	}
}

func panics(f func() interface{}) (res bool) {
	defer func() {
		if e := recover(); e != nil {
			res = true
		}
	}()
	f()
	return false
}
