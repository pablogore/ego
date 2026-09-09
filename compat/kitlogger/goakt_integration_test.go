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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goakt "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/ego/v4"
	egokit "github.com/tochemey/ego/v4/compat/kitlogger"
)

// goaktLogger builds the actor-system logger eGo hands to GoAkt for a given
// eGo Logger, which is the whole delivery chain under test:
//
//	GoAkt -> ego's internal loggerAdapter -> egokit.Adapter -> kit-logger
//
// It goes through ego.NewConfig/GoaktOptions — the same public path
// ego.NewEngine uses — so the adapter under test is the one a real deployment
// gets, not a stand-in assembled by the test.
func goaktLogger(t *testing.T, logger ego.Logger) goaktlog.Logger {
	t.Helper()

	system, err := goakt.NewActorSystem(
		"test-system",
		ego.NewConfig(nil, ego.WithLogger(logger)).GoaktOptions()...,
	)
	require.NoError(t, err)

	got := system.Logger()
	require.NotNil(t, got)
	return got
}

func TestGoAktDeliversToKitLogger(t *testing.T) {
	t.Run("a record reaches kit-logger with its message and fields intact", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelDebug)

		goaktLogger(t, egokit.New(backend)).Info("actor system starting", "shards", 4)

		got := handler.only(t)
		// Exactly the message GoAkt passed: no level prefix, no re-rendering.
		assert.Equal(t, "actor system starting", got.msg)
		assert.Equal(t, slog.LevelInfo, got.level)
		assert.Equal(t, []any{"shards", int64(4)}, attrPairs(got))
	})

	t.Run("component fields survive the whole chain exactly once", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelDebug)

		// WithFields finds the adapter's native FieldLogger, so the field is
		// stored by kit-logger itself rather than replayed by an eGo wrapper.
		component := ego.WithFields(egokit.New(backend), "component", "runtime")
		goaktLogger(t, component).Info("entity spawned", "entity_id", "abc-123")

		got := handler.only(t)
		assert.Equal(t, "entity spawned", got.msg)
		assert.Equal(t,
			[]any{"component", "runtime", "entity_id", "abc-123"},
			attrPairs(got),
		)
	})

	t.Run("a GoAkt child logger nests fields without duplicating them", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelDebug)

		component := ego.WithFields(egokit.New(backend), "component", "runtime")
		child := goaktLogger(t, component).With("actor", "order-42")
		child.Info("message received", "kind", "PlaceOrder")

		assert.Equal(t,
			[]any{"component", "runtime", "actor", "order-42", "kind", "PlaceOrder"},
			attrPairs(handler.only(t)),
		)
	})

	t.Run("formatted records are rendered once", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelDebug)

		goaktLogger(t, egokit.New(backend)).Infof("bound to %s:%d", "127.0.0.1", 8080)

		got := handler.only(t)
		assert.Equal(t, "bound to 127.0.0.1:8080", got.msg)
		assert.Empty(t, attrPairs(got))
	})
}

func TestGoAktLevelGatingFollowsKitLogger(t *testing.T) {
	t.Run("the reported level tracks the backend threshold", func(t *testing.T) {
		testCases := []struct {
			name     string
			minLevel slog.Level
			expected goaktlog.Level
		}{
			{"debug", slog.LevelDebug, goaktlog.DebugLevel},
			{"info", slog.LevelInfo, goaktlog.InfoLevel},
			{"warn", slog.LevelWarn, goaktlog.WarningLevel},
			{"error", slog.LevelError, goaktlog.ErrorLevel},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				backend, _ := newBackend(tc.minLevel)
				logger := goaktLogger(t, egokit.New(backend))

				assert.Equal(t, tc.expected, logger.LogLevel())
				assert.True(t, logger.Enabled(tc.expected))
			})
		}
	})

	t.Run("debug is refused below an info threshold", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelInfo)
		logger := goaktLogger(t, egokit.New(backend))

		assert.False(t, logger.Enabled(goaktlog.DebugLevel))

		logger.Debug("never emitted")
		logger.Debugf("never %s", "emitted")
		logger.DebugContext(t.Context(), "never emitted")
		logger.DebugfContext(t.Context(), "never %s", "emitted")

		assert.Empty(t, handler.captured())
	})

	t.Run("debug is delivered at a debug threshold", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelDebug)
		logger := goaktLogger(t, egokit.New(backend))

		require.True(t, logger.Enabled(goaktlog.DebugLevel))
		logger.Debug("a debug record")

		got := handler.only(t)
		assert.Equal(t, slog.LevelDebug, got.level)
		assert.Equal(t, "a debug record", got.msg)
	})
}

func TestGoAktPreservesContext(t *testing.T) {
	testCases := []struct {
		name     string
		expected slog.Level
		log      func(goaktlog.Logger, context.Context)
	}{
		{"debug", slog.LevelDebug, func(l goaktlog.Logger, ctx context.Context) {
			l.DebugContext(ctx, "a message", "k", "v")
		}},
		{"info", slog.LevelInfo, func(l goaktlog.Logger, ctx context.Context) {
			l.InfoContext(ctx, "a message", "k", "v")
		}},
		{"warn", slog.LevelWarn, func(l goaktlog.Logger, ctx context.Context) {
			l.WarnContext(ctx, "a message", "k", "v")
		}},
		{"error", slog.LevelError, func(l goaktlog.Logger, ctx context.Context) {
			l.ErrorContext(ctx, "a message", "k", "v")
		}},
		{"formatted info", slog.LevelInfo, func(l goaktlog.Logger, ctx context.Context) {
			l.InfofContext(ctx, "a %s", "message")
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend, handler := newBackend(slog.LevelDebug)
			ctx := context.WithValue(t.Context(), ctxKey{}, "carried")

			tc.log(goaktLogger(t, egokit.New(backend)), ctx)

			got := handler.only(t)
			// The context GoAkt carried must arrive unchanged, which is what
			// later lets a trace id be correlated with the record.
			assert.Same(t, ctx, got.ctx)
			assert.Equal(t, "carried", got.ctx.Value(ctxKey{}))
			assert.Equal(t, tc.expected, got.level)
			assert.Equal(t, "a message", got.msg)
		})
	}

	t.Run("a component logger still forwards the context", func(t *testing.T) {
		backend, handler := newBackend(slog.LevelDebug)
		ctx := context.WithValue(t.Context(), ctxKey{}, "carried")

		component := ego.WithFields(egokit.New(backend), "component", "runtime")
		goaktLogger(t, component).InfoContext(ctx, "entity spawned")

		got := handler.only(t)
		assert.Same(t, ctx, got.ctx)
		assert.Equal(t, []any{"component", "runtime"}, attrPairs(got))
	})
}
