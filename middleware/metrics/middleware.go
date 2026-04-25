// Package metrics provides a Store middleware that instruments every cache
// operation via a user-supplied Recorder. The package carries no metric
// library dependency: callers implement Recorder to bridge any backend
// (Prometheus, StatsD, DataDog, …).
//
// Usage:
//
//	base := memory.NewStore()
//	store := metrics.Wrap(base, myRecorder)
//	defer store.Close()
//
// For every operation the middleware calls RecordDuration unconditionally,
// then RecordHit on success, RecordMiss on ErrNotFound (read ops only), or
// RecordError on any other error.
package metrics

import (
	"context"
	"errors"
	"time"

	xcache "github.com/giulio333/xcache"
)

// Recorder receives metric events from the middleware. op is the operation
// name ("Get", "Set", "Delete", "DeleteMany", "DeleteByTag", "Clear", "Close",
// "GetMany"). Implementations must be safe for concurrent use.
type Recorder interface {
	// RecordHit is called when an operation succeeds.
	RecordHit(op string)
	// RecordMiss is called when a read operation returns ErrNotFound.
	RecordMiss(op string)
	// RecordError is called when an operation fails with any error other
	// than ErrNotFound.
	RecordError(op string)
	// RecordDuration is called after every operation with its elapsed time.
	RecordDuration(op string, d time.Duration)
}

// metricsStore wraps a Store and emits metric events for every call.
type metricsStore struct {
	next xcache.Store
	rec  Recorder
}

// Wrap returns a Store that delegates all operations to next and records
// metrics via rec after each call.
func Wrap(next xcache.Store, rec Recorder) xcache.Store {
	return &metricsStore{next: next, rec: rec}
}

// record emits duration + outcome for a single-key (or keyless) operation.
// readOp controls whether ErrNotFound is treated as a miss (true) or error.
func (s *metricsStore) record(op string, start time.Time, err error, readOp bool) {
	s.rec.RecordDuration(op, time.Since(start))
	switch {
	case err == nil:
		s.rec.RecordHit(op)
	case readOp && errors.Is(err, xcache.ErrNotFound):
		s.rec.RecordMiss(op)
	default:
		s.rec.RecordError(op)
	}
}

func (s *metricsStore) Get(ctx context.Context, key string) (xcache.Entry, error) {
	start := time.Now()
	entry, err := s.next.Get(ctx, key)
	s.record("Get", start, err, true)
	return entry, err
}

func (s *metricsStore) Set(ctx context.Context, key string, value any, opts ...xcache.Option) error {
	start := time.Now()
	err := s.next.Set(ctx, key, value, opts...)
	s.record("Set", start, err, false)
	return err
}

func (s *metricsStore) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := s.next.Delete(ctx, key)
	s.record("Delete", start, err, false)
	return err
}

func (s *metricsStore) GetMany(ctx context.Context, keys []string) (map[string]xcache.Entry, error) {
	start := time.Now()
	result, err := s.next.GetMany(ctx, keys)
	s.record("GetMany", start, err, true)
	return result, err
}

func (s *metricsStore) DeleteMany(ctx context.Context, keys []string) error {
	start := time.Now()
	err := s.next.DeleteMany(ctx, keys)
	s.record("DeleteMany", start, err, false)
	return err
}

func (s *metricsStore) DeleteByTag(ctx context.Context, tag string) error {
	start := time.Now()
	err := s.next.DeleteByTag(ctx, tag)
	s.record("DeleteByTag", start, err, false)
	return err
}

func (s *metricsStore) Clear(ctx context.Context) error {
	start := time.Now()
	err := s.next.Clear(ctx)
	s.record("Clear", start, err, false)
	return err
}

func (s *metricsStore) Close() error {
	start := time.Now()
	err := s.next.Close()
	s.record("Close", start, err, false)
	return err
}
