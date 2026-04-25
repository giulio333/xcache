package timeout_test

import (
	"context"
	"errors"
	"testing"
	"time"

	xcache "github.com/giulio333/xcache"
	"github.com/giulio333/xcache/middleware/timeout"
	"github.com/giulio333/xcache/store/memory"
)

// slowStore is a stub that sleeps for delay before delegating every
// context-bearing call. It is used to simulate a slow backend so that the
// timeout middleware can be observed firing.
type slowStore struct {
	xcache.Store
	delay time.Duration
}

func (s *slowStore) sleep(ctx context.Context) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *slowStore) Get(ctx context.Context, key string) (xcache.Entry, error) {
	if err := s.sleep(ctx); err != nil {
		return xcache.Entry{}, err
	}
	return s.Store.Get(ctx, key)
}

func (s *slowStore) Set(ctx context.Context, key string, value any, opts ...xcache.Option) error {
	if err := s.sleep(ctx); err != nil {
		return err
	}
	return s.Store.Set(ctx, key, value, opts...)
}

func (s *slowStore) Delete(ctx context.Context, key string) error {
	if err := s.sleep(ctx); err != nil {
		return err
	}
	return s.Store.Delete(ctx, key)
}

func (s *slowStore) Clear(ctx context.Context) error {
	if err := s.sleep(ctx); err != nil {
		return err
	}
	return s.Store.Clear(ctx)
}

func (s *slowStore) GetMany(ctx context.Context, keys []string) (map[string]xcache.Entry, error) {
	if err := s.sleep(ctx); err != nil {
		return nil, err
	}
	return s.Store.GetMany(ctx, keys)
}

func (s *slowStore) DeleteMany(ctx context.Context, keys []string) error {
	if err := s.sleep(ctx); err != nil {
		return err
	}
	return s.Store.DeleteMany(ctx, keys)
}

func (s *slowStore) DeleteByTag(ctx context.Context, tag string) error {
	if err := s.sleep(ctx); err != nil {
		return err
	}
	return s.Store.DeleteByTag(ctx, tag)
}

// newSlow returns a slowStore backed by a real MemoryStore.
func newSlow(delay time.Duration) *slowStore {
	return &slowStore{Store: memory.NewStore(), delay: delay}
}

// --- no-op path (d <= 0) ---

func TestWrap_ZeroDuration_ReturnsNext(t *testing.T) {
	base := memory.NewStore()
	defer base.Close()
	wrapped := timeout.Wrap(base, 0)
	// When d <= 0 the original Store is returned unchanged.
	if wrapped != base {
		t.Error("expected Wrap with d=0 to return the original store")
	}
}

func TestWrap_NegativeDuration_ReturnsNext(t *testing.T) {
	base := memory.NewStore()
	defer base.Close()
	wrapped := timeout.Wrap(base, -time.Second)
	if wrapped != base {
		t.Error("expected Wrap with d<0 to return the original store")
	}
}

// --- fast path: operations complete before the deadline ---

func TestGet_Fast(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "k", "v", xcache.WithTTL(time.Minute))
	store := timeout.Wrap(base, 100*time.Millisecond)

	entry, err := store.Get(ctx, "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Value != "v" {
		t.Errorf("expected value 'v', got %v", entry.Value)
	}
}

func TestSet_Fast(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()
	store := timeout.Wrap(base, 100*time.Millisecond)

	if err := store.Set(ctx, "k", "v"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDelete_Fast(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()
	store := timeout.Wrap(base, 100*time.Millisecond)

	if err := store.Delete(ctx, "k"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClear_Fast(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()
	store := timeout.Wrap(base, 100*time.Millisecond)

	if err := store.Clear(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetMany_Fast(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "a", 1)
	_ = base.Set(ctx, "b", 2)
	store := timeout.Wrap(base, 100*time.Millisecond)

	result, err := store.GetMany(ctx, []string{"a", "b", "missing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
}

func TestDeleteMany_Fast(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "x", 1)
	store := timeout.Wrap(base, 100*time.Millisecond)

	if err := store.DeleteMany(ctx, []string{"x"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteByTag_Fast(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "t1", "val", xcache.WithTags("group"))
	store := timeout.Wrap(base, 100*time.Millisecond)

	if err := store.DeleteByTag(ctx, "group"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClose_DelegatesWithoutTimeout(t *testing.T) {
	base := memory.NewStore()
	store := timeout.Wrap(base, 100*time.Millisecond)

	if err := store.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- timeout path: slow store fires the deadline ---

func TestGet_Timeout(t *testing.T) {
	ctx := context.Background()
	slow := newSlow(200 * time.Millisecond)
	defer slow.Close()
	store := timeout.Wrap(slow, 10*time.Millisecond)

	_, err := store.Get(ctx, "k")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestSet_Timeout(t *testing.T) {
	ctx := context.Background()
	slow := newSlow(200 * time.Millisecond)
	defer slow.Close()
	store := timeout.Wrap(slow, 10*time.Millisecond)

	err := store.Set(ctx, "k", "v")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestDelete_Timeout(t *testing.T) {
	ctx := context.Background()
	slow := newSlow(200 * time.Millisecond)
	defer slow.Close()
	store := timeout.Wrap(slow, 10*time.Millisecond)

	err := store.Delete(ctx, "k")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestClear_Timeout(t *testing.T) {
	ctx := context.Background()
	slow := newSlow(200 * time.Millisecond)
	defer slow.Close()
	store := timeout.Wrap(slow, 10*time.Millisecond)

	err := store.Clear(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestGetMany_Timeout(t *testing.T) {
	ctx := context.Background()
	slow := newSlow(200 * time.Millisecond)
	defer slow.Close()
	store := timeout.Wrap(slow, 10*time.Millisecond)

	_, err := store.GetMany(ctx, []string{"a", "b"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestDeleteMany_Timeout(t *testing.T) {
	ctx := context.Background()
	slow := newSlow(200 * time.Millisecond)
	defer slow.Close()
	store := timeout.Wrap(slow, 10*time.Millisecond)

	err := store.DeleteMany(ctx, []string{"k"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestDeleteByTag_Timeout(t *testing.T) {
	ctx := context.Background()
	slow := newSlow(200 * time.Millisecond)
	defer slow.Close()
	store := timeout.Wrap(slow, 10*time.Millisecond)

	err := store.DeleteByTag(ctx, "tag")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// TestCallerDeadline_Shorter verifies that when the caller already has a
// tighter deadline than the middleware's, the caller's deadline wins.
func TestCallerDeadline_Shorter(t *testing.T) {
	slow := newSlow(200 * time.Millisecond)
	defer slow.Close()
	// Middleware timeout is generous; caller has a tight deadline.
	store := timeout.Wrap(slow, 500*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := store.Get(ctx, "k")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded from caller context, got %v", err)
	}
}
