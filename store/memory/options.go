package memory

import "time"

const (
	defaultShards        = 64
	defaultSweepInterval = time.Minute
)

type StoreOptions struct {
	shards        uint64
	sweepInterval time.Duration
}

type StoreOption func(*StoreOptions)

func WithShards(n uint64) StoreOption {
	return func(o *StoreOptions) { o.shards = n }
}

func WithSweepInterval(d time.Duration) StoreOption {
	return func(o *StoreOptions) { o.sweepInterval = d }
}

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
