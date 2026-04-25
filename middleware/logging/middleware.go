// Package logging provides a Store middleware that logs every cache operation
// using the standard library's log/slog package.
//
// Usage:
//
//	base := memory.NewStore()
//	logged := logging.Wrap(base, slog.Default())
//	defer logged.Close()
//
// Each operation is logged at Debug level with the fields "op", "key" (or
// "keys" for batch operations), and "duration". Errors are logged at Error
// level; ErrNotFound is treated as a cache miss and logged at Debug level so
// that it does not pollute production logs.
package logging

import (
	"context"
	"errors"
	"log/slog"
	"time"

	xcache "github.com/giulio333/xcache"
)

// loggingStore wraps a Store and emits structured log entries for every call.
type loggingStore struct {
	next   xcache.Store
	logger *slog.Logger
}

// Wrap returns a Store that delegates all operations to next and logs each
// one. If logger is nil, slog.Default() is used.
func Wrap(next xcache.Store, logger *slog.Logger) xcache.Store {
	if logger == nil {
		logger = slog.Default()
	}
	return &loggingStore{next: next, logger: logger}
}

func (s *loggingStore) logOp(op, key string, start time.Time, err error) {
	dur := time.Since(start)
	if err == nil {
		s.logger.Debug("xcache op", "op", op, "key", key, "duration", dur)
		return
	}
	if errors.Is(err, xcache.ErrNotFound) {
		s.logger.Debug("xcache miss", "op", op, "key", key, "duration", dur)
		return
	}
	s.logger.Error("xcache error", "op", op, "key", key, "duration", dur, "error", err)
}

func (s *loggingStore) logBatchOp(op string, keys []string, start time.Time, err error) {
	dur := time.Since(start)
	if err == nil {
		s.logger.Debug("xcache op", "op", op, "keys", keys, "duration", dur)
		return
	}
	s.logger.Error("xcache error", "op", op, "keys", keys, "duration", dur, "error", err)
}

func (s *loggingStore) Get(ctx context.Context, key string) (xcache.Entry, error) {
	start := time.Now()
	entry, err := s.next.Get(ctx, key)
	s.logOp("Get", key, start, err)
	return entry, err
}

func (s *loggingStore) Set(ctx context.Context, key string, value any, opts ...xcache.Option) error {
	start := time.Now()
	err := s.next.Set(ctx, key, value, opts...)
	s.logOp("Set", key, start, err)
	return err
}

func (s *loggingStore) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := s.next.Delete(ctx, key)
	s.logOp("Delete", key, start, err)
	return err
}

func (s *loggingStore) Clear(ctx context.Context) error {
	start := time.Now()
	err := s.next.Clear(ctx)
	dur := time.Since(start)
	if err != nil {
		s.logger.Error("xcache error", "op", "Clear", "duration", dur, "error", err)
	} else {
		s.logger.Debug("xcache op", "op", "Clear", "duration", dur)
	}
	return err
}

func (s *loggingStore) Close() error {
	err := s.next.Close()
	if err != nil {
		s.logger.Error("xcache error", "op", "Close", "error", err)
	} else {
		s.logger.Debug("xcache op", "op", "Close")
	}
	return err
}

func (s *loggingStore) GetMany(ctx context.Context, keys []string) (map[string]xcache.Entry, error) {
	start := time.Now()
	result, err := s.next.GetMany(ctx, keys)
	s.logBatchOp("GetMany", keys, start, err)
	return result, err
}

func (s *loggingStore) DeleteMany(ctx context.Context, keys []string) error {
	start := time.Now()
	err := s.next.DeleteMany(ctx, keys)
	s.logBatchOp("DeleteMany", keys, start, err)
	return err
}

func (s *loggingStore) DeleteByTag(ctx context.Context, tag string) error {
	start := time.Now()
	err := s.next.DeleteByTag(ctx, tag)
	dur := time.Since(start)
	if err == nil {
		s.logger.Debug("xcache op", "op", "DeleteByTag", "tag", tag, "duration", dur)
		return nil
	}
	if errors.Is(err, xcache.ErrNotSupported) {
		s.logger.Debug("xcache not supported", "op", "DeleteByTag", "tag", tag, "duration", dur)
		return err
	}
	s.logger.Error("xcache error", "op", "DeleteByTag", "tag", tag, "duration", dur, "error", err)
	return err
}
