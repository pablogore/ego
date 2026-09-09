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

// Package kitlogger adapts github.com/pablogore/kit-logger to eGo's Logger
// seam. It is the officially recommended logging backend for eGo.
//
// eGo's core stays backend-neutral: the root module never imports kit-logger,
// so only applications that opt in pay for its dependency graph. This package
// lives in its own Go module for exactly that reason.
//
//	kl := kitlogger.New(kitlogger.Config{Level: "info", Format: "json"})
//	engine, err := ego.NewEngine(ego.NewConfig(store, ego.WithLogger(egokit.New(kl))))
//
// Lifecycle stays with the caller. The adapter never constructs, flushes, or
// closes the kit-logger it is handed: whoever created the logger owns Sync.
// Unwrap returns it when an application needs to reach it again.
package kitlogger

import (
	"context"
	"log/slog"
	"reflect"

	kitlogger "github.com/pablogore/kit-logger/pkg/logger"
	"github.com/tochemey/ego/v4"
)

// Adapter presents a kit-logger Logger as an ego.Logger.
//
// It implements every eGo capability kit-logger can genuinely back:
//
//   - ego.Logger        — the base seam.
//   - ego.ContextLogger — kit-logger has native *Context methods; the caller's
//     context is forwarded verbatim, never replaced by a fabricated one.
//   - ego.FieldLogger   — child loggers are built by kit-logger's own With, so
//     fields are stored once, by the backend, instead of being replayed here.
//   - ego.EnabledLogger — level checks are answered by the backend's own
//     *slog.Logger, so no level is inferred, parsed, or translated.
//
// ego.LeveledLogger is deliberately not implemented: kit-logger exposes
// SetLevel but no level getter, so satisfying it would mean inventing a level
// string the backend never reported. EnabledLogger already expresses the same
// capability exactly, and eGo prefers it over LeveledLogger anyway.
type Adapter struct {
	logger kitlogger.Logger

	// slogger is the backend's own *slog.Logger, resolved once: the dynamic
	// type of logger cannot change, so neither can the logger Slog() returns.
	// It answers Enabled without a round trip through kit-logger's variadic
	// argument scanning.
	slogger *slog.Logger
}

// Adapter implements every capability kit-logger can back, and none it cannot.
var (
	_ ego.Logger        = (*Adapter)(nil)
	_ ego.ContextLogger = (*Adapter)(nil)
	_ ego.FieldLogger   = (*Adapter)(nil)
	_ ego.EnabledLogger = (*Adapter)(nil)
)

// New adapts a kit-logger Logger to eGo's Logger seam.
//
// A nil or typed-nil logger yields ego.DefaultLogger rather than an error or a
// value that panics on first use. That mirrors ego.ResolveLogger, the semantics
// every other eGo entry point already applies to a missing Logger, and it keeps
// the signature usable inline in ego.WithLogger without forcing the caller to
// handle an error that has one obvious recovery.
func New(logger kitlogger.Logger) ego.Logger {
	if isNil(logger) {
		return ego.DefaultLogger
	}
	return newAdapter(logger)
}

// newAdapter builds an Adapter over a logger already known to be usable.
func newAdapter(logger kitlogger.Logger) *Adapter {
	return &Adapter{logger: logger, slogger: logger.Slog()}
}

// isNil reports whether logger is nil or a typed-nil pointer. A typed-nil
// interface value is non-nil at the interface level but wraps a nil pointer,
// which would panic on the first log call.
func isNil(logger kitlogger.Logger) bool {
	if logger == nil {
		return true
	}
	v := reflect.ValueOf(logger)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// Unwrap returns the kit-logger Logger this Adapter writes to, so an
// application can reach the backend it owns — to call Sync before exit, for
// instance. eGo itself never calls it.
func (a *Adapter) Unwrap() kitlogger.Logger { return a.logger }

func (a *Adapter) Debug(msg string, args ...any) { a.logger.Debug(msg, args...) }
func (a *Adapter) Info(msg string, args ...any)  { a.logger.Info(msg, args...) }
func (a *Adapter) Warn(msg string, args ...any)  { a.logger.Warn(msg, args...) }
func (a *Adapter) Error(msg string, args ...any) { a.logger.Error(msg, args...) }

// The *Context methods forward the caller's context unchanged. kit-logger
// hands it straight to log/slog, which passes it to the handler, so a
// context-aware handler sees exactly the context the eGo call site carried.

func (a *Adapter) DebugContext(ctx context.Context, msg string, args ...any) {
	a.logger.DebugContext(ctx, msg, args...)
}

func (a *Adapter) InfoContext(ctx context.Context, msg string, args ...any) {
	a.logger.InfoContext(ctx, msg, args...)
}

func (a *Adapter) WarnContext(ctx context.Context, msg string, args ...any) {
	a.logger.WarnContext(ctx, msg, args...)
}

func (a *Adapter) ErrorContext(ctx context.Context, msg string, args ...any) {
	a.logger.ErrorContext(ctx, msg, args...)
}

// With returns a child Adapter over kit-logger's own child logger. The fields
// live in the backend, so chained calls accumulate there — this package keeps
// no field slice of its own and therefore cannot duplicate or drop one.
func (a *Adapter) With(args ...any) ego.Logger {
	if len(args) == 0 {
		return a
	}
	return newAdapter(a.logger.With(args...))
}

// Enabled asks the backend's own *slog.Logger, so a level change made through
// kit-logger's SetLevel is honored on the very next check instead of being
// snapshotted at construction. The context is the one the log call carries,
// which lets a context-sensitive handler gate on it.
//
// A backend whose Slog returns nil declared no level, so every level reports
// enabled: eGo's rule is that an adapter never filters on behalf of a Logger
// that never said it wanted filtering.
func (a *Adapter) Enabled(ctx context.Context, level slog.Level) bool {
	if a.slogger == nil {
		return true
	}
	return a.slogger.Enabled(ctx, level)
}
