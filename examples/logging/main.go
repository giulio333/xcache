// This example shows how to wrap a Store with the logging middleware to get
// structured log entries for every cache operation.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	xcache "github.com/giulio333/xcache"
	"github.com/giulio333/xcache/middleware/logging"
	"github.com/giulio333/xcache/store/memory"
)

type User struct {
	ID   int
	Name string
}

func main() {
	ctx := context.Background()

	// JSON logger to stdout — in production use slog.Default() or a custom handler.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Compose: memory store → logging middleware → Cache[User]
	store := logging.Wrap(memory.NewStore(), logger)
	defer store.Close()

	cache := xcache.New[User](store)

	// Set — logs "xcache op" op=Set
	_ = cache.Set(ctx, "user:1", User{ID: 1, Name: "Alice"}, xcache.WithTTL(10*time.Minute))

	// Get hit — logs "xcache op" op=Get
	u, err := cache.Get(ctx, "user:1")
	if err == nil {
		logger.Info("found", "user", u)
	}

	// Get miss — logs "xcache miss" op=Get (not an error)
	_, err = cache.Get(ctx, "user:99")
	if err != nil {
		logger.Info("not found (expected)", "err", err)
	}

	// DeleteByTag — logs "xcache op" op=DeleteByTag
	_ = cache.Set(ctx, "user:2", User{ID: 2, Name: "Bob"}, xcache.WithTags("team:eng"))
	_ = cache.DeleteByTag(ctx, "team:eng")

	// GetOrLoad with singleflight: concurrent callers on the same missing key
	// trigger the loader exactly once. All goroutines wait and receive the same
	// result, protecting the upstream data source from stampedes.
	var wg sync.WaitGroup
	loaderCalls := 0
	for range 5 {
		wg.Go(func() {
			val, loadErr := cache.GetOrLoad(ctx, "user:42", func(_ context.Context) (User, error) {
				loaderCalls++
				return User{ID: 42, Name: "Charlie"}, nil
			}, xcache.WithTTL(5*time.Minute))
			if loadErr == nil {
				_ = val
			}
		})
	}
	wg.Wait()
	// loaderCalls will be 1 even though 5 goroutines called GetOrLoad concurrently.
	fmt.Printf("loader invoked %d time(s) for 5 concurrent callers\n", loaderCalls)
}
