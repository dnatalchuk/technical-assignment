package main

import "testing"

func BenchmarkPostEvent(b *testing.B) {
	hub := newEventHub()
	c1 := &fakeConn{}
	c2 := &fakeConn{}
	hub.registerConn("tenant1", c1)
	hub.registerConn("tenant1", c2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.postEvent("tenant1", "bench")
	}
	b.StopTimer()
	if len(c1.msgs) != b.N || len(c2.msgs) != b.N {
		b.Fatalf("expected %d messages, got %d and %d", b.N, len(c1.msgs), len(c2.msgs))
	}
}
