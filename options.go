package xcache

import "time"

// Options is the resolved configuration for a Set call. It is built from
// zero or more Option functions through ApplyOptions and is passed by
// backends to drive their write behaviour.
//
// Fields are exposed for use by Store implementations; user code should
// always go through the WithXxx constructors rather than building an
// Options literal directly.
type Options struct {
	// TTL is the time-to-live applied to the entry. A value of 0 means no
	// expiration: the entry is kept until it is explicitly deleted or
	// evicted by the backend.
	TTL time.Duration

	// Tags are labels associated with the entry. They are stored alongside
	// the value and used by DeleteByTag for group invalidation.
	Tags []string
}

// Option mutates an Options struct. It is the building block of the
// functional-options pattern used across the public Set / GetOrLoad APIs.
type Option func(*Options)

// WithTTL sets the time-to-live applied to the entry. A duration of 0 is
// equivalent to "no expiration" (the default).
//
//	cache.Set(ctx, "key", value, xcache.WithTTL(5*time.Minute))
func WithTTL(d time.Duration) Option {
	return func(o *Options) { o.TTL = d }
}

// WithTags attaches one or more tags to the entry. Tags can later be used
// with DeleteByTag to invalidate every entry that carries a given tag in a
// single call.
//
//	cache.Set(ctx, "user:1", u, xcache.WithTags("users", "admin"))
func WithTags(tags ...string) Option {
	return func(o *Options) { o.Tags = tags }
}

// ApplyOptions evaluates the given options in order and returns the
// resulting Options struct. It is intended to be called by Store
// implementations from inside their Set method.
func ApplyOptions(opts []Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
