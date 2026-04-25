package memory

import "time"

const (
	// defaultShards is the number of shards used when WithShards is not
	// supplied. 64 is enough to keep contention low under typical workloads
	// while keeping memory overhead modest.
	defaultShards = 64

	// defaultSweepInterval is the cadence at which the background sweeper
	// drops expired entries when WithSweepInterval is not supplied.
	defaultSweepInterval = time.Minute
)

// StoreOptions captures the configurable parameters of a MemoryStore. It is
// populated through StoreOption functions (WithShards, WithSweepInterval).
type StoreOptions struct {
	shards        uint64
	sweepInterval time.Duration
}

// StoreOption mutates a StoreOptions struct. It is the building block of
// the functional-options pattern used by NewStore.
type StoreOption func(*StoreOptions)

// WithShards overrides the number of shards used by the store. More shards
// reduce contention on highly concurrent workloads at the cost of more
// allocations (each shard owns its own maps).
func WithShards(n uint64) StoreOption {
	return func(o *StoreOptions) { o.shards = n }
}

// WithSweepInterval overrides the cadence of the background expiration
// sweep. Shorter intervals reclaim memory sooner at the cost of more
// background CPU. Use shorter intervals only on workloads with a high
// write rate, short TTLs and few re-reads.
func WithSweepInterval(d time.Duration) StoreOption {
	return func(o *StoreOptions) { o.sweepInterval = d }
}

// applyStoreOptions evaluates the given options against the package
// defaults and returns the resulting StoreOptions.
func applyStoreOptions(opts []StoreOption) *StoreOptions {
	o := &StoreOptions{
		shards:        defaultShards,
		sweepInterval: defaultSweepInterval,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
