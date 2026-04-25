// This example shows how to wrap a Store with the metrics middleware using a
// simple Recorder that prints counters to stdout. No external metric library
// is required: swap printRecorder for your own Prometheus / StatsD / DataDog
// implementation in production.
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	xcache "github.com/giulio333/xcache"
	"github.com/giulio333/xcache/middleware/metrics"
	"github.com/giulio333/xcache/store/memory"
)

// printRecorder implements metrics.Recorder by printing each event to stdout.
// A real implementation would call prometheus.CounterVec.WithLabelValues(op).Inc()
// or similar.
type printRecorder struct {
	mu   sync.Mutex
	hits map[string]int
}

func newPrintRecorder() *printRecorder {
	return &printRecorder{hits: make(map[string]int)}
}

func (r *printRecorder) RecordHit(op string) {
	r.mu.Lock()
	r.hits[op]++
	r.mu.Unlock()
	fmt.Printf("[metrics] hit   op=%s\n", op)
}

func (r *printRecorder) RecordMiss(op string) {
	fmt.Printf("[metrics] miss  op=%s\n", op)
}

func (r *printRecorder) RecordError(op string) {
	fmt.Printf("[metrics] error op=%s\n", op)
}

func (r *printRecorder) RecordDuration(op string, d time.Duration) {
	fmt.Printf("[metrics] dur   op=%s duration=%s\n", op, d.Round(time.Microsecond))
}

type Product struct {
	ID    int
	Name  string
	Price float64
}

func main() {
	ctx := context.Background()

	rec := newPrintRecorder()

	// Compose: memory store → metrics middleware → Cache[Product]
	store := metrics.Wrap(memory.NewStore(), rec)
	defer store.Close()

	cache := xcache.New[Product](store)

	// Set — RecordHit("Set") + RecordDuration("Set")
	_ = cache.Set(ctx, "product:1", Product{ID: 1, Name: "Widget", Price: 9.99},
		xcache.WithTTL(10*time.Minute))

	// Get hit — RecordHit("Get") + RecordDuration("Get")
	p, err := cache.Get(ctx, "product:1")
	if err == nil {
		fmt.Printf("found: %+v\n", p)
	}

	// Get miss — RecordMiss("Get") + RecordDuration("Get")
	_, err = cache.Get(ctx, "product:99")
	if err != nil {
		fmt.Println("miss (expected):", err)
	}

	// DeleteByTag — RecordHit("DeleteByTag") + RecordDuration("DeleteByTag")
	_ = cache.Set(ctx, "product:2", Product{ID: 2, Name: "Gadget", Price: 19.99},
		xcache.WithTags("category:electronics"))
	_ = cache.DeleteByTag(ctx, "category:electronics")

	fmt.Println("\nhit counters:", rec.hits)
}
