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
	"fmt"
	golog "log"
	"log/slog"
	"reflect"
	"strings"

	"github.com/tochemey/goakt/v4/log"
)

// Log level strings recognized by LeveledLogger.Level.
const (
	levelDebug   = "debug"
	levelInfo    = "info"
	levelWarn    = "warn"
	levelWarning = "warning"
	levelError   = "error"
	levelFatal   = "fatal"
	levelPanic   = "panic"
)

// Logger is the logging interface that developers can implement to plug in
// their own logging backend (e.g., zap, zerolog, slog, logrus).
//
// Methods follow the slog convention: msg is the log message and args are
// optional key-value pairs (alternating string keys and arbitrary values)
// for structured logging. Implementations that do not support structured
// fields may ignore args.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// LeveledLogger is an optional interface that a Logger can implement to
// advertise its minimum log level. When the inner Logger implements this
// interface, the adapter uses its level for engine-side log gating (via
// LogLevel and Enabled), avoiding unnecessary log formatting for messages
// below the configured threshold.
//
// Level returns a log level string: "debug", "info", "warn"/"warning",
// "error", "fatal", or "panic". Unrecognized values are treated as "info".
type LeveledLogger interface {
	Level() string
}

// EnabledLogger is an optional interface that a Logger can implement to answer
// level checks directly, instead of the adapter inferring them from a level
// name. When the inner Logger implements this interface it is the sole
// authority on gating: the adapter never second-guesses it.
//
// The signature mirrors log/slog's own Handler.Enabled, so any slog-backed
// logger satisfies it trivially, and slog.Level is a typed integer, so gating
// costs no string formatting or parsing on the hot path. The context is the one
// the log call carries, which lets a backend make context-sensitive gating
// decisions.
type EnabledLogger interface {
	Enabled(ctx context.Context, level slog.Level) bool
}

// ContextLogger is an optional interface that a Logger can implement to receive
// the context.Context associated with a log call. When the inner Logger
// implements it, the adapter forwards the caller's context verbatim; otherwise
// it falls back to the context-free methods. The adapter never fabricates a
// context to fill the gap.
type ContextLogger interface {
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}

// discardLogger is a Logger that silently discards all log output.
type discardLogger struct{}

func (discardLogger) Debug(string, ...any) {}
func (discardLogger) Info(string, ...any)  {}
func (discardLogger) Warn(string, ...any)  {}
func (discardLogger) Error(string, ...any) {}

// Enabled reports every level as disabled so that callers — the engine
// included — skip message formatting entirely for a discarded logger.
func (discardLogger) Enabled(context.Context, slog.Level) bool { return false }

// DiscardLogger is a Logger that silently discards all log output.
// It is useful in tests or when logging is not desired.
var DiscardLogger Logger = discardLogger{}

// defaultLogger delegates to the log/slog default logger so that the
// engine has reasonable out-of-the-box logging without requiring the
// caller to supply a Logger explicitly.
type defaultLogger struct{}

func (defaultLogger) Debug(msg string, args ...any) { slog.Debug(msg, args...) }
func (defaultLogger) Info(msg string, args ...any)  { slog.Info(msg, args...) }
func (defaultLogger) Warn(msg string, args ...any)  { slog.Warn(msg, args...) }
func (defaultLogger) Error(msg string, args ...any) { slog.Error(msg, args...) }

// Enabled asks the log/slog default logger for the truth on every call, so the
// engine's gating tracks slog.SetDefault instead of a construction-time
// snapshot.
func (defaultLogger) Enabled(ctx context.Context, level slog.Level) bool {
	return slog.Default().Enabled(ctx, level)
}

// DefaultLogger is the Logger used when none is supplied. It delegates to the
// log/slog default logger.
var DefaultLogger Logger = defaultLogger{}

// loggerAdapter wraps a Logger and satisfies the goaktlog.Logger interface
// used internally by the underlying engine.
type loggerAdapter struct {
	inner Logger

	// Capability assertions, resolved once: the dynamic type of inner cannot
	// change, so the assertions are cached even though the level value they
	// report never is.
	enabled EnabledLogger
	leveled LeveledLogger
	ctxLog  ContextLogger

	fields []any // accumulated key-value pairs from With()
}

// compile-time check
var _ log.Logger = (*loggerAdapter)(nil)

// newLoggerAdapter creates a loggerAdapter wrapping the given Logger and
// resolves which optional capability interfaces it implements.
func newLoggerAdapter(inner Logger) *loggerAdapter {
	a := &loggerAdapter{inner: inner}
	a.enabled, _ = inner.(EnabledLogger)
	a.leveled, _ = inner.(LeveledLogger)
	a.ctxLog, _ = inner.(ContextLogger)
	return a
}

// goaktLevelsByVerbosity lists every goaktlog.Level from most to least
// verbose. LogLevel walks it to report the most verbose enabled level.
var goaktLevelsByVerbosity = []log.Level{
	log.DebugLevel,
	log.InfoLevel,
	log.WarningLevel,
	log.ErrorLevel,
	log.FatalLevel,
	log.PanicLevel,
}

// enabledAt resolves the level check for the given context, per call, in
// capability precedence order:
//
//  1. EnabledLogger — the inner Logger answers exactly, no inference.
//  2. LeveledLogger — Level() is read on every check, so a runtime level
//     change is honored instead of being snapshotted at construction.
//  3. Neither — permissive. The adapter must not filter on behalf of a Logger
//     that never declared a level; the inner Logger does its own filtering,
//     and guessing InfoLevel is exactly what silently dropped every DEBUG
//     record the engine produced.
func (a *loggerAdapter) enabledAt(ctx context.Context, l log.Level) bool {
	switch {
	case a.enabled != nil:
		return a.enabled.Enabled(ctx, goaktToSlogLevel(l))
	case a.leveled != nil:
		return levelSeverity(l) >= levelSeverity(parseLevel(a.leveled.Level()))
	default:
		return true
	}
}

// isNilLogger returns true when l is nil or a typed-nil (e.g. (*MyLogger)(nil)).
// A typed-nil interface value is non-nil at the interface level but wraps a nil
// pointer, which would cause a nil-dereference panic on the first log call.
func isNilLogger(l Logger) bool {
	if l == nil {
		return true
	}
	v := reflect.ValueOf(l)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// levelSeverity maps a goaktlog.Level to an integer severity for comparison.
// Lower values are more verbose. GoAkt's level enum has non-standard ordering
// (Debug=5 in the iota) so direct numeric comparison does not reflect severity.
func levelSeverity(l log.Level) int {
	switch l {
	case log.DebugLevel:
		return 0
	case log.InfoLevel:
		return 1
	case log.WarningLevel:
		return 2
	case log.ErrorLevel:
		return 3
	case log.FatalLevel:
		return 4
	case log.PanicLevel:
		return 5
	default:
		return 1
	}
}

// goaktToSlogLevel maps a goaktlog.Level to the slog.Level an EnabledLogger is
// asked about. GoAkt's level enum has non-standard ordering (DebugLevel is the
// last iota value), so the mapping is explicit rather than a numeric cast.
// Fatal and Panic sit above Error to keep the result strictly monotonic in
// severity, which lets LogLevel probe levels in verbosity order.
func goaktToSlogLevel(l log.Level) slog.Level {
	switch l {
	case log.DebugLevel:
		return slog.LevelDebug
	case log.InfoLevel:
		return slog.LevelInfo
	case log.WarningLevel:
		return slog.LevelWarn
	case log.ErrorLevel:
		return slog.LevelError
	case log.FatalLevel:
		return slog.LevelError + 4
	case log.PanicLevel:
		return slog.LevelError + 8
	default:
		return slog.LevelInfo
	}
}

// parseLevel converts a human-readable level string to a goaktlog.Level.
// Unrecognized values default to InfoLevel.
func parseLevel(s string) log.Level {
	switch strings.ToLower(s) {
	case levelDebug:
		return log.DebugLevel
	case levelInfo:
		return log.InfoLevel
	case levelWarn, levelWarning:
		return log.WarningLevel
	case levelError:
		return log.ErrorLevel
	case levelFatal:
		return log.FatalLevel
	case levelPanic:
		return log.PanicLevel
	default:
		return log.InfoLevel
	}
}

// goaktArgsToMsg splits GoAkt variadic log arguments into a message string
// and optional key-value fields. GoAkt follows the slog convention where the
// first argument is the log message and subsequent arguments are alternating
// key-value pairs for structured logging.
func goaktArgsToMsg(args []any) (string, []any) {
	if len(args) == 0 {
		return "", nil
	}

	msg, ok := args[0].(string) // the common case — avoids fmt.Sprint reflection
	if !ok {
		msg = fmt.Sprint(args[0])
	}

	if len(args) == 1 {
		return msg, nil
	}
	return msg, args[1:]
}

// mergeFields returns the adapter's accumulated fields combined with additional
// fields. If the adapter has no accumulated fields, extra is returned as-is.
func (a *loggerAdapter) mergeFields(extra []any) []any {
	if len(a.fields) == 0 {
		return extra
	}

	if len(extra) == 0 {
		return a.fields
	}

	merged := make([]any, 0, len(a.fields)+len(extra))
	merged = append(merged, a.fields...)
	merged = append(merged, extra...)
	return merged
}

// route sends one record to the inner Logger's context-free method for level.
func (a *loggerAdapter) route(level log.Level, msg string, fields []any) {
	switch level {
	case log.DebugLevel:
		a.inner.Debug(msg, fields...)
	case log.WarningLevel:
		a.inner.Warn(msg, fields...)
	case log.ErrorLevel:
		a.inner.Error(msg, fields...)
	default:
		a.inner.Info(msg, fields...)
	}
}

// routeContext sends one record together with the caller's context. It is used
// only on paths where GoAkt actually supplied a context: when the inner Logger
// implements ContextLogger the context is forwarded verbatim, otherwise the
// context-free method is used rather than dropping the record.
func (a *loggerAdapter) routeContext(ctx context.Context, level log.Level, msg string, fields []any) {
	if a.ctxLog == nil {
		a.route(level, msg, fields)
		return
	}
	switch level {
	case log.DebugLevel:
		a.ctxLog.DebugContext(ctx, msg, fields...)
	case log.WarningLevel:
		a.ctxLog.WarnContext(ctx, msg, fields...)
	case log.ErrorLevel:
		a.ctxLog.ErrorContext(ctx, msg, fields...)
	default:
		a.ctxLog.InfoContext(ctx, msg, fields...)
	}
}

// The plain methods do not re-check the level: GoAkt already guards its call
// sites with Enabled, and the inner Logger applies its own filtering. The
// formatted methods do check, because the fmt.Sprintf allocation is one this
// adapter would otherwise pay for a record nobody will emit.

func (a *loggerAdapter) Debug(args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.route(log.DebugLevel, msg, a.mergeFields(fields))
}
func (a *loggerAdapter) Debugf(format string, args ...any) {
	if !a.enabledAt(context.Background(), log.DebugLevel) {
		return
	}
	a.route(log.DebugLevel, fmt.Sprintf(format, args...), a.fields)
}
func (a *loggerAdapter) DebugContext(ctx context.Context, args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.routeContext(ctx, log.DebugLevel, msg, a.mergeFields(fields))
}
func (a *loggerAdapter) DebugfContext(ctx context.Context, format string, args ...any) {
	if !a.enabledAt(ctx, log.DebugLevel) {
		return
	}
	a.routeContext(ctx, log.DebugLevel, fmt.Sprintf(format, args...), a.fields)
}
func (a *loggerAdapter) Info(args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.route(log.InfoLevel, msg, a.mergeFields(fields))
}
func (a *loggerAdapter) Infof(format string, args ...any) {
	if !a.enabledAt(context.Background(), log.InfoLevel) {
		return
	}
	a.route(log.InfoLevel, fmt.Sprintf(format, args...), a.fields)
}
func (a *loggerAdapter) InfoContext(ctx context.Context, args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.routeContext(ctx, log.InfoLevel, msg, a.mergeFields(fields))
}
func (a *loggerAdapter) InfofContext(ctx context.Context, format string, args ...any) {
	if !a.enabledAt(ctx, log.InfoLevel) {
		return
	}
	a.routeContext(ctx, log.InfoLevel, fmt.Sprintf(format, args...), a.fields)
}
func (a *loggerAdapter) Warn(args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.route(log.WarningLevel, msg, a.mergeFields(fields))
}
func (a *loggerAdapter) Warnf(format string, args ...any) {
	if !a.enabledAt(context.Background(), log.WarningLevel) {
		return
	}
	a.route(log.WarningLevel, fmt.Sprintf(format, args...), a.fields)
}
func (a *loggerAdapter) WarnContext(ctx context.Context, args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.routeContext(ctx, log.WarningLevel, msg, a.mergeFields(fields))
}
func (a *loggerAdapter) WarnfContext(ctx context.Context, format string, args ...any) {
	if !a.enabledAt(ctx, log.WarningLevel) {
		return
	}
	a.routeContext(ctx, log.WarningLevel, fmt.Sprintf(format, args...), a.fields)
}
func (a *loggerAdapter) Error(args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.route(log.ErrorLevel, msg, a.mergeFields(fields))
}
func (a *loggerAdapter) Errorf(format string, args ...any) {
	if !a.enabledAt(context.Background(), log.ErrorLevel) {
		return
	}
	a.route(log.ErrorLevel, fmt.Sprintf(format, args...), a.fields)
}
func (a *loggerAdapter) ErrorContext(ctx context.Context, args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.routeContext(ctx, log.ErrorLevel, msg, a.mergeFields(fields))
}
func (a *loggerAdapter) ErrorfContext(ctx context.Context, format string, args ...any) {
	if !a.enabledAt(ctx, log.ErrorLevel) {
		return
	}
	a.routeContext(ctx, log.ErrorLevel, fmt.Sprintf(format, args...), a.fields)
}

// LogLevel reports the most verbose level the inner Logger currently accepts.
// It is resolved per call, never cached. When the Logger enables nothing —
// DiscardLogger, for instance — the result is InvalidLevel, matching what
// GoAkt's own Zap adapter reports for an unmappable level.
func (a *loggerAdapter) LogLevel() log.Level {
	switch {
	case a.enabled != nil:
		for _, l := range goaktLevelsByVerbosity {
			if a.enabled.Enabled(context.Background(), goaktToSlogLevel(l)) {
				return l
			}
		}
		return log.InvalidLevel
	case a.leveled != nil:
		return parseLevel(a.leveled.Level())
	default:
		return log.DebugLevel
	}
}

// Enabled reports whether the inner Logger accepts the given level. GoAkt's
// Logger interface carries no context here, so Background is used; the
// *Context methods forward the caller's real context instead.
func (a *loggerAdapter) Enabled(l log.Level) bool {
	return a.enabledAt(context.Background(), l)
}

// With returns a child logger carrying the given key-value pairs.
func (a *loggerAdapter) With(keyValues ...any) log.Logger {
	if len(keyValues) == 0 {
		return a
	}
	child := newLoggerAdapter(a.inner)
	child.fields = make([]any, 0, len(a.fields)+len(keyValues))
	child.fields = append(child.fields, a.fields...)
	child.fields = append(child.fields, keyValues...)
	return child
}
func (a *loggerAdapter) Flush() error { return nil }

func (a *loggerAdapter) StdLogger() *golog.Logger {
	return golog.New(&loggerWriter{inner: a.inner}, "", 0)
}

// loggerWriter adapts Logger.Info to io.Writer for use with *log.Logger.
type loggerWriter struct {
	inner Logger
}

func (w *loggerWriter) Write(p []byte) (int, error) {
	w.inner.Info(strings.TrimRight(string(p), "\r\n"))
	return len(p), nil
}
