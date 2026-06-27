package tqueue

import (
	"context"
	"testing"
)

// eidMax is MaxID expressed as EventID for table literals (MaxID is a typed
// int32 const, so it needs conversion in EventID-typed struct fields).
const eidMax = EventID(MaxID)

// TestEventIDValue exercises EventID.Value, the raw int32 accessor.
func TestEventIDValue(t *testing.T) {
	for _, tc := range []struct {
		in   EventID
		want int32
	}{
		{0, 0},
		{1, 1},
		{42, 42},
		{eidMax, MaxID},
		{-7, -7},
	} {
		if got := tc.in.Value(); got != tc.want {
			t.Errorf("EventID(%d).Value() = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestEventIDEmpty exercises EventID.Empty.
func TestEventIDEmpty(t *testing.T) {
	for _, tc := range []struct {
		in   EventID
		want bool
	}{
		{0, true},
		{1, false},
		{-1, false},
		{eidMax, false},
	} {
		if got := tc.in.Empty(); got != tc.want {
			t.Errorf("EventID(%d).Empty() = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestEventIDIsValid exercises EventID.IsValid across the range boundaries.
func TestEventIDIsValid(t *testing.T) {
	for _, tc := range []struct {
		in   EventID
		want bool
	}{
		{0, false},      // zero is not a valid id (must be > 0)
		{1, true},       // first valid id
		{-1, false},     // negative
		{eidMax, true},     // upper bound inclusive
		{eidMax + 1, false}, // over max
	} {
		if got := tc.in.IsValid(); got != tc.want {
			t.Errorf("EventID(%d).IsValid() = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestEventIDEqual exercises EventID.Equal.
func TestEventIDEqual(t *testing.T) {
	for _, tc := range []struct {
		a, b EventID
		want bool
	}{
		{5, 5, true},
		{5, 6, false},
		{0, 0, true},
		{eidMax, eidMax, true},
		{1, eidMax, false},
	} {
		if got := tc.a.Equal(tc.b); got != tc.want {
			t.Errorf("EventID(%d).Equal(%d) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestEventIDLess exercises EventID.Less.
func TestEventIDLess(t *testing.T) {
	for _, tc := range []struct {
		a, b EventID
		want bool
	}{
		{5, 6, true},
		{6, 5, false},
		{5, 5, false}, // not strictly less
		{0, 1, true},
		{eidMax - 1, eidMax, true},
	} {
		if got := tc.a.Less(tc.b); got != tc.want {
			t.Errorf("EventID(%d).Less(%d) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestEventIDFromInt32 covers both the success and error branches.
func TestEventIDFromInt32(t *testing.T) {
	for _, tc := range []struct {
		in        int32
		want      EventID
		wantError bool
	}{
		{0, 0, false},
		{1, 1, false},
		{42, 42, false},
		{MaxID, eidMax, false},
		{-1, 0, true},       // negative → error
		{MaxID + 1, 0, true}, // over max → error
	} {
		got, err := EventIDFromInt32(tc.in)
		if tc.wantError {
			if err == nil {
				t.Errorf("EventIDFromInt32(%d): expected error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("EventIDFromInt32(%d): unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("EventIDFromInt32(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestCallbackAccessor covers SetCallback + Callback round-trip (Callback is 0%).
func TestCallbackAccessor(t *testing.T) {
	tq := New()
	if cb := tq.Callback(); cb != nil {
		t.Errorf("fresh TQueue Callback() = %v, want nil", cb)
	}
	inst := &memCB{}
	tq.SetCallback(inst)
	if got := tq.Callback(); got != inst {
		t.Errorf("Callback() = %p, want %p", got, inst)
	}
}

// TestNowFunc covers the NowFunc setter. The default clock func is also
// exercised so New's function literal is covered.
func TestNowFunc(t *testing.T) {
	tq := New()
	// Exercise the default clock assigned by New (covers the func literal body).
	if v := tq.now(); v <= 0 {
		t.Errorf("default now() = %d, want > 0", v)
	}
	// Replace it with a fixed clock.
	fixed := int32(12345)
	tq.NowFunc(func() int32 { return fixed })
	if got := tq.now(); got != fixed {
		t.Errorf("custom now() = %d, want %d", got, fixed)
	}
}

// TestClear exercises Clear: removing all-but-N events, the dropped RawEvent
// return, and the missing-queue early return.
func TestClear(t *testing.T) {
	t.Run("keepSome", func(t *testing.T) {
		tq := New()
		tq.SetCallback(&memCB{})
		qid := QueueID(1)
		ctx := context.Background()
		for i := 0; i < 5; i++ {
			if _, err := tq.Push(ctx, qid, []byte{byte('a' + i)}, 0, 0, 0); err != nil {
				t.Fatal(err)
			}
		}

		// Keep the last 2 events → drop the oldest 3.
		dropped := tq.Clear(ctx, qid, 2)
		if len(dropped) != 3 {
			t.Fatalf("len(dropped) = %d, want 3", len(dropped))
		}
		if tq.Size(qid) != 2 {
			t.Errorf("Size after Clear = %d, want 2", tq.Size(qid))
		}
		// Surviving events are the last two assigned ids (4 and 5).
		if got := tq.Head(qid); got != 4 {
			t.Errorf("Head after Clear = %d, want 4", got)
		}
		if got := tq.Tail(qid); got != 5 {
			t.Errorf("Tail after Clear = %d, want 5", got)
		}
		// Dropped events are the oldest, in ascending order.
		if dropped[0].ID != 1 || dropped[2].ID != 3 {
			t.Errorf("dropped ids = %d,%d,%d, want 1,2,3", dropped[0].ID, dropped[1].ID, dropped[2].ID)
		}
	})

	t.Run("keepNone", func(t *testing.T) {
		tq := New()
		qid := QueueID(1)
		ctx := context.Background()
		for i := 0; i < 3; i++ {
			if _, err := tq.Push(ctx, qid, []byte("x"), 0, 0, 0); err != nil {
				t.Fatal(err)
			}
		}
		dropped := tq.Clear(ctx, qid, 0)
		if len(dropped) != 3 {
			t.Fatalf("len(dropped) = %d, want 3", len(dropped))
		}
		if tq.Size(qid) != 0 {
			t.Errorf("Size after Clear(0) = %d, want 0", tq.Size(qid))
		}
		if got := tq.Head(qid); got != 0 {
			t.Errorf("Head after Clear(0) = %d, want 0 (empty)", got)
		}
	})

	t.Run("keepMoreThanPresent", func(t *testing.T) {
		tq := New()
		qid := QueueID(1)
		ctx := context.Background()
		tq.Push(ctx, qid, []byte("x"), 0, 0, 0)
		// keepCount > size → nothing dropped.
		dropped := tq.Clear(ctx, qid, 10)
		if len(dropped) != 0 {
			t.Fatalf("len(dropped) = %d, want 0", len(dropped))
		}
		if tq.Size(qid) != 1 {
			t.Errorf("Size = %d, want 1", tq.Size(qid))
		}
	})

	t.Run("noSuchQueue", func(t *testing.T) {
		tq := New()
		if got := tq.Clear(context.Background(), QueueID(999), 0); got != nil {
			t.Errorf("Clear on missing queue = %v, want nil", got)
		}
	})
}

// TestReplay exercises Replay: restoring persisted events into a fresh TQueue
// and verifying head/tail/size/get.
func TestReplay(t *testing.T) {
	tq := New()
	qid := QueueID(7)

	events := []RawEvent{
		{QueueID: qid, ID: 3, ExpiresAt: 0, Data: []byte("c"), Extra: 1, LogID: 30, UpdateType: "msg"},
		{QueueID: qid, ID: 1, ExpiresAt: 0, Data: []byte("a"), Extra: 1, LogID: 10, UpdateType: "msg"},
		{QueueID: qid, ID: 2, ExpiresAt: 0, Data: []byte("b"), Extra: 1, LogID: 20, UpdateType: "msg"},
	}
	tq.Replay(events)

	if got := tq.Size(qid); got != 3 {
		t.Fatalf("Size after Replay = %d, want 3", got)
	}
	// head = smallest id, tail = largest id (replay tracks both).
	if got := tq.Head(qid); got != 1 {
		t.Errorf("Head = %d, want 1", got)
	}
	if got := tq.Tail(qid); got != 3 {
		t.Errorf("Tail = %d, want 3", got)
	}

	// Get all events strictly after 0 → sorted ascending by id.
	got, err := tq.Get(context.Background(), qid, 0, false, 1<<30, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("Get returned %d events, want 3", len(got))
	}
	wantIDs := []EventID{1, 2, 3}
	for i, e := range got {
		if e.ID != wantIDs[i] {
			t.Errorf("event %d: ID = %d, want %d", i, e.ID, wantIDs[i])
		}
	}
	// Data and metadata survived the round-trip.
	if string(got[1].Data) != "b" || got[1].UpdateType != "msg" || got[1].Extra != 1 {
		t.Errorf("event 2 metadata mismatch: %+v", got[1])
	}

	// Replay into a second queue does not disturb the first.
	qid2 := QueueID(8)
	tq.Replay([]RawEvent{{QueueID: qid2, ID: 1, Data: []byte("z")}})
	if tq.Size(qid) != 3 || tq.Size(qid2) != 1 {
		t.Errorf("sizes = %d/%d, want 3/1", tq.Size(qid), tq.Size(qid2))
	}
}

// TestTrimOldestLocked exercises the cap-enforcement helper directly (it is
// otherwise only reachable after pushing >100k events). A memCB is used so the
// LogID-pop branch is also covered.
func TestTrimOldestLocked(t *testing.T) {
	tq := New()
	tq.SetCallback(&memCB{})
	qid := QueueID(1)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := tq.Push(ctx, qid, []byte{byte('a' + i)}, 0, 0, 0); err != nil {
			t.Fatal(err)
		}
	}

	// Reach the internal queue (same package) and trim the oldest 3.
	tq.mu.RLock()
	q := tq.queues[qid]
	tq.mu.RUnlock()

	pops := tq.trimOldestLocked(q, 3)
	if len(pops) != 3 {
		t.Fatalf("len(pops) = %d, want 3 (each trimmed event had a LogID)", len(pops))
	}
	// Oldest three (ids 1,2,3) removed; head recomputed to 4.
	if q.head != 4 {
		t.Errorf("q.head = %d, want 4", q.head)
	}
	if len(q.events) != 2 {
		t.Errorf("len(q.events) = %d, want 2", len(q.events))
	}

	// count > len(ids): trims everything remaining without panicking.
	pops2 := tq.trimOldestLocked(q, 100)
	if len(pops2) != 2 {
		t.Errorf("len(pops2) = %d, want 2", len(pops2))
	}
	if q.head != 0 || len(q.events) != 0 {
		t.Errorf("after trimming all: head=%d len=%d, want 0/0", q.head, len(q.events))
	}
}

// TestHeadTailSizeEmpty covers the q==nil early-return branch in Head/Tail/Size
// (a queue id that was never pushed to).
func TestHeadTailSizeEmpty(t *testing.T) {
	tq := New()
	missing := QueueID(404)
	if got := tq.Head(missing); got != 0 {
		t.Errorf("Head(missing) = %d, want 0", got)
	}
	if got := tq.Tail(missing); got != 0 {
		t.Errorf("Tail(missing) = %d, want 0", got)
	}
	if got := tq.Size(missing); got != 0 {
		t.Errorf("Size(missing) = %d, want 0", got)
	}

	// A populated queue reports the expected values.
	qid := QueueID(1)
	tq.Push(context.Background(), qid, []byte("a"), 0, 0, 0)
	tq.Push(context.Background(), qid, []byte("b"), 0, 0, 0)
	if got := tq.Head(qid); got != 1 {
		t.Errorf("Head = %d, want 1", got)
	}
	if got := tq.Tail(qid); got != 2 {
		t.Errorf("Tail = %d, want 2", got)
	}
	if got := tq.Size(qid); got != 2 {
		t.Errorf("Size = %d, want 2", got)
	}
}

// TestForgetNoSuchQueue covers the q==nil early return in Forget.
func TestForgetNoSuchQueue(t *testing.T) {
	tq := New()
	// Forgetting from a non-existent queue must be a no-op (no panic).
	tq.Forget(context.Background(), QueueID(404), 1)
	// Forgetting a non-existent id in an existing queue is also a no-op.
	qid := QueueID(1)
	tq.Push(context.Background(), qid, []byte("a"), 0, 0, 0)
	tq.Forget(context.Background(), qid, 9999)
	if tq.Size(qid) != 1 {
		t.Errorf("Size after bogus Forget = %d, want 1", tq.Size(qid))
	}
}

// TestPushWithDataBuildError covers the build-func error early return in
// PushWithData.
func TestPushWithDataBuildError(t *testing.T) {
	tq := New()
	tq.SetCallback(&memCB{})
	ctx := context.Background()
	buildErr := errBuildSentinel{}
	_, err := tq.PushWithData(ctx, QueueID(1), 0, 0, 0, func(EventID) ([]byte, string, error) {
		return nil, "", buildErr
	})
	if err != buildErr {
		t.Errorf("PushWithData build-error: err = %v, want %v", err, buildErr)
	}
	// Nothing should have been persisted.
	if tq.Size(QueueID(1)) != 0 {
		t.Errorf("Size after failed push = %d, want 0", tq.Size(QueueID(1)))
	}
}

// errBuildSentinel is a sentinel error used by TestPushWithDataBuildError.
type errBuildSentinel struct{}

func (errBuildSentinel) Error() string { return "build failed" }

// TestGetForgetWithCallback drives forgetUpToLocked with events that carry a
// non-zero LogID (via memCB), covering the LogID-pop branch inside it.
func TestGetForgetWithCallback(t *testing.T) {
	tq := New()
	cb := &memCB{}
	tq.SetCallback(cb)
	qid := QueueID(1)
	ctx := context.Background()
	a, _ := tq.Push(ctx, qid, []byte("a"), 0, 0, 0)
	b, _ := tq.Push(ctx, qid, []byte("b"), 0, 0, 0)
	tq.Push(ctx, qid, []byte("c"), 0, 0, 0)

	// Confirm up to b with forgetPrevious: drops a and b (both have LogIDs).
	got, err := tq.Get(ctx, qid, b, true, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != b+1 {
		t.Errorf("Get(forget) = %+v, want only id %d", got, b+1)
	}
	if tq.Size(qid) != 1 {
		t.Errorf("Size after forget = %d, want 1", tq.Size(qid))
	}
	if cb.popped != 2 {
		t.Errorf("cb.popped = %d, want 2 (a and b had LogIDs)", cb.popped)
	}
	_ = a
}

// TestRunGCWithCallback drives RunGC over expired events that carry a non-zero
// LogID, covering the LogID-pop branch inside RunGC.
func TestRunGCWithCallback(t *testing.T) {
	tq := New()
	cb := &memCB{}
	tq.SetCallback(cb)
	qid := QueueID(1)
	ctx := context.Background()
	// Two expired, one live. All get LogIDs from memCB.
	tq.Push(ctx, qid, []byte("old1"), 10, 0, 0)  // expired at now=200
	tq.Push(ctx, qid, []byte("old2"), 100, 0, 0) // expired at now=200
	tq.Push(ctx, qid, []byte("new"), 999999999, 0, 0)

	deleted, complete := tq.RunGC(200)
	if deleted != 2 || !complete {
		t.Fatalf("RunGC: deleted=%d complete=%v, want 2/true", deleted, complete)
	}
	if tq.Size(qid) != 1 {
		t.Errorf("Size after GC = %d, want 1", tq.Size(qid))
	}
	if cb.popped != 2 {
		t.Errorf("cb.popped = %d, want 2 (expired events had LogIDs)", cb.popped)
	}
	// The surviving event is the live one.
	if got := tq.Head(qid); got != 3 {
		t.Errorf("Head after GC = %d, want 3", got)
	}
}

// TestRunGCEmpty covers RunGC over a TQueue with no queues (no deletion path),
// and over queues containing only non-expired events.
func TestRunGCEmpty(t *testing.T) {
	t.Run("noQueues", func(t *testing.T) {
		tq := New()
		deleted, complete := tq.RunGC(1 << 30)
		if deleted != 0 || !complete {
			t.Errorf("RunGC no-queues: deleted=%d complete=%v, want 0/true", deleted, complete)
		}
	})

	t.Run("neverExpiring", func(t *testing.T) {
		tq := New()
		qid := QueueID(1)
		ctx := context.Background()
		tq.Push(ctx, qid, []byte("forever"), 0, 0, 0) // ExpiresAt == 0 → never expires
		deleted, complete := tq.RunGC(1 << 30)
		if deleted != 0 || !complete {
			t.Errorf("RunGC never-expiring: deleted=%d complete=%v, want 0/true", deleted, complete)
		}
		if tq.Size(qid) != 1 {
			t.Errorf("Size = %d, want 1 (event must survive GC)", tq.Size(qid))
		}
	})
}

// TestPopOutsideLock covers the nil-callback early return and the normal path.
func TestPopOutsideLock(t *testing.T) {
	ctx := context.Background()

	// nil callback + non-empty pops: no-op, no panic.
	popOutsideLock(ctx, nil, []uint64{1, 2, 3})

	// nil callback + empty pops.
	popOutsideLock(ctx, nil, nil)

	// real callback: every id is popped.
	cb := &memCB{}
	popOutsideLock(ctx, cb, []uint64{7, 8, 9})
	if cb.popped != 3 {
		t.Errorf("cb.popped = %d, want 3", cb.popped)
	}

	// empty pops slice with real callback: no pops.
	cb2 := &memCB{}
	popOutsideLock(ctx, cb2, nil)
	if cb2.popped != 0 {
		t.Errorf("cb2.popped = %d, want 0", cb2.popped)
	}
}

// TestPushWithDataUsesHint covers the hint branch of PushWithData: when hint is
// greater than the current tail, the next id is derived from the hint.
func TestPushWithDataUsesHint(t *testing.T) {
	tq := New()
	qid := QueueID(1)
	ctx := context.Background()

	// First push assigns id 1.
	id1, err := tq.Push(ctx, qid, []byte("a"), 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Push with a hint far beyond the tail; the next id must exceed the hint.
	id2, err := tq.Push(ctx, qid, []byte("b"), 0, 0, EventID(100))
	if err != nil {
		t.Fatal(err)
	}
	if id2 <= EventID(100) {
		t.Errorf("hinted push id = %d, want > 100", id2)
	}
	if id2 <= id1 {
		t.Errorf("ids not monotonic: %d then %d", id1, id2)
	}
	if tq.Tail(qid) != id2 {
		t.Errorf("Tail = %d, want %d", tq.Tail(qid), id2)
	}
}
