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
	"io"
	"log/slog"
	"testing"

	kitlogger "github.com/pablogore/kit-logger/pkg/logger"
	goakt "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/ego/v4"
	egokit "github.com/tochemey/ego/v4/compat/kitlogger"
)

// slogLogger is the baseline: the thinnest possible ego.Logger over the same
// *slog.Logger the kit-logger benchmarks write to. Comparing against it isolates
// what the adapter and kit-logger add, rather than measuring slog's formatting.
type slogLogger struct{ inner *slog.Logger }

func (l slogLogger) Debug(msg string, args ...any) { l.inner.Debug(msg, args...) }
func (l slogLogger) Info(msg string, args ...any)  { l.inner.Info(msg, args...) }
func (l slogLogger) Warn(msg string, args ...any)  { l.inner.Warn(msg, args...) }
func (l slogLogger) Error(msg string, args ...any) { l.inner.Error(msg, args...) }

func (l slogLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.inner.DebugContext(ctx, msg, args...)
}

func (l slogLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.inner.InfoContext(ctx, msg, args...)
}

func (l slogLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.inner.WarnContext(ctx, msg, args...)
}

func (l slogLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.inner.ErrorContext(ctx, msg, args...)
}

func (l slogLogger) With(args ...any) ego.Logger {
	return slogLogger{inner: l.inner.With(args...)}
}

func (l slogLogger) Enabled(ctx context.Context, level slog.Level) bool {
	return l.inner.Enabled(ctx, level)
}

var (
	_ ego.Logger        = slogLogger{}
	_ ego.ContextLogger = slogLogger{}
	_ ego.FieldLogger   = slogLogger{}
	_ ego.EnabledLogger = slogLogger{}
)

// discardHandler is a text handler over io.Discard, so both sides of every
// comparison pay the same real formatting cost and nothing else.
func discardHandler(level slog.Level) slog.Handler {
	return slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: level})
}

func benchDirect(level slog.Level) ego.Logger {
	return slogLogger{inner: slog.New(discardHandler(level))}
}

func benchAdapted(level slog.Level) ego.Logger {
	return egokit.New(kitlogger.New(kitlogger.Config{Handler: discardHandler(level)}))
}

// benchGoakt builds the full GoAkt-facing chain for an eGo Logger, matching the
// root module's BenchmarkLoggerAdapterDebugf* so the two can be read together.
func benchGoakt(b *testing.B, logger ego.Logger) goaktlog.Logger {
	b.Helper()

	system, err := goakt.NewActorSystem("bench", ego.NewConfig(nil, ego.WithLogger(logger)).GoaktOptions()...)
	if err != nil {
		b.Fatal(err)
	}
	return system.Logger()
}

func benchmarkLoggers(b *testing.B, level slog.Level, fn func(*testing.B, ego.Logger)) {
	b.Helper()

	b.Run("direct", func(b *testing.B) { fn(b, benchDirect(level)) })
	b.Run("kitlogger", func(b *testing.B) { fn(b, benchAdapted(level)) })
}

// BenchmarkDisabled measures the path that matters most: a record nobody will
// emit. Both loggers must reject it without formatting anything.
func BenchmarkDisabled(b *testing.B) {
	benchmarkLoggers(b, slog.LevelInfo, func(b *testing.B, logger ego.Logger) {
		b.ReportAllocs()
		for b.Loop() {
			logger.Debug("a discarded record", "entity_id", "abc-123")
		}
	})
}

// BenchmarkEnabledStructured measures a delivered record carrying fields.
func BenchmarkEnabledStructured(b *testing.B) {
	benchmarkLoggers(b, slog.LevelDebug, func(b *testing.B, logger ego.Logger) {
		b.ReportAllocs()
		for b.Loop() {
			logger.Info("entity spawned", "entity_id", "abc-123", "shards", 8)
		}
	})
}

// BenchmarkWith measures building a child logger, the operation kit-logger
// backs natively instead of having eGo replay fields on every record.
func BenchmarkWith(b *testing.B) {
	benchmarkLoggers(b, slog.LevelDebug, func(b *testing.B, logger ego.Logger) {
		fielder := logger.(ego.FieldLogger)
		b.ReportAllocs()
		for b.Loop() {
			_ = fielder.With("component", "runtime")
		}
	})
}

// BenchmarkWithThenLog measures logging through a child logger, which is how
// every component-tagged eGo logger is actually used.
func BenchmarkWithThenLog(b *testing.B) {
	benchmarkLoggers(b, slog.LevelDebug, func(b *testing.B, logger ego.Logger) {
		child := logger.(ego.FieldLogger).With("component", "runtime")
		b.ReportAllocs()
		for b.Loop() {
			child.Info("entity spawned", "entity_id", "abc-123")
		}
	})
}

// BenchmarkGoAktDebugfDisabled mirrors the root module's benchmark of the same
// name, so the cost the kit-logger backend adds to GoAkt's hottest gated path
// can be compared directly.
func BenchmarkGoAktDebugfDisabled(b *testing.B) {
	benchmarkLoggers(b, slog.LevelInfo, func(b *testing.B, logger ego.Logger) {
		adapter := benchGoakt(b, logger)
		b.ReportAllocs()
		for b.Loop() {
			adapter.Debugf("a discarded record for %s", "abc-123")
		}
	})
}

// BenchmarkGoAktDebugfEnabled mirrors the root module's benchmark of the same
// name for the delivered path.
func BenchmarkGoAktDebugfEnabled(b *testing.B) {
	benchmarkLoggers(b, slog.LevelDebug, func(b *testing.B, logger ego.Logger) {
		adapter := benchGoakt(b, logger)
		b.ReportAllocs()
		for b.Loop() {
			adapter.Debugf("a delivered record for %s", "abc-123")
		}
	})
}
