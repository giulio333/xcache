package metrics_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	xcache "github.com/giulio333/xcache"
	"github.com/giulio333/xcache/middleware/metrics"
	"github.com/giulio333/xcache/store/memory"
)

// fakeRecorder counts metric calls per operation so tests can assert on them.
type fakeRecorder struct {
	mu       sync.Mutex
	hits     map[string]int
	misses   map[string]int
	errs     map[string]int
	durations map[string][]time.Duration
}

func newFakeRecorder() *fakeRecorder {
	return &fakeRecorder{
		hits:      make(map[string]int),
		misses:    make(map[string]int),
		errs:      make(map[string]int),
		durations: make(map[string][]time.Duration),
	}
}

func (r *fakeRecorder) RecordHit(op string) {
	r.mu.Lock(); defer r.mu.Unlock()
	r.hits[op]++
}
func (r *fakeRecorder) RecordMiss(op string) {
	r.mu.Lock(); defer r.mu.Unlock()
	r.misses[op]++
}
func (r *fakeRecorder) RecordError(op string) {
	r.mu.Lock(); defer r.mu.Unlock()
	r.errs[op]++
}
func (r *fakeRecorder) RecordDuration(op string, d time.Duration) {
	r.mu.Lock(); defer r.mu.Unlock()
	r.durations[op] = append(r.durations[op], d)
}

// errStore returns a sentinel error for every write / mutation call.
type errStore struct {
	xcache.Store
	err error
}

func (e *errStore) Set(_ context.Context, _ string, _ any, _ ...xcache.Option) error {
	return e.err
}
func (e *errStore) Delete(_ context.Context, _ string) error      { return e.err }
func (e *errStore) DeleteMany(_ context.Context, _ []string) error { return e.err }
func (e *errStore) DeleteByTag(_ context.Context, _ string) error  { return e.err }
func (e *errStore) Clear(_ context.Context) error                  { return e.err }
func (e *errStore) Close() error                                   { return e.err }

func TestGet_Hit(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()
	_ = base.Set(ctx, "k", "v", xcache.WithTTL(time.Minute))

	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	_, err := store.Get(ctx, "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.hits["Get"] != 1 {
		t.Errorf("expected 1 hit for Get, got %d", rec.hits["Get"])
	}
	if len(rec.durations["Get"]) != 1 {
		t.Errorf("expected 1 duration for Get, got %d", len(rec.durations["Get"]))
	}
}

func TestGet_Miss(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	_, err := store.Get(ctx, "missing")
	if !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if rec.misses["Get"] != 1 {
		t.Errorf("expected 1 miss for Get, got %d", rec.misses["Get"])
	}
	if rec.hits["Get"] != 0 {
		t.Errorf("expected 0 hits for Get on miss, got %d", rec.hits["Get"])
	}
}

func TestSet_Hit(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	if err := store.Set(ctx, "k", "v"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.hits["Set"] != 1 {
		t.Errorf("expected 1 hit for Set, got %d", rec.hits["Set"])
	}
}

func TestSet_Error(t *testing.T) {
	ctx := context.Background()
	sentinel := errors.New("set failed")
	base := &errStore{Store: memory.NewStore(), err: sentinel}
	defer base.Store.Close()

	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	err := store.Set(ctx, "k", "v")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
	if rec.errs["Set"] != 1 {
		t.Errorf("expected 1 error for Set, got %d", rec.errs["Set"])
	}
}

func TestDelete_Hit(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	if err := store.Delete(ctx, "k"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.hits["Delete"] != 1 {
		t.Errorf("expected 1 hit for Delete, got %d", rec.hits["Delete"])
	}
}

func TestGetMany(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()
	_ = base.Set(ctx, "a", 1)
	_ = base.Set(ctx, "b", 2)

	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	result, err := store.GetMany(ctx, []string{"a", "b", "missing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
	if rec.hits["GetMany"] != 1 {
		t.Errorf("expected 1 hit for GetMany, got %d", rec.hits["GetMany"])
	}
}

func TestDeleteMany_Hit(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	if err := store.DeleteMany(ctx, []string{"a", "b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.hits["DeleteMany"] != 1 {
		t.Errorf("expected 1 hit for DeleteMany, got %d", rec.hits["DeleteMany"])
	}
}

func TestDeleteByTag_Hit(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()
	_ = base.Set(ctx, "t1", "v", xcache.WithTags("grp"))

	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	if err := store.DeleteByTag(ctx, "grp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.hits["DeleteByTag"] != 1 {
		t.Errorf("expected 1 hit for DeleteByTag, got %d", rec.hits["DeleteByTag"])
	}
}

func TestClear_Hit(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	if err := store.Clear(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.hits["Clear"] != 1 {
		t.Errorf("expected 1 hit for Clear, got %d", rec.hits["Clear"])
	}
}

func TestClose_Hit(t *testing.T) {
	base := memory.NewStore()
	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	if err := store.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.hits["Close"] != 1 {
		t.Errorf("expected 1 hit for Close, got %d", rec.hits["Close"])
	}
}

func TestDuration_AlwaysRecorded(t *testing.T) {
	ctx := context.Background()
	sentinel := errors.New("boom")
	base := &errStore{Store: memory.NewStore(), err: sentinel}
	defer base.Store.Close()

	rec := newFakeRecorder()
	store := metrics.Wrap(base, rec)

	_ = store.Set(ctx, "k", "v")
	if len(rec.durations["Set"]) != 1 {
		t.Errorf("expected duration recorded even on error, got %d", len(rec.durations["Set"]))
	}
}
