package xcache

import "time"

type Options struct {
	TTL  time.Duration // 0 means no expiration
	Tags []string      // Invalidates all keys with any of these tags when deleted
}

type Option func(*Options)

func WithTTL(d time.Duration) Option {
	return func(o *Options) { o.TTL = d }
}

func WithTags(tags ...string) Option {
	return func(o *Options) { o.Tags = tags }
}

// applyOptions applies the given options and returns the resulting Options struct.
func ApplyOptions(opts []Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
