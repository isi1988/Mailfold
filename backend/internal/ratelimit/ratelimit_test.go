package ratelimit

import (
	"testing"
	"time"
)

func TestAllowWithinAndOverLimit(t *testing.T) {
	l := New(2, time.Minute)
	base := time.Unix(1000, 0)
	l.now = func() time.Time { return base }

	if ok, _ := l.Allow("a"); !ok {
		t.Error("1st request should be allowed")
	}
	if ok, _ := l.Allow("a"); !ok {
		t.Error("2nd request should be allowed")
	}
	ok, retry := l.Allow("a")
	if ok {
		t.Error("3rd request should be denied")
	}
	if retry <= 0 || retry > time.Minute {
		t.Errorf("retry-after out of range: %v", retry)
	}

	// A different key has its own independent budget.
	if ok, _ := l.Allow("b"); !ok {
		t.Error("a different key should have its own budget")
	}
}

func TestWindowReset(t *testing.T) {
	l := New(1, time.Minute)
	now := time.Unix(0, 0)
	l.now = func() time.Time { return now }

	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("1st request should be allowed")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("2nd request in the same window should be denied")
	}
	now = now.Add(time.Minute) // window has elapsed
	if ok, _ := l.Allow("a"); !ok {
		t.Error("request in a fresh window should be allowed")
	}
}

func TestDisabledWhenMaxNonPositive(t *testing.T) {
	l := New(0, time.Minute)
	for i := 0; i < 100; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatal("a max of 0 must always allow")
		}
	}
}

func TestGC(t *testing.T) {
	l := New(1, time.Minute)
	now := time.Unix(0, 0)
	l.now = func() time.Time { return now }

	l.Allow("a")
	now = now.Add(2 * time.Minute) // entry's window has fully elapsed
	l.GC()

	l.mu.Lock()
	remaining := len(l.entries)
	l.mu.Unlock()
	if remaining != 0 {
		t.Errorf("GC should have removed the expired entry, have %d", remaining)
	}
}
