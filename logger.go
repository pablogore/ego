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

// FieldLogger is an optional interface that a Logger can implement to return a
// child logger carrying additional structured fields. When the inner Logger
// implements it, child loggers are built natively by the backend instead of
// having fields accumulated and replayed by the adapter.
type FieldLogger interface {
	With(args ...any) Logger
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
	fielder FieldLogger

	// backend is inner with Ego's own wrappers removed — the Logger every
	// record is routed to. It is what the capability assertions above were
	// resolved against, so one consistent target serves both the context-free
	// and the context-bearing paths.
	backend Logger

	// fields are the key-value pairs replayed on every record: the fields the
	// unwrapped wrappers contribute, followed by those accumulated from With().
	fields []any
}

// compile-time check
var _ log.Logger = (*loggerAdapter)(nil)

// newLoggerAdapter creates a loggerAdapter wrapping the given Logger and
// resolves which optional capability interfaces it implements.
func newLoggerAdapter(inner Logger) *loggerAdapter {
	// Ego's own wrappers do not implement the capability interfaces on behalf of
	// what they wrap, so capabilities are resolved against the backend the
	// wrappers sit in front of. The fields those wrappers contribute come back
	// with it, because routing straight to the backend bypasses them.
	backend, fields := unwrapLogger(inner)
	a := &loggerAdapter{inner: inner, backend: backend, fields: fields}
	a.enabled, _ = backend.(EnabledLogger)
	a.leveled, _ = backend.(LeveledLogger)
	a.ctxLog, _ = backend.(ContextLogger)
	// With() must build on the current wrapper, not the backend, otherwise the
	// wrapper's own fields are lost from the child logger.
	a.fielder, _ = inner.(FieldLogger)
	return a
}

// maxLoggerUnwrapDepth caps the unwrap walk. fieldsLogger.With flattens chained
// calls instead of nesting, so the real depth is one; the cap is a defense against
// an accidentally cyclic internal wrapper.
const maxLoggerUnwrapDepth = 16

// loggerUnwrapper is the unexported protocol Ego's own Logger wrappers use to
// expose the Logger they wrap, together with the fields they contribute to
// every record. It is deliberately not exported: a public Unwrap would become
// an extensibility contract this package would have to keep forever.
type loggerUnwrapper interface {
	unwrapLogger() (Logger, []any)
}

// unwrapLogger walks Ego's own logger wrappers down to the backend that
// actually implements the capability interfaces, collecting the fields those
// wrappers contribute so a caller routing straight to the backend still emits
// them. Fields come back outermost-to-innermost, matching the order the wrapper
// chain would have produced.
//
// Only Ego's own wrappers implement loggerUnwrapper, and a wrapper's entire
// behaviour is prepending its fields, so routing to the returned backend with
// the returned fields prepended is exactly equivalent to routing through the
// chain. The walk stops on a nil or self-referential link and at the depth cap;
// in every one of those cases the returned backend is a wrapper that still
// prepends its own fields, so nothing is dropped or applied twice.
func unwrapLogger(l Logger) (Logger, []any) {
	var fields []any
	current := l
	for range maxLoggerUnwrapDepth {
		u, ok := current.(loggerUnwrapper)
		if !ok {
			return current, fields
		}
		next, contributed := u.unwrapLogger()
		if next == nil || next == current {
			return current, fields
		}
		fields = append(fields, contributed...)
		current = next
	}
	return current, fields
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

// ResolveLogger returns logger when it is usable, and DefaultLogger when it is
// nil or a typed-nil pointer. It lets packages outside the root apply the same
// nil-logger semantics the engine uses, without each of them re-implementing
// the typed-nil detection.
func ResolveLogger(logger Logger) Logger {
	if isNilLogger(logger) {
		return DefaultLogger
	}
	return logger
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

// route sends one record to the backend Logger's context-free method for
// level. Every record — context-bearing or not — goes to the same backend, so
// the accumulated wrapper fields are applied exactly once on either path.
func (a *loggerAdapter) route(level log.Level, msg string, fields []any) {
	switch level {
	case log.DebugLevel:
		a.backend.Debug(msg, fields...)
	case log.WarningLevel:
		a.backend.Warn(msg, fields...)
	case log.ErrorLevel:
		a.backend.Error(msg, fields...)
	default:
		a.backend.Info(msg, fields...)
	}
}

// routeContext sends one record together with the caller's context. It is used
// only on paths where GoAkt actually supplied a context: when the backend
// Logger implements ContextLogger the context is forwarded verbatim, otherwise the
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

// With returns a child logger carrying the given key-value pairs. When the
// inner Logger implements FieldLogger the child is built natively by the
// backend, so a logger with real child-logger support is used instead of the
// adapter replaying accumulated fields on every record.
func (a *loggerAdapter) With(keyValues ...any) log.Logger {
	if len(keyValues) == 0 {
		return a
	}
	if a.fielder != nil {
		return newLoggerAdapter(a.fielder.With(keyValues...))
	}
	child := newLoggerAdapter(a.inner)
	fields := make([]any, 0, len(a.fields)+len(keyValues))
	fields = append(fields, a.fields...)
	fields = append(fields, keyValues...)
	child.fields = fields
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

// WithFields returns a Logger that adds the given key-value pairs to every
// record it writes. It uses the Logger's native child-logger support when
// available (FieldLogger) and otherwise wraps it, so a caller can build a
// component-tagged logger without knowing which capabilities the backend has.
//
// The returned Logger declares only Logger and FieldLogger when it is a
// wrapper, so a type assertion for EnabledLogger, LeveledLogger or
// ContextLogger reports what the backend really supports rather than what
// wrapping added. Ego itself still resolves those capabilities on the wrapped
// Logger, so tagging fields never downgrades level gating or context
// propagation. A nil or typed-nil Logger yields DiscardLogger rather than a
// wrapper that panics on first use.
func WithFields(logger Logger, args ...any) Logger {
	if isNilLogger(logger) {
		return DiscardLogger
	}
	if len(args) == 0 {
		return logger
	}
	if fl, ok := logger.(FieldLogger); ok {
		return fl.With(args...)
	}
	return &fieldsLogger{inner: logger, fields: args}
}

// fieldsLogger is the fallback used by WithFields for a Logger without native
// child-logger support. Prepending its own fields to every record is its entire
// behaviour; it claims none of the wrapped Logger's optional capabilities.
type fieldsLogger struct {
	inner  Logger
	fields []any
}

// fieldsLogger declares only what it genuinely owns. Advertising the wrapped
// Logger's optional interfaces would make `_, ok := logger.(ContextLogger)`
// report a capability the backend may not have. Ego reaches the real
// capabilities through unwrapLogger instead.
var (
	_ Logger          = (*fieldsLogger)(nil)
	_ FieldLogger     = (*fieldsLogger)(nil)
	_ loggerUnwrapper = (*fieldsLogger)(nil)
)

// prepend returns this logger's fields followed by the record's own fields.
func (f *fieldsLogger) prepend(args []any) []any {
	if len(args) == 0 {
		return f.fields
	}
	merged := make([]any, 0, len(f.fields)+len(args))
	merged = append(merged, f.fields...)
	merged = append(merged, args...)
	return merged
}

func (f *fieldsLogger) Debug(msg string, args ...any) { f.inner.Debug(msg, f.prepend(args)...) }
func (f *fieldsLogger) Info(msg string, args ...any)  { f.inner.Info(msg, f.prepend(args)...) }
func (f *fieldsLogger) Warn(msg string, args ...any)  { f.inner.Warn(msg, f.prepend(args)...) }
func (f *fieldsLogger) Error(msg string, args ...any) { f.inner.Error(msg, f.prepend(args)...) }

// unwrapLogger exposes the wrapped Logger and the fields this wrapper
// contributes, so Ego can resolve the backend's real capabilities and still
// emit these fields when it routes there directly.
func (f *fieldsLogger) unwrapLogger() (Logger, []any) { return f.inner, f.fields }

// With flattens chained WithFields calls onto the same wrapped Logger instead
// of nesting wrappers.
func (f *fieldsLogger) With(args ...any) Logger {
	if len(args) == 0 {
		return f
	}
	return &fieldsLogger{inner: f.inner, fields: f.prepend(args)}
}
