package xcache_test

import (
	"context"
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
	if err != xcache.ErrNotFound {
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
	if err != xcache.ErrNotFound {
		t.Fatalf("expected ErrNotFound after TTL, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	c := NewCache()
	_ = c.Set(context.Background(), "u1", User{ID: 1})
	_ = c.Delete(context.Background(), "u1")

	_, err := c.Get(context.Background(), "u1")
	if err != xcache.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
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
}

func TestDeleteMany(t *testing.T) {
	c := NewCache()
	_ = c.Set(context.Background(), "u1", User{ID: 1})
	_ = c.Set(context.Background(), "u2", User{ID: 2})
	_ = c.DeleteMany(context.Background(), []string{"u1", "u2"})

	_, err1 := c.Get(context.Background(), "u1")
	_, err2 := c.Get(context.Background(), "u2")
	if err1 != xcache.ErrNotFound || err2 != xcache.ErrNotFound {
		t.Fatal("expected both keys deleted")
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
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.GetOrLoad(context.Background(), "u1", loader)
		}()
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

	// Scrivi solo su L2 con TTL breve
	_ = l2.Set(ctx, "u1", User{ID: 1}, xcache.WithTTL(100*time.Millisecond))

	// Get: miss su L1, hit su L2 → write-back su L1 con TTL residuo
	got, err := c.Get(ctx, "u1")
	if err != nil {
		t.Fatalf("expected hit, got %v", err)
	}
	if got.ID != 1 {
		t.Fatalf("unexpected value: %v", got)
	}

	// Aspetta la scadenza del TTL originale
	time.Sleep(150 * time.Millisecond)

	// L1 deve aver già scaduto la chiave — non deve sopravvivere oltre il TTL di L2
	_, err = l1.Get(ctx, "u1")
	if err != xcache.ErrNotFound {
		t.Fatal("L1 should have expired the key after the original TTL")
	}
}

func TestCache_TypeMismatchReturnsError(t *testing.T) {
	store := memory.NewStore()
	ctx := context.Background()

	// Scrivi un int nello store raw (bypassando i generics)
	_ = store.Set(ctx, "k1", 42)

	// Leggi con Cache[User]: il tipo non corrisponde → errore, non panic
	c := xcache.New[User](store)
	_, err := c.Get(ctx, "k1")
	if err == nil {
		t.Fatal("expected type mismatch error, got nil")
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
