package memory_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	xcache "github.com/giulio333/xcache"
	"github.com/giulio333/xcache/store/memory"
)

func TestMemoryStore_GetMissingReturnsNotFound(t *testing.T) {
	s := memory.NewStore()
	defer s.Close()

	if _, err := s.Get(context.Background(), "nope"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_GetReturnsEntryMetadata(t *testing.T) {
	s := memory.NewStore()
	defer s.Close()
	ctx := context.Background()

	_ = s.Set(ctx, "k", "v", xcache.WithTTL(time.Minute), xcache.WithTags("a", "b"))

	entry, err := s.Get(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Value != "v" {
		t.Fatalf("unexpected value: %v", entry.Value)
	}
	if entry.ExpiresAt.IsZero() {
		t.Fatalf("ExpiresAt should be populated when TTL is set")
	}
	if len(entry.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", entry.Tags)
	}
	if entry.RemainingTTL() <= 0 || entry.RemainingTTL() > time.Minute {
		t.Fatalf("RemainingTTL out of range: %v", entry.RemainingTTL())
	}
}

func TestMemoryStore_PassiveEvictionOnGet(t *testing.T) {
	s := memory.NewStore(memory.WithSweepInterval(time.Hour))
	defer s.Close()
	ctx := context.Background()

	_ = s.Set(ctx, "k", "v", xcache.WithTTL(20*time.Millisecond), xcache.WithTags("g"))
	time.Sleep(40 * time.Millisecond)

	if _, err := s.Get(ctx, "k"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after TTL, got %v", err)
	}

	if err := s.DeleteByTag(ctx, "g"); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryStore_ActiveSweepEvictsExpired(t *testing.T) {
	s := memory.NewStore(memory.WithSweepInterval(20 * time.Millisecond))
	defer s.Close()
	ctx := context.Background()

	_ = s.Set(ctx, "k", "v", xcache.WithTTL(10*time.Millisecond), xcache.WithTags("g"))

	time.Sleep(80 * time.Millisecond)

	if _, err := s.Get(ctx, "k"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("expected sweep to drop the key, got %v", err)
	}
	if err := s.DeleteByTag(ctx, "g"); err != nil {
		t.Fatalf("DeleteByTag should be a no-op after sweep, got %v", err)
	}
}

func TestMemoryStore_ClearWipesItemsAndTags(t *testing.T) {
	s := memory.NewStore()
	defer s.Close()
	ctx := context.Background()

	_ = s.Set(ctx, "k", "v", xcache.WithTags("g"))
	if err := s.Clear(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Get(ctx, "k"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after Clear, got %v", err)
	}

	_ = s.Set(ctx, "k2", "v2", xcache.WithTags("g"))
	if err := s.DeleteByTag(ctx, "g"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "k2"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("tag index should still work after Clear, got %v", err)
	}
}

func TestMemoryStore_DeleteMany(t *testing.T) {
	s := memory.NewStore()
	defer s.Close()
	ctx := context.Background()

	_ = s.Set(ctx, "a", 1)
	_ = s.Set(ctx, "b", 2)

	if err := s.DeleteMany(ctx, []string{"a", "b", "missing"}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"a", "b"} {
		if _, err := s.Get(ctx, k); !errors.Is(err, xcache.ErrNotFound) {
			t.Fatalf("expected %q deleted, got %v", k, err)
		}
	}
}

func TestMemoryStore_ShardingDistributesKeys(t *testing.T) {
	s := memory.NewStore(memory.WithShards(8))
	defer s.Close()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 1024; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			k := strconv.Itoa(i)
			_ = s.Set(ctx, k, i)
		}(i)
	}
	wg.Wait()

	for i := 0; i < 1024; i++ {
		entry, err := s.Get(ctx, strconv.Itoa(i))
		if err != nil {
			t.Fatalf("missing key %d: %v", i, err)
		}
		if entry.Value.(int) != i {
			t.Fatalf("wrong value for %d: %v", i, entry.Value)
		}
	}
}

func TestMemoryStore_DeleteByTag_IsolatesTags(t *testing.T) {
	s := memory.NewStore()
	defer s.Close()
	ctx := context.Background()

	_ = s.Set(ctx, "a", 1, xcache.WithTags("x"))
	_ = s.Set(ctx, "b", 2, xcache.WithTags("x", "y"))
	_ = s.Set(ctx, "c", 3, xcache.WithTags("y"))

	if err := s.DeleteByTag(ctx, "x"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Get(ctx, "a"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("a (tag x) should be gone")
	}
	if _, err := s.Get(ctx, "b"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("b (tag x,y) should be gone")
	}
	if _, err := s.Get(ctx, "c"); err != nil {
		t.Fatalf("c (tag y) should survive, got %v", err)
	}

	if err := s.DeleteByTag(ctx, "y"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "c"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("c should be gone after DeleteByTag(y)")
	}
}

func TestMemoryStore_OverwriteUpdatesTagIndex(t *testing.T) {
	s := memory.NewStore()
	defer s.Close()
	ctx := context.Background()

	_ = s.Set(ctx, "k", 1, xcache.WithTags("v1"))
	_ = s.Set(ctx, "k", 2, xcache.WithTags("v2"))

	if err := s.DeleteByTag(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "k"); err != nil {
		t.Fatalf("k should still exist (v1 was detached on overwrite), got %v", err)
	}

	if err := s.DeleteByTag(ctx, "v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "k"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("k should be gone after DeleteByTag(v2), got %v", err)
	}
}

func TestMemoryStore_GetMany_OmitsMissing(t *testing.T) {
	s := memory.NewStore()
	defer s.Close()
	ctx := context.Background()

	_ = s.Set(ctx, "a", 1)

	out, err := s.GetMany(ctx, []string{"a", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out))
	}
	if out["a"].Value != 1 {
		t.Fatalf("unexpected value: %v", out["a"].Value)
	}
}
