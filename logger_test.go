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

package ego

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tochemey/goakt/v4/log"
)

// spyLogger records the last method called, the message, and any structured
// fields passed to it. It implements Logger and is used to verify that
// loggerAdapter routes each GoAkt method to the correct inner method with the
// correct message and fields.
type spyLogger struct {
	lastMethod string
	lastMsg    string
	lastFields []any
}

func (s *spyLogger) Debug(msg string, args ...any) {
	s.lastMethod = "debug"
	s.lastMsg = msg
	s.lastFields = args
}
func (s *spyLogger) Info(msg string, args ...any) {
	s.lastMethod = "info"
	s.lastMsg = msg
	s.lastFields = args
}
func (s *spyLogger) Warn(msg string, args ...any) {
	s.lastMethod = "warn"
	s.lastMsg = msg
	s.lastFields = args
}
func (s *spyLogger) Error(msg string, args ...any) {
	s.lastMethod = "error"
	s.lastMsg = msg
	s.lastFields = args
}

func newAdapter(spy *spyLogger) *loggerAdapter {
	return newLoggerAdapter(spy)
}

// leveledSpyLogger extends spyLogger with LeveledLogger support.
type leveledSpyLogger struct {
	spyLogger
	level string
}

func (l *leveledSpyLogger) Level() string { return l.level }

// -----------------------------------------------------------------------------
// Debug family
// -----------------------------------------------------------------------------

func TestLoggerAdapterDebug(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Debug("debug msg")
	assert.Equal(t, "debug", spy.lastMethod)
	assert.Equal(t, "debug msg", spy.lastMsg)
}

func TestLoggerAdapterDebugf(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Debugf("hello %s", "world")
	assert.Equal(t, "debug", spy.lastMethod)
	assert.Equal(t, "hello world", spy.lastMsg)
}

func TestLoggerAdapterDebugContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.DebugContext(context.Background(), "ctx debug")
	assert.Equal(t, "debug", spy.lastMethod)
	assert.Equal(t, "ctx debug", spy.lastMsg)
}

func TestLoggerAdapterDebugfContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.DebugfContext(context.Background(), "ctx %s", "debugf")
	assert.Equal(t, "debug", spy.lastMethod)
	assert.Equal(t, "ctx debugf", spy.lastMsg)
}

// -----------------------------------------------------------------------------
// Info family
// -----------------------------------------------------------------------------

func TestLoggerAdapterInfo(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Info("info msg")
	assert.Equal(t, "info", spy.lastMethod)
	assert.Equal(t, "info msg", spy.lastMsg)
}

func TestLoggerAdapterInfof(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Infof("hello %s", "world")
	assert.Equal(t, "info", spy.lastMethod)
	assert.Equal(t, "hello world", spy.lastMsg)
}

func TestLoggerAdapterInfoContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.InfoContext(context.Background(), "ctx info")
	assert.Equal(t, "info", spy.lastMethod)
	assert.Equal(t, "ctx info", spy.lastMsg)
}

func TestLoggerAdapterInfofContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.InfofContext(context.Background(), "ctx %s", "infof")
	assert.Equal(t, "info", spy.lastMethod)
	assert.Equal(t, "ctx infof", spy.lastMsg)
}

// -----------------------------------------------------------------------------
// Warn family
// -----------------------------------------------------------------------------

func TestLoggerAdapterWarn(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Warn("warn msg")
	assert.Equal(t, "warn", spy.lastMethod)
	assert.Equal(t, "warn msg", spy.lastMsg)
}

func TestLoggerAdapterWarnf(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Warnf("hello %s", "world")
	assert.Equal(t, "warn", spy.lastMethod)
	assert.Equal(t, "hello world", spy.lastMsg)
}

func TestLoggerAdapterWarnContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.WarnContext(context.Background(), "ctx warn")
	assert.Equal(t, "warn", spy.lastMethod)
	assert.Equal(t, "ctx warn", spy.lastMsg)
}

func TestLoggerAdapterWarnfContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.WarnfContext(context.Background(), "ctx %s", "warnf")
	assert.Equal(t, "warn", spy.lastMethod)
	assert.Equal(t, "ctx warnf", spy.lastMsg)
}

// -----------------------------------------------------------------------------
// Error family
// -----------------------------------------------------------------------------

func TestLoggerAdapterError(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Error("error msg")
	assert.Equal(t, "error", spy.lastMethod)
	assert.Equal(t, "error msg", spy.lastMsg)
}

func TestLoggerAdapterErrorf(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Errorf("hello %s", "world")
	assert.Equal(t, "error", spy.lastMethod)
	assert.Equal(t, "hello world", spy.lastMsg)
}

func TestLoggerAdapterErrorContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.ErrorContext(context.Background(), "ctx error")
	assert.Equal(t, "error", spy.lastMethod)
	assert.Equal(t, "ctx error", spy.lastMsg)
}

func TestLoggerAdapterErrorfContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.ErrorfContext(context.Background(), "ctx %s", "errorf")
	assert.Equal(t, "error", spy.lastMethod)
	assert.Equal(t, "ctx errorf", spy.lastMsg)
}

// -----------------------------------------------------------------------------
// Utility methods
// -----------------------------------------------------------------------------

func TestLoggerAdapterLogLevel(t *testing.T) {
	// A Logger that declares no level is not filtered by the adapter: reporting
	// InfoLevel here is what silently dropped every GoAkt DEBUG record.
	t.Run("reports DebugLevel when no capability interface is implemented", func(t *testing.T) {
		a := newAdapter(&spyLogger{})
		assert.Equal(t, log.DebugLevel, a.LogLevel())
	})

	t.Run("uses LeveledLogger level when implemented", func(t *testing.T) {
		a := newLoggerAdapter(&leveledSpyLogger{level: "debug"})
		assert.Equal(t, log.DebugLevel, a.LogLevel())
	})

	t.Run("unrecognized LeveledLogger value defaults to InfoLevel", func(t *testing.T) {
		a := newLoggerAdapter(&leveledSpyLogger{level: "unknown"})
		assert.Equal(t, log.InfoLevel, a.LogLevel())
	})
}

func TestLoggerAdapterEnabled(t *testing.T) {
	t.Run("no capability interface enables every level", func(t *testing.T) {
		a := newAdapter(&spyLogger{})
		assert.True(t, a.Enabled(log.DebugLevel))
		assert.True(t, a.Enabled(log.InfoLevel))
		assert.True(t, a.Enabled(log.WarningLevel))
		assert.True(t, a.Enabled(log.ErrorLevel))
		assert.True(t, a.Enabled(log.FatalLevel))
		assert.True(t, a.Enabled(log.PanicLevel))
	})

	t.Run("DebugLevel enables all levels", func(t *testing.T) {
		a := newLoggerAdapter(&leveledSpyLogger{level: "debug"})
		assert.True(t, a.Enabled(log.DebugLevel))
		assert.True(t, a.Enabled(log.InfoLevel))
		assert.True(t, a.Enabled(log.ErrorLevel))
	})

	t.Run("ErrorLevel gates debug, info, and warning", func(t *testing.T) {
		a := newLoggerAdapter(&leveledSpyLogger{level: "error"})
		assert.False(t, a.Enabled(log.DebugLevel))
		assert.False(t, a.Enabled(log.InfoLevel))
		assert.False(t, a.Enabled(log.WarningLevel))
		assert.True(t, a.Enabled(log.ErrorLevel))
		assert.True(t, a.Enabled(log.FatalLevel))
	})
}

func TestLoggerAdapterWith(t *testing.T) {
	t.Run("no args returns same adapter", func(t *testing.T) {
		a := newAdapter(&spyLogger{})
		result := a.With()
		assert.Same(t, a, result)
	})
	t.Run("returns new adapter with accumulated fields", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		child := a.With("tenant", "t1")
		assert.NotSame(t, a, child)

		// child should carry the fields
		child.Info("hello")
		assert.Equal(t, "info", spy.lastMethod)
		assert.Equal(t, "hello", spy.lastMsg)
		assert.Equal(t, []any{"tenant", "t1"}, spy.lastFields)

		// original adapter should not have the fields
		a.Info("plain")
		assert.Equal(t, "plain", spy.lastMsg)
		assert.Empty(t, spy.lastFields)
	})
	t.Run("fields accumulate across chained With calls", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		child := a.With("k1", "v1").With("k2", "v2")
		child.Info("msg")
		assert.Equal(t, []any{"k1", "v1", "k2", "v2"}, spy.lastFields)
	})
	t.Run("preserves LogLevel and Enabled", func(t *testing.T) {
		spy := &leveledSpyLogger{spyLogger: spyLogger{}, level: "error"}
		a := newLoggerAdapter(spy)
		child := a.With("key", "value")
		la := child.(*loggerAdapter)
		assert.Equal(t, log.ErrorLevel, la.LogLevel())
		assert.True(t, la.Enabled(log.ErrorLevel))
		assert.False(t, la.Enabled(log.DebugLevel))
	})
	t.Run("fields are prepended to formatted log methods", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		child := a.With("rid", "123")
		child.(*loggerAdapter).Debugf("count=%d", 5)
		assert.Equal(t, "debug", spy.lastMethod)
		assert.Equal(t, "count=5", spy.lastMsg)
		assert.Equal(t, []any{"rid", "123"}, spy.lastFields)
	})
}

func TestLoggerAdapterFlush(t *testing.T) {
	a := newAdapter(&spyLogger{})
	assert.NoError(t, a.Flush())
}

func TestLoggerAdapterStdLogger(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	std := a.StdLogger()
	require.NotNil(t, std)
	std.Print("std message")
	assert.Equal(t, "info", spy.lastMethod)
	assert.Contains(t, spy.lastMsg, "std message")
	assert.False(t, strings.HasSuffix(spy.lastMsg, "\n"), "trailing newline should be trimmed")
}

// -----------------------------------------------------------------------------
// goaktArgsToMsg helper
// -----------------------------------------------------------------------------

func TestGoaktArgsToMsg(t *testing.T) {
	t.Run("empty args returns empty message and nil fields", func(t *testing.T) {
		msg, fields := goaktArgsToMsg(nil)
		assert.Equal(t, "", msg)
		assert.Nil(t, fields)
	})

	t.Run("single arg returns it as message with nil fields", func(t *testing.T) {
		msg, fields := goaktArgsToMsg([]any{"hello"})
		assert.Equal(t, "hello", msg)
		assert.Nil(t, fields)
	})

	t.Run("multiple args splits message from key-value fields", func(t *testing.T) {
		msg, fields := goaktArgsToMsg([]any{"hello", "key", "value"})
		assert.Equal(t, "hello", msg)
		assert.Equal(t, []any{"key", "value"}, fields)
	})
}

// -----------------------------------------------------------------------------
// Multi-arg routing — verifies fields are passed through, not swallowed
// -----------------------------------------------------------------------------

func TestLoggerAdapterMultiArgRouting(t *testing.T) {
	t.Run("Debug passes fields to inner logger", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		a.Debug("msg", "k", "v")
		assert.Equal(t, "msg", spy.lastMsg)
		assert.Equal(t, []any{"k", "v"}, spy.lastFields)
	})

	t.Run("Info passes fields to inner logger", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		a.Info("msg", "k", "v")
		assert.Equal(t, "msg", spy.lastMsg)
		assert.Equal(t, []any{"k", "v"}, spy.lastFields)
	})

	t.Run("Warn passes fields to inner logger", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		a.Warn("msg", "k", "v")
		assert.Equal(t, "msg", spy.lastMsg)
		assert.Equal(t, []any{"k", "v"}, spy.lastFields)
	})

	t.Run("Error passes fields to inner logger", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		a.Error("msg", "k", "v")
		assert.Equal(t, "msg", spy.lastMsg)
		assert.Equal(t, []any{"k", "v"}, spy.lastFields)
	})

	t.Run("empty args produces empty message", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		a.Info()
		assert.Equal(t, "", spy.lastMsg)
		assert.Nil(t, spy.lastFields)
	})
}

// -----------------------------------------------------------------------------
// isNilLogger helper
// -----------------------------------------------------------------------------

func TestIsNilLogger(t *testing.T) {
	t.Run("untyped nil returns true", func(t *testing.T) {
		assert.True(t, isNilLogger(nil))
	})

	t.Run("typed-nil pointer returns true", func(t *testing.T) {
		var spy *spyLogger // typed-nil
		assert.True(t, isNilLogger(spy))
	})

	t.Run("non-nil pointer returns false", func(t *testing.T) {
		assert.False(t, isNilLogger(&spyLogger{}))
	})

	t.Run("value type returns false", func(t *testing.T) {
		assert.False(t, isNilLogger(noopLogger{}))
	})
}

// -----------------------------------------------------------------------------
// parseLevel helper
// -----------------------------------------------------------------------------

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  log.Level
	}{
		{"debug", log.DebugLevel},
		{"DEBUG", log.DebugLevel},
		{"info", log.InfoLevel},
		{"warn", log.WarningLevel},
		{"warning", log.WarningLevel},
		{"error", log.ErrorLevel},
		{"fatal", log.FatalLevel},
		{"panic", log.PanicLevel},
		{"unknown", log.InfoLevel},
		{"", log.InfoLevel},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseLevel(tt.input))
		})
	}
}

// -----------------------------------------------------------------------------
// levelSeverity helper
// -----------------------------------------------------------------------------

func TestLevelSeverity(t *testing.T) {
	assert.Less(t, levelSeverity(log.DebugLevel), levelSeverity(log.InfoLevel))
	assert.Less(t, levelSeverity(log.InfoLevel), levelSeverity(log.WarningLevel))
	assert.Less(t, levelSeverity(log.WarningLevel), levelSeverity(log.ErrorLevel))
	assert.Less(t, levelSeverity(log.ErrorLevel), levelSeverity(log.FatalLevel))
	assert.Less(t, levelSeverity(log.FatalLevel), levelSeverity(log.PanicLevel))
}

// -----------------------------------------------------------------------------
// loggerWriter newline trimming
// -----------------------------------------------------------------------------

func TestLoggerWriterTrimsNewline(t *testing.T) {
	spy := &spyLogger{}
	w := &loggerWriter{inner: spy}
	n, err := w.Write([]byte("hello\n"))
	require.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, "hello", spy.lastMsg)
	assert.False(t, strings.HasSuffix(spy.lastMsg, "\n"))
}

func TestLoggerWriterTrimsCRLF(t *testing.T) {
	spy := &spyLogger{}
	w := &loggerWriter{inner: spy}
	n, err := w.Write([]byte("hello\r\n"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)
	assert.Equal(t, "hello", spy.lastMsg)
}

func TestLoggerWriterNoNewline(t *testing.T) {
	spy := &spyLogger{}
	w := &loggerWriter{inner: spy}
	n, err := w.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", spy.lastMsg)
}

// noopLogger is a Logger implementation that discards all output. It is used
// in tests to exercise WithLogger with a concrete Logger value; goaktlog is
// still imported in this file for unrelated assertions.
type noopLogger struct{}

func (noopLogger) Debug(_ string, _ ...any) {}
func (noopLogger) Info(_ string, _ ...any)  {}
func (noopLogger) Warn(_ string, _ ...any)  {}
func (noopLogger) Error(_ string, _ ...any) {}

// noopPtrLogger is a pointer-receiver Logger used to test typed-nil detection.
type noopPtrLogger struct{}

func (*noopPtrLogger) Debug(_ string, _ ...any) {}
func (*noopPtrLogger) Info(_ string, _ ...any)  {}
func (*noopPtrLogger) Warn(_ string, _ ...any)  {}
func (*noopPtrLogger) Error(_ string, _ ...any) {}

// leveledNoopLogger implements both Logger and LeveledLogger.
type leveledNoopLogger struct {
	level string
}

func (*leveledNoopLogger) Debug(_ string, _ ...any) {}
func (*leveledNoopLogger) Info(_ string, _ ...any)  {}
func (*leveledNoopLogger) Warn(_ string, _ ...any)  {}
func (*leveledNoopLogger) Error(_ string, _ ...any) {}
func (l *leveledNoopLogger) Level() string          { return l.level }

// -----------------------------------------------------------------------------
// Package-level defaults
// -----------------------------------------------------------------------------

func TestPackageLevelLoggerDefaults(t *testing.T) {
	tests := []struct {
		name   string
		logger Logger
	}{
		{name: "DefaultLogger is set", logger: DefaultLogger},
		{name: "DiscardLogger is set", logger: DiscardLogger},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.logger)
			assert.False(t, isNilLogger(tt.logger))
		})
	}

	// DefaultLogger must actually log, so it cannot be the discarding one.
	assert.NotEqual(t, DiscardLogger, DefaultLogger)
	assert.IsType(t, defaultLogger{}, DefaultLogger)
}

func TestNewConfigUsesDefaultLoggerWhenNoneSupplied(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{name: "no options at all", opts: nil},
		{name: "explicit nil logger falls back", opts: []Option{WithLogger(nil)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig(nil, tt.opts...)
			require.NotNil(t, cfg.logger)
			assert.Equal(t, DefaultLogger, cfg.logger)
		})
	}
}

// -----------------------------------------------------------------------------
// Capability interfaces — level gating (D1)
// -----------------------------------------------------------------------------

// mutableLeveledLogger returns a different level on every Level() call so a
// test can prove the adapter re-reads the level instead of caching it.
type mutableLeveledLogger struct {
	spyLogger
	levels []string
	calls  int
}

func (m *mutableLeveledLogger) Level() string {
	level := m.levels[min(m.calls, len(m.levels)-1)]
	m.calls++
	return level
}

// enabledSpyLogger implements Logger and EnabledLogger. It answers level
// checks from a minimum slog.Level and records the context it was handed.
type enabledSpyLogger struct {
	spyLogger
	min     slog.Level
	lastCtx context.Context
	calls   int
}

func (e *enabledSpyLogger) Enabled(ctx context.Context, level slog.Level) bool {
	e.lastCtx = ctx
	e.calls++
	return level >= e.min
}

// enabledAndLeveledLogger implements both capability interfaces with
// deliberately contradictory answers so precedence is observable.
type enabledAndLeveledLogger struct {
	enabledSpyLogger
}

func (enabledAndLeveledLogger) Level() string { return levelError }

func TestGoaktDebugSurvivesLoggerWithoutCapabilities(t *testing.T) {
	// GoAkt guards every DEBUG emission with `if logger.Enabled(log.DebugLevel)`
	// (see goakt actor/death_watch.go). A Logger that declares no level must not
	// be gated by the adapter, otherwise GoAkt drops DEBUG output entirely.
	spy := &spyLogger{}
	a := newLoggerAdapter(spy)

	require.True(t, a.Enabled(log.DebugLevel),
		"a Logger declaring no level must not have DEBUG filtered by the adapter")
	assert.Equal(t, log.DebugLevel, a.LogLevel())

	if a.Enabled(log.DebugLevel) { // exactly GoAkt's guard
		a.Debug("actor stopped", "actor", "a1")
	}
	assert.Equal(t, "debug", spy.lastMethod)
	assert.Equal(t, "actor stopped", spy.lastMsg)
	assert.Equal(t, []any{"actor", "a1"}, spy.lastFields)
}

func TestLoggerAdapterRereadsLeveledLoggerPerCall(t *testing.T) {
	inner := &mutableLeveledLogger{levels: []string{levelError, levelDebug}}
	a := newLoggerAdapter(inner)

	assert.False(t, a.Enabled(log.DebugLevel), "first check sees the error level")
	assert.True(t, a.Enabled(log.DebugLevel), "second check must see the new debug level")
	assert.Equal(t, log.DebugLevel, a.LogLevel(), "LogLevel must also be re-read")
}

func TestEnabledLoggerTakesPrecedenceOverLeveledLogger(t *testing.T) {
	inner := &enabledAndLeveledLogger{}
	inner.min = slog.LevelDebug // permits debug; Level() says "error"
	a := newLoggerAdapter(inner)

	assert.True(t, a.Enabled(log.DebugLevel), "EnabledLogger must win over LeveledLogger")
	assert.Equal(t, log.DebugLevel, a.LogLevel())
	assert.Positive(t, inner.calls, "EnabledLogger must actually be consulted")
}

func TestEnabledLoggerReceivesContext(t *testing.T) {
	inner := &enabledSpyLogger{min: slog.LevelDebug}
	a := newLoggerAdapter(inner)

	a.Enabled(log.DebugLevel)
	assert.Equal(t, context.Background(), inner.lastCtx,
		"GoAkt's Enabled carries no context, so the adapter passes Background")
}

func TestDiscardLoggerDisablesEveryLevel(t *testing.T) {
	el, ok := DiscardLogger.(EnabledLogger)
	require.True(t, ok, "DiscardLogger must declare EnabledLogger")

	for _, level := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
		assert.False(t, el.Enabled(context.Background(), level), "level %v", level)
	}

	a := newLoggerAdapter(DiscardLogger)
	for _, level := range []log.Level{log.DebugLevel, log.InfoLevel, log.WarningLevel, log.ErrorLevel, log.FatalLevel, log.PanicLevel} {
		assert.False(t, a.Enabled(level), "level %v", level)
	}
	assert.Equal(t, log.InvalidLevel, a.LogLevel())
}

func TestDefaultLoggerAnswersFromSlog(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn})))
	a := newLoggerAdapter(DefaultLogger)
	assert.False(t, a.Enabled(log.DebugLevel))
	assert.False(t, a.Enabled(log.InfoLevel))
	assert.True(t, a.Enabled(log.WarningLevel))
	assert.True(t, a.Enabled(log.ErrorLevel))
	assert.Equal(t, log.WarningLevel, a.LogLevel())

	// Same adapter, new default: the level must be re-read, never cached.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})))
	assert.True(t, a.Enabled(log.DebugLevel))
	assert.Equal(t, log.DebugLevel, a.LogLevel())
}

func TestGoaktToSlogLevel(t *testing.T) {
	tests := []struct {
		name string
		in   log.Level
		want slog.Level
	}{
		{"debug", log.DebugLevel, slog.LevelDebug},
		{"info", log.InfoLevel, slog.LevelInfo},
		{"warning", log.WarningLevel, slog.LevelWarn},
		{"error", log.ErrorLevel, slog.LevelError},
		{"fatal is above error", log.FatalLevel, slog.LevelError + 4},
		{"panic is above fatal", log.PanicLevel, slog.LevelError + 8},
		{"invalid falls back to info", log.InvalidLevel, slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, goaktToSlogLevel(tt.in))
		})
	}
}

func TestFormattedMethodsSkipFormattingWhenLevelDisabled(t *testing.T) {
	inner := &enabledSpyLogger{min: slog.LevelError}
	a := newLoggerAdapter(inner)

	a.Debugf("expensive %s", "debug")
	a.Infof("expensive %s", "info")
	assert.Empty(t, inner.lastMethod, "disabled levels must not reach the inner logger")

	a.Errorf("boom %s", "now")
	assert.Equal(t, "error", inner.lastMethod)
	assert.Equal(t, "boom now", inner.lastMsg)
}

// -----------------------------------------------------------------------------
// Capability interfaces — context propagation (D2)
// -----------------------------------------------------------------------------

// ctxKey is the sentinel key used to prove a context round-trips unchanged.
type ctxKey struct{}

// ctxSpyLogger implements Logger and ContextLogger, recording the context it
// received and which context-aware method received it.
type ctxSpyLogger struct {
	spyLogger
	lastCtx   context.Context
	ctxMethod string
	ctxCalls  int
}

func (c *ctxSpyLogger) record(ctx context.Context, method string) {
	c.lastCtx = ctx
	c.ctxMethod = method
	c.ctxCalls++
}

func (c *ctxSpyLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	c.record(ctx, "debug")
	c.Debug(msg, args...)
}
func (c *ctxSpyLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	c.record(ctx, "info")
	c.Info(msg, args...)
}
func (c *ctxSpyLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	c.record(ctx, "warn")
	c.Warn(msg, args...)
}
func (c *ctxSpyLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	c.record(ctx, "error")
	c.Error(msg, args...)
}

func TestContextLoggerReceivesCallerContext(t *testing.T) {
	tests := []struct {
		name   string
		call   func(*loggerAdapter, context.Context)
		method string
		msg    string
	}{
		{"DebugContext", func(a *loggerAdapter, c context.Context) { a.DebugContext(c, "d") }, "debug", "d"},
		{"DebugfContext", func(a *loggerAdapter, c context.Context) { a.DebugfContext(c, "d%d", 1) }, "debug", "d1"},
		{"InfoContext", func(a *loggerAdapter, c context.Context) { a.InfoContext(c, "i") }, "info", "i"},
		{"InfofContext", func(a *loggerAdapter, c context.Context) { a.InfofContext(c, "i%d", 2) }, "info", "i2"},
		{"WarnContext", func(a *loggerAdapter, c context.Context) { a.WarnContext(c, "w") }, "warn", "w"},
		{"WarnfContext", func(a *loggerAdapter, c context.Context) { a.WarnfContext(c, "w%d", 3) }, "warn", "w3"},
		{"ErrorContext", func(a *loggerAdapter, c context.Context) { a.ErrorContext(c, "e") }, "error", "e"},
		{"ErrorfContext", func(a *loggerAdapter, c context.Context) { a.ErrorfContext(c, "e%d", 4) }, "error", "e4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), ctxKey{}, "trace-1")
			inner := &ctxSpyLogger{}
			a := newLoggerAdapter(inner)

			tt.call(a, ctx)

			require.Equal(t, 1, inner.ctxCalls, "the context-aware method must be used")
			assert.Equal(t, ctx, inner.lastCtx, "the caller's context must be forwarded verbatim")
			assert.Equal(t, "trace-1", inner.lastCtx.Value(ctxKey{}))
			assert.Equal(t, tt.method, inner.ctxMethod)
			assert.Equal(t, tt.msg, inner.lastMsg)
		})
	}
}

func TestPlainLoggerFallsBackToNonContextPath(t *testing.T) {
	spy := &spyLogger{}
	a := newLoggerAdapter(spy)

	a.InfoContext(context.WithValue(context.Background(), ctxKey{}, "trace-1"), "hello", "k", "v")

	assert.Equal(t, "info", spy.lastMethod)
	assert.Equal(t, "hello", spy.lastMsg)
	assert.Equal(t, []any{"k", "v"}, spy.lastFields)
}

// -----------------------------------------------------------------------------
// Capability interfaces — component loggers (D3)
// -----------------------------------------------------------------------------

// nativeFieldLogger implements Logger and FieldLogger with real child-logger
// support: With returns a new logger that merges its own fields, and every
// record lands on a sink shared with its children.
type nativeFieldLogger struct {
	sink      *spyLogger
	own       []any
	withCalls *int
}

func newNativeFieldLogger() *nativeFieldLogger {
	return &nativeFieldLogger{sink: &spyLogger{}, withCalls: new(int)}
}

func (n *nativeFieldLogger) merge(args []any) []any {
	if len(n.own) == 0 {
		return args
	}
	return append(append([]any{}, n.own...), args...)
}

func (n *nativeFieldLogger) Debug(msg string, args ...any) { n.sink.Debug(msg, n.merge(args)...) }
func (n *nativeFieldLogger) Info(msg string, args ...any)  { n.sink.Info(msg, n.merge(args)...) }
func (n *nativeFieldLogger) Warn(msg string, args ...any)  { n.sink.Warn(msg, n.merge(args)...) }
func (n *nativeFieldLogger) Error(msg string, args ...any) { n.sink.Error(msg, n.merge(args)...) }

func (n *nativeFieldLogger) With(args ...any) Logger {
	*n.withCalls++
	return &nativeFieldLogger{sink: n.sink, own: n.merge(args), withCalls: n.withCalls}
}

func TestLoggerAdapterWithUsesFieldLogger(t *testing.T) {
	native := newNativeFieldLogger()
	a := newLoggerAdapter(native)

	child := a.With("component", "runtime")
	require.Equal(t, 1, *native.withCalls, "the backend's native child logger must be used")

	child.Info("started")
	assert.Equal(t, "info", native.sink.lastMethod)
	assert.Equal(t, "started", native.sink.lastMsg)
	assert.Equal(t, []any{"component", "runtime"}, native.sink.lastFields)
	assert.Empty(t, child.(*loggerAdapter).fields,
		"the adapter must not also accumulate fields the backend already carries")

	child.With("tenant", "t1").Info("nested")
	assert.Equal(t, 2, *native.withCalls)
	assert.Equal(t, []any{"component", "runtime", "tenant", "t1"}, native.sink.lastFields)
}

func TestWithFields(t *testing.T) {
	t.Run("empty args returns the same logger", func(t *testing.T) {
		spy := &spyLogger{}
		assert.Same(t, spy, WithFields(spy))
	})

	t.Run("nil logger yields DiscardLogger", func(t *testing.T) {
		var typedNil *spyLogger
		assert.Equal(t, DiscardLogger, WithFields(nil, "k", "v"))
		assert.Equal(t, DiscardLogger, WithFields(typedNil, "k", "v"))
	})

	t.Run("uses native child logger when FieldLogger is implemented", func(t *testing.T) {
		native := newNativeFieldLogger()
		child := WithFields(native, "component", "projection")
		require.Equal(t, 1, *native.withCalls)
		assert.IsType(t, &nativeFieldLogger{}, child)

		child.Warn("lagging", "offset", 7)
		assert.Equal(t, "warn", native.sink.lastMethod)
		assert.Equal(t, []any{"component", "projection", "offset", 7}, native.sink.lastFields)
	})

	t.Run("wrapper prepends fields for a plain logger", func(t *testing.T) {
		spy := &spyLogger{}
		child := WithFields(spy, "component", "engine")
		assert.NotSame(t, spy, child)

		child.Error("boom", "err", "nope")
		assert.Equal(t, "error", spy.lastMethod)
		assert.Equal(t, "boom", spy.lastMsg)
		assert.Equal(t, []any{"component", "engine", "err", "nope"}, spy.lastFields)

		child.Debug("plain")
		assert.Equal(t, []any{"component", "engine"}, spy.lastFields)
	})

	t.Run("wrapper honors the wrapped EnabledLogger", func(t *testing.T) {
		inner := &enabledSpyLogger{min: slog.LevelWarn}
		child := WithFields(inner, "component", "engine")

		el, ok := child.(EnabledLogger)
		require.True(t, ok, "wrapping must not drop EnabledLogger")
		assert.False(t, el.Enabled(context.Background(), slog.LevelDebug))
		assert.True(t, el.Enabled(context.Background(), slog.LevelError))

		// The same must hold through the adapter, which is what gates GoAkt.
		a := newLoggerAdapter(child)
		assert.False(t, a.Enabled(log.DebugLevel))
		assert.True(t, a.Enabled(log.ErrorLevel))
	})

	t.Run("wrapper honors the wrapped LeveledLogger", func(t *testing.T) {
		child := WithFields(&leveledSpyLogger{level: levelError}, "component", "engine")

		ll, ok := child.(LeveledLogger)
		require.True(t, ok, "wrapping must not drop LeveledLogger")
		assert.Equal(t, levelError, ll.Level())

		a := newLoggerAdapter(child)
		assert.False(t, a.Enabled(log.DebugLevel))
		assert.True(t, a.Enabled(log.ErrorLevel))
	})

	t.Run("wrapper honors the wrapped ContextLogger", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ctxKey{}, "trace-1")
		inner := &ctxSpyLogger{}
		child := WithFields(inner, "component", "engine")

		newLoggerAdapter(child).InfoContext(ctx, "started", "k", "v")

		require.Equal(t, 1, inner.ctxCalls, "wrapping must not drop context propagation")
		assert.Equal(t, ctx, inner.lastCtx)
		assert.Equal(t, "started", inner.lastMsg)
		assert.Equal(t, []any{"component", "engine", "k", "v"}, inner.lastFields)
	})

	t.Run("wrapper stays permissive for a plain logger", func(t *testing.T) {
		child := WithFields(&spyLogger{}, "component", "engine")
		el, ok := child.(EnabledLogger)
		require.True(t, ok)
		assert.True(t, el.Enabled(context.Background(), slog.LevelDebug))
		assert.Equal(t, levelDebug, child.(LeveledLogger).Level())
		assert.True(t, newLoggerAdapter(child).Enabled(log.DebugLevel))
	})

	t.Run("chained calls flatten onto the same logger", func(t *testing.T) {
		spy := &spyLogger{}
		WithFields(WithFields(spy, "a", 1), "b", 2).Info("msg", "c", 3)
		assert.Equal(t, []any{"a", 1, "b", 2, "c", 3}, spy.lastFields)
	})
}

// -----------------------------------------------------------------------------
// Benchmarks — a disabled level must not pay for message formatting
// -----------------------------------------------------------------------------

func BenchmarkLoggerAdapterDebugfDisabled(b *testing.B) {
	a := newLoggerAdapter(DiscardLogger) // EnabledLogger reporting false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.Debugf("processing entity %s at offset %d", "entity-1", i)
	}
}

func BenchmarkLoggerAdapterDebugfEnabled(b *testing.B) {
	a := newLoggerAdapter(noopLogger{}) // no capabilities, so permissive
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.Debugf("processing entity %s at offset %d", "entity-1", i)
	}
}
