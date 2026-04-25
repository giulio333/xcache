package readonly_test

import (
	"context"
	"errors"
	"testing"
	"time"

	xcache "github.com/giulio333/xcache"
	"github.com/giulio333/xcache/middleware/readonly"
	"github.com/giulio333/xcache/store/memory"
)

func TestGet_DelegatesToNext(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "k1", "hello", xcache.WithTTL(time.Minute))

	store := readonly.Wrap(base)
	entry, err := store.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Value != "hello" {
		t.Errorf("expected value hello, got %v", entry.Value)
	}
}

func TestGet_Miss_ReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	store := readonly.Wrap(base)
	_, err := store.Get(ctx, "missing")
	if !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetMany_DelegatesToNext(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "a", 1)
	_ = base.Set(ctx, "b", 2)

	store := readonly.Wrap(base)
	result, err := store.GetMany(ctx, []string{"a", "b", "missing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
}

func TestSet_ReturnsErrReadOnly(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	store := readonly.Wrap(base)
	err := store.Set(ctx, "k", "v")
	if !errors.Is(err, readonly.ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly, got %v", err)
	}
}

func TestDelete_ReturnsErrReadOnly(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	store := readonly.Wrap(base)
	err := store.Delete(ctx, "k")
	if !errors.Is(err, readonly.ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly, got %v", err)
	}
}

func TestDeleteMany_ReturnsErrReadOnly(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	store := readonly.Wrap(base)
	err := store.DeleteMany(ctx, []string{"k1", "k2"})
	if !errors.Is(err, readonly.ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly, got %v", err)
	}
}

func TestDeleteByTag_ReturnsErrReadOnly(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	store := readonly.Wrap(base)
	err := store.DeleteByTag(ctx, "tag1")
	if !errors.Is(err, readonly.ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly, got %v", err)
	}
}

func TestClear_ReturnsErrReadOnly(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	store := readonly.Wrap(base)
	err := store.Clear(ctx)
	if !errors.Is(err, readonly.ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly, got %v", err)
	}
}

func TestClose_DelegatesToNext(t *testing.T) {
	base := memory.NewStore()
	store := readonly.Wrap(base)
	if err := store.Close(); err != nil {
		t.Fatalf("expected nil error from Close, got %v", err)
	}
}

// TestSet_DoesNotMutateUnderlying verifies that a Set through the read-only
// wrapper does not reach the underlying store.
func TestSet_DoesNotMutateUnderlying(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	store := readonly.Wrap(base)
	_ = store.Set(ctx, "k", "v")

	// The key must not exist in the underlying store.
	_, err := base.Get(ctx, "k")
	if !errors.Is(err, xcache.ErrNotFound) {
		t.Errorf("underlying store was mutated; expected ErrNotFound, got %v", err)
	}
}
