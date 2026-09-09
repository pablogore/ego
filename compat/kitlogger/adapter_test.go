// MIT License
//
// Copyright (c) 2022-2026 Arsene Tochemey Gandote
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package kitlogger_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	kitlogger "github.com/pablogore/kit-logger/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tochemey/ego/v4"

	egokit "github.com/tochemey/ego/v4/compat/kitlogger"
)

// record is one captured log entry, kept with the exact context the handler
// was called with so a test can assert on context identity rather than on a
// copy that merely compares equal.
type record struct {
	ctx   context.Context
	level slog.Level
	msg   string
	attrs []slog.Attr
}

// captureHandler is a slog.Handler that records everything it is handed and
// gates on a configurable minimum level. Passing it to kit-logger as
// Config.Handler bypasses kit-logger's own handler chain, so a test observes
// exactly what the adapter forwarded — no Prometheus counter, no caller-frame
// walk, and no "Logger initialized" record of kit-logger's own.
type captureHandler struct {
	mu       *sync.Mutex
	records  *[]record
	minLevel *slog.LevelVar
	attrs    []slog.Attr
	groups   []string
}

func newCaptureHandler(minLevel slog.Level) *captureHandler {
	lv := new(slog.LevelVar)
	lv.Set(minLevel)
	return &captureHandler{mu: new(sync.Mutex), records: new([]record), minLevel: lv}
}

func (h *captureHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.minLevel.Level()
}

func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())
	attrs = append(attrs, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, record{ctx: ctx, level: r.Level, msg: r.Message, attrs: attrs})
	return nil
}

// WithAttrs accumulates the attributes a child logger carries, in the order
// they were added, which is what makes the nested-With ordering assertions
// meaningful.
func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	child := *h
	child.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &child
}

func (h *captureHandler) WithGroup(name string) slog.Handler {
	child := *h
	child.groups = append(append([]string{}, h.groups...), name)
	return &child
}

func (h *captureHandler) captured() []record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]record{}, *h.records...)
}

// only returns the single captured record, failing the test when the count is
// anything but one.
func (h *captureHandler) only(t *testing.T) record {
	t.Helper()
	got := h.captured()
	require.Len(t, got, 1)
	return got[0]
}

// attrPairs flattens a record's attributes to key/value pairs, so an assertion
// can state the expected fields and their order in one literal.
func attrPairs(r record) []any {
	pairs := make([]any, 0, len(r.attrs)*2)
	for _, a := range r.attrs {
		pairs = append(pairs, a.Key, a.Value.Any())
	}
	return pairs
}

// newBackend builds a kit-logger backed by a capture handler, and returns both.
func newBackend(minLevel slog.Level) (kitlogger.Logger, *captureHandler) {
	h := newCaptureHandler(minLevel)
	return kitlogger.New(kitlogger.Config{Handler: h}), h
}

// TestCapabilityMatrix documents, as executable fact, exactly which eGo
// capabilities kit-logger satisfies on its own and which ones only the adapter
// supplies. The negative assertions are the point: they are what makes this
// package necessary, and they will fail loudly if kit-logger ever grows the
// methods, at which point the adapter can be reconsidered.
func TestCapabilityMatrix(t *testing.T) {
	backend, _ := newBackend(slog.LevelDebug)

	t.Run("kit-logger satisfies the capabilities Go lets it satisfy", func(t *testing.T) {
		// Identical method signatures, so no adaptation is needed for these.
		assert.Implements(t, (*ego.Logger)(nil), backend)
		assert.Implements(t, (*ego.ContextLogger)(nil), backend)
	})

	t.Run("kit-logger cannot satisfy the rest", func(t *testing.T) {
		// With(...any) kitlogger.Logger is not With(...any) ego.Logger: Go has
		// no covariance on a method's return type.
		assert.NotImplements(t, (*ego.FieldLogger)(nil), backend)
		// kit-logger has no Enabled method; the adapter routes to Slog().
		assert.NotImplements(t, (*ego.EnabledLogger)(nil), backend)
		// kit-logger has SetLevel but no level getter.
		assert.NotImplements(t, (*ego.LeveledLogger)(nil), backend)
	})

	t.Run("the adapter supplies every capability kit-logger can back", func(t *testing.T) {
		adapter := egokit.New(backend)
		assert.Implements(t, (*ego.Logger)(nil), adapter)
		assert.Implements(t, (*ego.ContextLogger)(nil), adapter)
		assert.Implements(t, (*ego.FieldLogger)(nil), adapter)
		assert.Implements(t, (*ego.EnabledLogger)(nil), adapter)
	})

	t.Run("the adapter claims no capability kit-logger cannot back", func(t *testing.T) {
		// Reporting a level the backend never exposes would be a fabrication;
		// EnabledLogger already answers the same question exactly.
		assert.NotImplements(t, (*ego.LeveledLogger)(nil), egokit.New(backend))
	})
}

func TestNew(t *testing.T) {
	t.Run("a usable logger yields an adapter over it", func(t *testing.T) {
		backend, _ := newBackend(slog.LevelDebug)
		adapter, ok := egokit.New(backend).(*egokit.Adapter)
		require.True(t, ok)
		assert.Same(t, backend, adapter.Unwrap())
	})

	t.Run("a nil logger falls back to the eGo default", func(t *testing.T) {
		assert.Equal(t, ego.DefaultLogger, egokit.New(nil))
	})

	t.Run("a typed-nil logger falls back to the eGo default", func(t *testing.T) {
		// Non-nil at the interface level, nil underneath: logging through it
		// would panic, so it must never reach an Adapter.
		var typedNil *kitlogger.SlogLogger
		assert.Equal(t, ego.DefaultLogger, egokit.New(typedNil))
	})
}

func TestBaseMethodsReachTheBackend(t *testing.T) {
	testCases := []struct {
		name  string
		level slog.Level
		log   func(ego.Logger)
	}{
		{"debug", slog.LevelDebug, func(l ego.Logger) { l.Debug("a message") }},
		{"info", slog.LevelInfo, func(l ego.Logger) { l.Info("a message") }},
		{"warn", slog.LevelWarn, func(l ego.Logger) { l.Warn("a message") }},
		{"error", slog.LevelError, func(l ego.Logger) { l.Error("a message") }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend, handler := newBackend(slog.LevelDebug)
			tc.log(egokit.New(backend))

			got := handler.only(t)
			assert.Equal(t, tc.level, got.level)
			assert.Equal(t, "a message", got.msg)
		})
	}
}

func TestStructuredFieldsArePreserved(t *testing.T) {
	backend, handler := newBackend(slog.LevelDebug)

	egokit.New(backend).Info("entity spawned", "entity_id", "abc-123", "shards", 8)

	got := handler.only(t)
	assert.Equal(t, "entity spawned", got.msg)
	assert.Equal(t, []any{"entity_id", "abc-123", "shards", int64(8)}, attrPairs(got))
}

func TestWith(t *testing.T) {
	t.Run("a child logger keeps its field on later calls", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelDebug)

		child := egokit.New(backend).(ego.FieldLogger).With("component", "runtime")
		child.Info("first")
		child.Info("second")

		got := handler.captured()
		require.Len(t, got, 2)
		for _, r := range got {
			assert.Equal(t, []any{"component", "runtime"}, attrPairs(r))
		}
	})

	t.Run("nested calls accumulate in order without duplicates", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelDebug)

		root := egokit.New(backend)
		child := root.(ego.FieldLogger).With("a", 1)
		grandchild := child.(ego.FieldLogger).With("b", 2)
		grandchild.Info("nested", "c", 3)

		assert.Equal(t,
			[]any{"a", int64(1), "b", int64(2), "c", int64(3)},
			attrPairs(handler.only(t)),
		)
	})

	t.Run("a parent is not polluted by its child", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelDebug)

		root := egokit.New(backend)
		_ = root.(ego.FieldLogger).With("child_only", true)
		root.Info("from the parent")

		assert.Empty(t, attrPairs(handler.only(t)))
	})

	t.Run("no arguments returns the same adapter", func(t *testing.T) {
		backend, _ := newBackend(slog.LevelDebug)

		root := egokit.New(backend)
		assert.Same(t, root, root.(ego.FieldLogger).With())
	})

	t.Run("a child keeps every capability of its parent", func(t *testing.T) {
		backend, _ := newBackend(slog.LevelDebug)

		child := egokit.New(backend).(ego.FieldLogger).With("component", "runtime")
		assert.Implements(t, (*ego.ContextLogger)(nil), child)
		assert.Implements(t, (*ego.EnabledLogger)(nil), child)
		assert.Implements(t, (*ego.FieldLogger)(nil), child)
	})
}

// ctxKey is a private context key, so nothing outside this file can produce a
// context that compares equal to the one under test by accident.
type ctxKey struct{}

func TestContextIsForwardedVerbatim(t *testing.T) {
	testCases := []struct {
		name  string
		level slog.Level
		log   func(ego.ContextLogger, context.Context)
	}{
		{"debug", slog.LevelDebug, func(l ego.ContextLogger, ctx context.Context) {
			l.DebugContext(ctx, "a message", "k", "v")
		}},
		{"info", slog.LevelInfo, func(l ego.ContextLogger, ctx context.Context) {
			l.InfoContext(ctx, "a message", "k", "v")
		}},
		{"warn", slog.LevelWarn, func(l ego.ContextLogger, ctx context.Context) {
			l.WarnContext(ctx, "a message", "k", "v")
		}},
		{"error", slog.LevelError, func(l ego.ContextLogger, ctx context.Context) {
			l.ErrorContext(ctx, "a message", "k", "v")
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend, handler := newBackend(slog.LevelDebug)
			ctx := context.WithValue(t.Context(), ctxKey{}, "carried")

			tc.log(egokit.New(backend).(ego.ContextLogger), ctx)

			got := handler.only(t)
			// Identity, not equality: the backend must receive the very
			// context the call site carried, so a trace id stays reachable.
			assert.Same(t, ctx, got.ctx)
			assert.Equal(t, "carried", got.ctx.Value(ctxKey{}))
			assert.Equal(t, tc.level, got.level)
			assert.Equal(t, []any{"k", "v"}, attrPairs(got))
		})
	}

	t.Run("a child logger forwards the context too", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelDebug)
		ctx := context.WithValue(t.Context(), ctxKey{}, "carried")

		child := egokit.New(backend).(ego.FieldLogger).With("component", "runtime")
		child.(ego.ContextLogger).InfoContext(ctx, "a message")

		got := handler.only(t)
		assert.Same(t, ctx, got.ctx)
		assert.Equal(t, []any{"component", "runtime"}, attrPairs(got))
	})
}

func TestEnabled(t *testing.T) {
	t.Run("the backend decides", func(t *testing.T) {
		testCases := []struct {
			name     string
			minLevel slog.Level
			asked    slog.Level
			expected bool
		}{
			{"debug disabled below an info threshold", slog.LevelInfo, slog.LevelDebug, false},
			{"debug enabled at a debug threshold", slog.LevelDebug, slog.LevelDebug, true},
			{"info enabled at an info threshold", slog.LevelInfo, slog.LevelInfo, true},
			{"info disabled below an error threshold", slog.LevelError, slog.LevelInfo, false},
			{"error enabled at an error threshold", slog.LevelError, slog.LevelError, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				backend, _ := newBackend(tc.minLevel)
				adapter := egokit.New(backend).(ego.EnabledLogger)
				assert.Equal(t, tc.expected, adapter.Enabled(t.Context(), tc.asked))
			})
		}
	})

	t.Run("a disabled level emits no record", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelInfo)

		egokit.New(backend).Debug("never emitted")

		assert.Empty(t, handler.captured())
	})

	t.Run("a level change through the backend is honored immediately", func(t *testing.T) {
		handler := newCaptureHandler(slog.LevelInfo)
		backend := kitlogger.New(kitlogger.Config{Handler: handler})
		adapter := egokit.New(backend).(ego.EnabledLogger)

		require.False(t, adapter.Enabled(t.Context(), slog.LevelDebug))

		// The adapter caches the *slog.Logger, never the level it reports, so
		// the very next check must see the new threshold.
		handler.minLevel.Set(slog.LevelDebug)
		assert.True(t, adapter.Enabled(t.Context(), slog.LevelDebug))
	})

	t.Run("a child logger answers with its parent's threshold", func(t *testing.T) {
		backend, _ := newBackend(slog.LevelInfo)

		child := egokit.New(backend).(ego.FieldLogger).With("component", "runtime")
		assert.False(t, child.(ego.EnabledLogger).Enabled(t.Context(), slog.LevelDebug))
		assert.True(t, child.(ego.EnabledLogger).Enabled(t.Context(), slog.LevelInfo))
	})
}

func TestErrorValuesAreNotStringified(t *testing.T) {
	backend, handler := newBackend(slog.LevelDebug)
	failure := assert.AnError

	// kit-logger ships helpers (logger.Error, logger.ErrorValue) that flatten an
	// error to its message. The adapter must not apply them on the caller's
	// behalf: an error passed as a field reaches the handler as an error, so a
	// handler that unwraps, groups, or serializes it can still do so.
	egokit.New(backend).Error("persist failed", "error", failure)

	got := handler.only(t)
	require.Len(t, got.attrs, 1)
	assert.Equal(t, "error", got.attrs[0].Key)
	assert.Same(t, failure, got.attrs[0].Value.Any())
	assert.ErrorIs(t, got.attrs[0].Value.Any().(error), assert.AnError)
}
