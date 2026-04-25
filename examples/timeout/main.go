// Package main demonstrates using the timeout middleware to enforce a
// per-operation deadline on a cache store. This is useful when the backing
// store (e.g. Redis) may become slow under load and you need a hard upper
// bound on how long a single cache call is allowed to block.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	xcache "github.com/giulio333/xcache"
	"github.com/giulio333/xcache/middleware/timeout"
	"github.com/giulio333/xcache/store/memory"
)

func main() {
	ctx := context.Background()

	base := memory.NewStore()
	defer base.Close()

	// Every operation gets at most 50 ms to complete.
	ts := timeout.Wrap(base, 50*time.Millisecond)

	cache := xcache.New[string](ts)

	// Normal operations complete well within the deadline.
	if err := cache.Set(ctx, "greeting", "hello", xcache.WithTTL(time.Minute)); err != nil {
		fmt.Printf("set error: %v\n", err)
		return
	}

	val, err := cache.Get(ctx, "greeting")
	if err != nil {
		fmt.Printf("get error: %v\n", err)
		return
	}
	fmt.Printf("greeting = %s\n", val)

	// Passing d <= 0 makes Wrap a no-op — the original store is returned.
	passthrough := timeout.Wrap(base, 0)
	if passthrough == base {
		fmt.Println("d=0: no-op, original store returned")
	}

	// When the caller's own deadline is tighter than the middleware's, the
	// caller's deadline takes effect first — context.WithTimeout picks the
	// minimum of the two.
	tightCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()

	_, err = cache.Get(tightCtx, "greeting")
	if errors.Is(err, context.DeadlineExceeded) {
		fmt.Println("caller's tight deadline fired: DeadlineExceeded")
	}
}
