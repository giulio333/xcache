package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	xcache "github.com/giulio333/xcache"
	"github.com/giulio333/xcache/middleware/logging"
	"github.com/giulio333/xcache/store/memory"
)

// newTestLogger returns a slog.Logger writing JSON to buf so tests can
// inspect log entries without any I/O.
func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// logEntries parses every JSON line in buf into a slice of maps.
func logEntries(buf *bytes.Buffer) []map[string]any {
	var entries []map[string]any
	dec := json.NewDecoder(buf)
	for dec.More() {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			break
		}
		entries = append(entries, m)
	}
	return entries
}

func TestGet_Hit(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "k1", "value", xcache.WithTTL(time.Minute))

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	_, err := store.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["op"] != "Get" {
		t.Errorf("expected op=Get, got %v", entries[0]["op"])
	}
	if entries[0]["level"] != "DEBUG" {
		t.Errorf("expected DEBUG level, got %v", entries[0]["level"])
	}
}

func TestGet_Miss(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	_, err := store.Get(ctx, "missing")
	if !errors.Is(err, xcache.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	// ErrNotFound must be DEBUG (miss), not ERROR.
	if entries[0]["level"] != "DEBUG" {
		t.Errorf("expected DEBUG level for miss, got %v", entries[0]["level"])
	}
	if entries[0]["msg"] != "xcache miss" {
		t.Errorf("expected msg=xcache miss, got %v", entries[0]["msg"])
	}
}

func TestSet_LogsDebug(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	if err := store.Set(ctx, "k2", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["op"] != "Set" {
		t.Errorf("expected op=Set, got %v", entries[0]["op"])
	}
	if entries[0]["level"] != "DEBUG" {
		t.Errorf("expected DEBUG, got %v", entries[0]["level"])
	}
}

func TestDelete_LogsDebug(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	if err := store.Delete(ctx, "k3"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) == 0 {
		t.Fatal("expected at least 1 log entry")
	}
	if entries[0]["op"] != "Delete" {
		t.Errorf("expected op=Delete, got %v", entries[0]["op"])
	}
}

func TestGetMany(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "a", 1)
	_ = base.Set(ctx, "b", 2)

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	result, err := store.GetMany(ctx, []string{"a", "b", "missing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["op"] != "GetMany" {
		t.Errorf("expected op=GetMany, got %v", entries[0]["op"])
	}
}

func TestDeleteMany(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "x", 1)
	_ = base.Set(ctx, "y", 2)

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	if err := store.DeleteMany(ctx, []string{"x", "y"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["op"] != "DeleteMany" {
		t.Errorf("expected op=DeleteMany, got %v", entries[0]["op"])
	}
}

func TestDeleteByTag(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	_ = base.Set(ctx, "t1", "val", xcache.WithTags("group1"))

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	if err := store.DeleteByTag(ctx, "group1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["op"] != "DeleteByTag" {
		t.Errorf("expected op=DeleteByTag, got %v", entries[0]["op"])
	}
}

func TestClear(t *testing.T) {
	ctx := context.Background()
	base := memory.NewStore()
	defer base.Close()

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	if err := store.Clear(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["op"] != "Clear" {
		t.Errorf("expected op=Clear, got %v", entries[0]["op"])
	}
}

func TestClose_Propagates(t *testing.T) {
	base := memory.NewStore()
	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	if err := store.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["op"] != "Close" {
		t.Errorf("expected op=Close, got %v", entries[0]["op"])
	}
}

func TestNilLogger_UsesDefault(t *testing.T) {
	base := memory.NewStore()
	defer base.Close()

	// Should not panic when logger is nil.
	store := logging.Wrap(base, nil)
	ctx := context.Background()
	_, _ = store.Get(ctx, "any")
}

// errStore is an inline Store stub that returns a controlled error for every
// write/mutation operation. It embeds a real MemoryStore so read paths work.
type errStore struct {
	xcache.Store
	err error
}

func (e *errStore) Set(_ context.Context, _ string, _ any, _ ...xcache.Option) error {
	return e.err
}
func (e *errStore) Clear(_ context.Context) error { return e.err }
func (e *errStore) Close() error                  { return e.err }
func (e *errStore) DeleteByTag(_ context.Context, _ string) error {
	return e.err
}
func (e *errStore) DeleteMany(_ context.Context, _ []string) error { return e.err }

func TestSet_Error_LogsError(t *testing.T) {
	ctx := context.Background()
	sentinel := errors.New("set failed")
	base := &errStore{Store: memory.NewStore(), err: sentinel}
	defer base.Store.Close()

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	err := store.Set(ctx, "k", "v")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["level"] != "ERROR" {
		t.Errorf("expected ERROR level, got %v", entries[0]["level"])
	}
	if entries[0]["error"] == nil {
		t.Error("expected 'error' field in log entry")
	}
}

func TestClear_Error_LogsError(t *testing.T) {
	ctx := context.Background()
	sentinel := errors.New("clear failed")
	base := &errStore{Store: memory.NewStore(), err: sentinel}
	defer base.Store.Close()

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	err := store.Clear(ctx)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["level"] != "ERROR" {
		t.Errorf("expected ERROR level, got %v", entries[0]["level"])
	}
	if entries[0]["error"] == nil {
		t.Error("expected 'error' field in log entry")
	}
}

func TestClose_Error_LogsError(t *testing.T) {
	sentinel := errors.New("close failed")
	base := &errStore{Store: memory.NewStore(), err: sentinel}

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	err := store.Close()
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["level"] != "ERROR" {
		t.Errorf("expected ERROR level, got %v", entries[0]["level"])
	}
	if entries[0]["error"] == nil {
		t.Error("expected 'error' field in log entry")
	}
}

func TestDeleteByTag_ErrNotSupported_LogsDebug(t *testing.T) {
	ctx := context.Background()
	base := &errStore{Store: memory.NewStore(), err: xcache.ErrNotSupported}
	defer base.Store.Close()

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	err := store.DeleteByTag(ctx, "some-tag")
	if !errors.Is(err, xcache.ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["level"] != "DEBUG" {
		t.Errorf("expected DEBUG level for ErrNotSupported, got %v", entries[0]["level"])
	}
	if entries[0]["msg"] != "xcache not supported" {
		t.Errorf("expected msg=xcache not supported, got %v", entries[0]["msg"])
	}
}

func TestDeleteMany_Error_LogsError(t *testing.T) {
	ctx := context.Background()
	sentinel := errors.New("delete many failed")
	base := &errStore{Store: memory.NewStore(), err: sentinel}
	defer base.Store.Close()

	var buf bytes.Buffer
	store := logging.Wrap(base, newTestLogger(&buf))

	err := store.DeleteMany(ctx, []string{"k1", "k2"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	entries := logEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0]["level"] != "ERROR" {
		t.Errorf("expected ERROR level, got %v", entries[0]["level"])
	}
	if entries[0]["error"] == nil {
		t.Error("expected 'error' field in log entry")
	}
}
