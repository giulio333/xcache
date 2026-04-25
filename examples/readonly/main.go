// Package main demonstrates using the readonly middleware to expose a
// pre-populated cache as a read-only view. This is useful in staging
// environments where a shared cache is seeded once from production data and
// then served to multiple services that must not modify it.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	xcache "github.com/giulio333/xcache"
	"github.com/giulio333/xcache/middleware/readonly"
	"github.com/giulio333/xcache/store/memory"
)

func main() {
	ctx := context.Background()

	// Seed a store with production-like data.
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "product:42", "Widget Pro", xcache.WithTTL(time.Hour))
	_ = base.Set(ctx, "product:99", "Gadget Plus", xcache.WithTTL(time.Hour))

	// Wrap it as read-only before handing it to staging services.
	ro := readonly.Wrap(base)

	// Read operations work as expected.
	cache := xcache.New[string](ro)

	name, err := cache.Get(ctx, "product:42")
	if err != nil {
		fmt.Printf("get error: %v\n", err)
		return
	}
	fmt.Printf("product:42 = %s\n", name)

	// Write operations are blocked and return ErrReadOnly.
	err = cache.Set(ctx, "product:42", "Widget Pro v2")
	if errors.Is(err, readonly.ErrReadOnly) {
		fmt.Println("write rejected: store is read-only")
	}

	// Deletions are blocked too.
	err = cache.Delete(ctx, "product:99")
	if errors.Is(err, readonly.ErrReadOnly) {
		fmt.Println("delete rejected: store is read-only")
	}
}
