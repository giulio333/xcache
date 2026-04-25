package xcache_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/giulio333/xcache"
	"github.com/giulio333/xcache/store/memory"
)

type User struct {
	ID   int
	Name string
}

func NewCache() xcache.Cache[User] {
	return xcache.New[User](memory.NewStore())
}

func TestGet_NotFound(t *testing.T) {
	c := NewCache()
	_, err := c.Get(context.Background(), "missing")
	if !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSetAndGet(t *testing.T) {
	c := NewCache()
	want := User{ID: 1, Name: "Alice"}

	_ = c.Set(context.Background(), "u1", want)
	got, err := c.Get(context.Background(), "u1")

	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSet_TTLExpired(t *testing.T) {
	c := NewCache()
	_ = c.Set(context.Background(), "u1", User{ID: 1}, xcache.WithTTL(50*time.Millisecond))

	time.Sleep(100 * time.Millisecond)

	_, err := c.Get(context.Background(), "u1")
	if !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after TTL, got %v", err)
	}
}

func TestSet_OverwritePreservesValue(t *testing.T) {
	c := NewCache()
	ctx := context.Background()

	_ = c.Set(ctx, "u1", User{ID: 1, Name: "Alice"})
	_ = c.Set(ctx, "u1", User{ID: 1, Name: "Updated"})

	got, err := c.Get(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Updated" {
		t.Fatalf("expected overwrite to win, got %v", got)
	}
}

func TestDelete(t *testing.T) {
	c := NewCache()
	_ = c.Set(context.Background(), "u1", User{ID: 1})
	_ = c.Delete(context.Background(), "u1")

	_, err := c.Get(context.Background(), "u1")
	if !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDelete_MissingKeyIsNoop(t *testing.T) {
	c := NewCache()
	if err := c.Delete(context.Background(), "absent"); err != nil {
		t.Fatalf("Delete on missing key should be a no-op, got %v", err)
	}
}

func TestClear(t *testing.T) {
	c := NewCache()
	ctx := context.Background()

	_ = c.Set(ctx, "u1", User{ID: 1})
	_ = c.Set(ctx, "u2", User{ID: 2})

	if err := c.Clear(ctx); err != nil {
		t.Fatal(err)
	}

	for _, k := range []string{"u1", "u2"} {
		if _, err := c.Get(ctx, k); !errors.Is(err, xcache.ErrNotFound) {
			t.Fatalf("expected %q cleared, got %v", k, err)
		}
	}
}

func TestGetMany(t *testing.T) {
	c := NewCache()
	_ = c.Set(context.Background(), "u1", User{ID: 1})
	_ = c.Set(context.Background(), "u2", User{ID: 2})

	result, err := c.GetMany(context.Background(), []string{"u1", "u2", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result["u1"].ID != 1 || result["u2"].ID != 2 {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestDeleteMany(t *testing.T) {
	c := NewCache()
	_ = c.Set(context.Background(), "u1", User{ID: 1})
	_ = c.Set(context.Background(), "u2", User{ID: 2})
	_ = c.DeleteMany(context.Background(), []string{"u1", "u2"})

	_, err1 := c.Get(context.Background(), "u1")
	_, err2 := c.Get(context.Background(), "u2")
	if !errors.Is(err1, xcache.ErrNotFound) || !errors.Is(err2, xcache.ErrNotFound) {
		t.Fatal("expected both keys deleted")
	}
}

func TestDeleteByTag(t *testing.T) {
	c := NewCache()
	ctx := context.Background()

	_ = c.Set(ctx, "u1", User{ID: 1}, xcache.WithTags("users", "admin"))
	_ = c.Set(ctx, "u2", User{ID: 2}, xcache.WithTags("users"))
	_ = c.Set(ctx, "p1", User{ID: 3}, xcache.WithTags("products"))

	if err := c.DeleteByTag(ctx, "users"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.Get(ctx, "u1"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("u1 should be gone after DeleteByTag(users), got %v", err)
	}
	if _, err := c.Get(ctx, "u2"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("u2 should be gone after DeleteByTag(users), got %v", err)
	}
	if _, err := c.Get(ctx, "p1"); err != nil {
		t.Fatalf("p1 (untagged with users) should survive, got %v", err)
	}
}

func TestDeleteByTag_UnknownTagIsNoop(t *testing.T) {
	c := NewCache()
	ctx := context.Background()

	_ = c.Set(ctx, "u1", User{ID: 1}, xcache.WithTags("users"))

	if err := c.DeleteByTag(ctx, "does-not-exist"); err != nil {
		t.Fatalf("DeleteByTag on unknown tag should be a no-op, got %v", err)
	}

	if _, err := c.Get(ctx, "u1"); err != nil {
		t.Fatalf("u1 should survive a no-op DeleteByTag, got %v", err)
	}
}

func TestDeleteByTag_StaleIndexAfterOverwrite(t *testing.T) {
	c := NewCache()
	ctx := context.Background()

	_ = c.Set(ctx, "u1", User{ID: 1}, xcache.WithTags("v1"))
	_ = c.Set(ctx, "u1", User{ID: 1}, xcache.WithTags("v2"))

	if err := c.DeleteByTag(ctx, "v1"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.Get(ctx, "u1"); err != nil {
		t.Fatalf("overwrite should have detached u1 from v1, got %v", err)
	}

	if err := c.DeleteByTag(ctx, "v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(ctx, "u1"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("u1 should be gone after DeleteByTag(v2), got %v", err)
	}
}

func TestGetOrLoad_CallsLoader(t *testing.T) {
	c := NewCache()
	called := false

	user, err := c.GetOrLoad(context.Background(), "u1", func(ctx context.Context) (User, error) {
		called = true
		return User{ID: 1, Name: "Alice"}, nil
	})

	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("loader should have been called")
	}
	if user.ID != 1 {
		t.Fatalf("unexpected user: %v", user)
	}
}

func TestGetOrLoad_CacheHit(t *testing.T) {
	c := NewCache()
	_ = c.Set(context.Background(), "u1", User{ID: 1})

	calls := 0
	_, _ = c.GetOrLoad(context.Background(), "u1", func(ctx context.Context) (User, error) {
		calls++
		return User{}, nil
	})

	if calls != 0 {
		t.Fatal("loader should not be called on cache hit")
	}
}

func TestGetOrLoad_LoaderError(t *testing.T) {
	c := NewCache()
	wantErr := errors.New("upstream down")

	_, err := c.GetOrLoad(context.Background(), "u1", func(ctx context.Context) (User, error) {
		return User{}, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected loader error to bubble up, got %v", err)
	}
}

func TestGetOrLoad_PersistsTTL(t *testing.T) {
	c := NewCache()
	ctx := context.Background()

	_, err := c.GetOrLoad(ctx, "u1", func(ctx context.Context) (User, error) {
		return User{ID: 1}, nil
	}, xcache.WithTTL(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.Get(ctx, "u1"); err != nil {
		t.Fatalf("loaded value should be cached, got %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	if _, err := c.Get(ctx, "u1"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("loaded value should expire, got %v", err)
	}
}

func TestGetOrLoad_Singleflight(t *testing.T) {
	c := NewCache()
	var calls atomic.Int32

	loader := func(ctx context.Context) (User, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return User{ID: 1, Name: "Alice"}, nil
	}

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			_, _ = c.GetOrLoad(context.Background(), "u1", loader)
		})
	}
	wg.Wait()

	if calls.Load() != 1 {
		t.Fatalf("loader called %d times, expected 1", calls.Load())
	}
}

func TestChainStore_TTLPreservedOnWriteBack(t *testing.T) {
	l1 := memory.NewStore()
	l2 := memory.NewStore()
	chain := xcache.NewChain(l1, l2)
	c := xcache.New[User](chain)
	ctx := context.Background()

	_ = l2.Set(ctx, "u1", User{ID: 1}, xcache.WithTTL(100*time.Millisecond))

	got, err := c.Get(ctx, "u1")
	if err != nil {
		t.Fatalf("expected hit, got %v", err)
	}
	if got.ID != 1 {
		t.Fatalf("unexpected value: %v", got)
	}

	time.Sleep(150 * time.Millisecond)

	_, err = l1.Get(ctx, "u1")
	if !errors.Is(err, xcache.ErrNotFound) {
		t.Fatal("L1 should have expired the key after the original TTL")
	}
}

func TestChainStore_BackfillPropagatesTags(t *testing.T) {
	l1 := memory.NewStore()
	l2 := memory.NewStore()
	chain := xcache.NewChain(l1, l2)
	c := xcache.New[User](chain)
	ctx := context.Background()

	_ = l2.Set(ctx, "u1", User{ID: 1}, xcache.WithTags("users"))

	if _, err := c.Get(ctx, "u1"); err != nil {
		t.Fatalf("expected hit through chain, got %v", err)
	}

	if err := c.DeleteByTag(ctx, "users"); err != nil {
		t.Fatal(err)
	}
	if _, err := l1.Get(ctx, "u1"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("L1 should drop the back-filled tagged entry, got %v", err)
	}
	if _, err := l2.Get(ctx, "u1"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("L2 should drop the original tagged entry, got %v", err)
	}
}

func TestChainStore_GetMany_AcrossLayers(t *testing.T) {
	l1 := memory.NewStore()
	l2 := memory.NewStore()
	c := xcache.New[User](xcache.NewChain(l1, l2))
	ctx := context.Background()

	_ = l1.Set(ctx, "u1", User{ID: 1})
	_ = l2.Set(ctx, "u2", User{ID: 2})

	out, err := c.GetMany(ctx, []string{"u1", "u2", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out["u1"].ID != 1 || out["u2"].ID != 2 {
		t.Fatalf("unexpected result: %v", out)
	}

	if _, err := l1.Get(ctx, "u2"); err != nil {
		t.Fatalf("u2 should have been back-filled into L1, got %v", err)
	}
}

func TestChainStore_DeleteByTag_PropagatesToAllLayers(t *testing.T) {
	l1 := memory.NewStore()
	l2 := memory.NewStore()
	chain := xcache.NewChain(l1, l2)
	ctx := context.Background()

	_ = l1.Set(ctx, "u1", User{ID: 1}, xcache.WithTags("users"))
	_ = l2.Set(ctx, "u1", User{ID: 1}, xcache.WithTags("users"))
	_ = l2.Set(ctx, "u2", User{ID: 2}, xcache.WithTags("users"))

	if err := chain.DeleteByTag(ctx, "users"); err != nil {
		t.Fatal(err)
	}

	for _, layer := range []*memory.MemoryStore{l1, l2} {
		for _, k := range []string{"u1", "u2"} {
			if _, err := layer.Get(ctx, k); !errors.Is(err, xcache.ErrNotFound) {
				t.Fatalf("expected %q deleted from every layer, got %v", k, err)
			}
		}
	}
}

func TestCache_TypeMismatchReturnsError(t *testing.T) {
	store := memory.NewStore()
	ctx := context.Background()

	_ = store.Set(ctx, "k1", 42)

	c := xcache.New[User](store)
	_, err := c.Get(ctx, "k1")
	if err == nil {
		t.Fatal("expected type mismatch error, got nil")
	}
}

func TestCache_GetMany_TypeMismatchReturnsError(t *testing.T) {
	store := memory.NewStore()
	ctx := context.Background()

	_ = store.Set(ctx, "k1", 42)

	c := xcache.New[User](store)
	_, err := c.GetMany(ctx, []string{"k1"})
	if err == nil {
		t.Fatal("expected type mismatch error from GetMany, got nil")
	}
}

func TestEntry_RemainingTTL(t *testing.T) {
	t.Run("zero ExpiresAt means no expiration", func(t *testing.T) {
		e := xcache.Entry{}
		if got := e.RemainingTTL(); got != 0 {
			t.Fatalf("expected 0, got %v", got)
		}
	})

	t.Run("past deadline returns 0", func(t *testing.T) {
		e := xcache.Entry{ExpiresAt: time.Now().Add(-time.Second)}
		if got := e.RemainingTTL(); got != 0 {
			t.Fatalf("expected 0 for past deadline, got %v", got)
		}
	})

	t.Run("future deadline returns positive duration", func(t *testing.T) {
		e := xcache.Entry{ExpiresAt: time.Now().Add(time.Minute)}
		if got := e.RemainingTTL(); got <= 0 {
			t.Fatalf("expected positive duration, got %v", got)
		}
	})
}

// — WithPrefix tests —

func TestWithPrefix_GetSet(t *testing.T) {
	store := memory.NewStore()
	c := xcache.New[User](store, xcache.WithPrefix("users:"))
	ctx := context.Background()

	want := User{ID: 1, Name: "Alice"}
	if err := c.Set(ctx, "1", want); err != nil {
		t.Fatal(err)
	}

	// Value must be reachable through the prefixed cache.
	got, err := c.Get(ctx, "1")
	if err != nil || got != want {
		t.Fatalf("Get returned (%v, %v), want (%v, nil)", got, err, want)
	}

	// The Store holds the prefixed key, not the bare key.
	if _, err := store.Get(ctx, "1"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatal("bare key must not exist in store")
	}
	if _, err := store.Get(ctx, "users:1"); err != nil {
		t.Fatalf("prefixed key must exist in store, got %v", err)
	}
}

func TestWithPrefix_Delete(t *testing.T) {
	store := memory.NewStore()
	c := xcache.New[User](store, xcache.WithPrefix("users:"))
	ctx := context.Background()

	_ = c.Set(ctx, "1", User{ID: 1})
	_ = c.Delete(ctx, "1")

	if _, err := c.Get(ctx, "1"); !errors.Is(err, xcache.ErrNotFound) {
		t.Fatal("key should be deleted")
	}
}

func TestWithPrefix_GetMany_KeysStripped(t *testing.T) {
	store := memory.NewStore()
	c := xcache.New[User](store, xcache.WithPrefix("users:"))
	ctx := context.Background()

	_ = c.Set(ctx, "1", User{ID: 1})
	_ = c.Set(ctx, "2", User{ID: 2})

	result, err := c.GetMany(ctx, []string{"1", "2", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	// Keys in the returned map must be the original, unprefixed keys.
	if _, ok := result["1"]; !ok {
		t.Fatal("result must contain key \"1\", not \"users:1\"")
	}
	if _, ok := result["2"]; !ok {
		t.Fatal("result must contain key \"2\", not \"users:2\"")
	}
}

func TestWithPrefix_DeleteMany(t *testing.T) {
	store := memory.NewStore()
	c := xcache.New[User](store, xcache.WithPrefix("users:"))
	ctx := context.Background()

	_ = c.Set(ctx, "1", User{ID: 1})
	_ = c.Set(ctx, "2", User{ID: 2})
	_ = c.DeleteMany(ctx, []string{"1", "2"})

	for _, k := range []string{"1", "2"} {
		if _, err := c.Get(ctx, k); !errors.Is(err, xcache.ErrNotFound) {
			t.Fatalf("key %q should be deleted", k)
		}
	}
}

func TestWithPrefix_GetOrLoad(t *testing.T) {
	store := memory.NewStore()
	c := xcache.New[User](store, xcache.WithPrefix("users:"))
	ctx := context.Background()

	calls := 0
	got, err := c.GetOrLoad(ctx, "1", func(ctx context.Context) (User, error) {
		calls++
		return User{ID: 1, Name: "Alice"}, nil
	})
	if err != nil || got.ID != 1 {
		t.Fatalf("unexpected result (%v, %v)", got, err)
	}
	if calls != 1 {
		t.Fatalf("loader should be called once, got %d", calls)
	}

	// Second call must be a cache hit.
	_, _ = c.GetOrLoad(ctx, "1", func(ctx context.Context) (User, error) {
		calls++
		return User{}, nil
	})
	if calls != 1 {
		t.Fatal("loader must not be called on cache hit")
	}

	// Underlying store must hold the prefixed key.
	if _, err := store.Get(ctx, "users:1"); err != nil {
		t.Fatalf("prefixed key must be stored, got %v", err)
	}
}

func TestWithPrefix_EmptyPrefix_NoChange(t *testing.T) {
	store := memory.NewStore()
	c := xcache.New[User](store, xcache.WithPrefix(""))
	ctx := context.Background()

	_ = c.Set(ctx, "k", User{ID: 99})

	if _, err := store.Get(ctx, "k"); err != nil {
		t.Fatalf("empty prefix: key must be stored as-is, got %v", err)
	}
}

func BenchmarkSet(b *testing.B) {
	c := NewCache()
	ctx := context.Background()
	u := User{ID: 1, Name: "Alice"}
	b.ResetTimer()
	var i atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = c.Set(ctx, strconv.FormatInt(i.Add(1), 10), u)
		}
	})
}

func BenchmarkGet_Hit(b *testing.B) {
	c := NewCache()
	ctx := context.Background()
	const keys = 1024
	for i := range keys {
		_ = c.Set(ctx, strconv.Itoa(i), User{ID: i})
	}
	var i atomic.Int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = c.Get(ctx, strconv.FormatInt(i.Add(1)%keys, 10))
		}
	})
}

func BenchmarkGetOrLoad_Hit(b *testing.B) {
	c := NewCache()
	ctx := context.Background()
	const keys = 1024
	for i := range keys {
		_ = c.Set(ctx, strconv.Itoa(i), User{ID: i})
	}
	loader := func(ctx context.Context) (User, error) { return User{ID: 1}, nil }
	var i atomic.Int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = c.GetOrLoad(ctx, strconv.FormatInt(i.Add(1)%keys, 10), loader)
		}
	})
}

func BenchmarkDeleteByTag(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		c := NewCache()
		ctx := context.Background()
		for j := range 1024 {
			_ = c.Set(ctx, strconv.Itoa(j), User{ID: j}, xcache.WithTags("group"))
		}
		b.StartTimer()
		_ = c.DeleteByTag(ctx, "group")
	}
}
