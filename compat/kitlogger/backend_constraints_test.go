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
	"bufio"
	"os"
	"strings"
	"testing"
	"time"

	kitlogger "github.com/pablogore/kit-logger/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below record why eGo's core module does not import kit-logger and
// why ego.DefaultLogger is still slog-backed. They assert properties of
// kit-logger itself, not of this adapter, and they are here — in the module
// that already pays for kit-logger — precisely so the root module does not have
// to import it to state them.
//
// If either stops holding, the tradeoff changes and the deferred decision
// (making kit-logger eGo's default) should be revisited. That is the point of
// asserting them instead of writing them down.

// TestImportingKitLoggerRegistersAGlobalCollector pins the side effect that
// keeps kit-logger out of eGo's core module: the package registers a Prometheus
// collector on the default registry from an init function, so *importing* it —
// not configuring it, not using it — mutates process-global state.
//
// eGo's core cannot impose that on every application that merely links eGo.
func TestImportingKitLoggerRegistersAGlobalCollector(t *testing.T) {
	// Registering the same metric again is the probe: it succeeds on a clean
	// registry, and on a dirty one it reports the collision. The description
	// must match kit-logger's own, otherwise Prometheus rejects the
	// registration for an inconsistent help string instead — which would prove
	// the same point, but less precisely.
	duplicate := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "slog_logged_total",
			Help: "Total number of slog log entries.",
		},
		[]string{"level"},
	)

	err := prometheus.DefaultRegisterer.Register(duplicate)

	require.Error(t, err, "kit-logger no longer claims slog_logged_total on import")
	assert.IsType(t, prometheus.AlreadyRegisteredError{}, err)
}

// TestDefaultKitLoggerConfigIsNotSilent pins the second reason: a kit-logger
// built with the zero Config writes a record to stdout as a side effect of
// construction, and installs a handler that resolves the calling frame for
// every record after that.
//
// An eGo default has to be cheap and predictable — ego.New must not print
// anything the caller did not ask for — so eGo cannot build this logger on the
// caller's behalf. An application that wants it constructs it itself, which is
// exactly what ego.WithLogger is for.
func TestDefaultKitLoggerConfigIsNotSilent(t *testing.T) {
	assert.NotEmpty(t, captureStdout(t, func() {
		_ = kitlogger.New(kitlogger.Config{})
	}), "kit-logger's default config no longer logs on construction")

	// The escape hatch, and what every test in this package uses: supplying a
	// handler bypasses the whole chain, the construction record included.
	assert.Empty(t, captureStdout(t, func() {
		_ = kitlogger.New(kitlogger.Config{Handler: newCaptureHandler(0)})
	}))
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it
// wrote. It reads concurrently so a write larger than the pipe buffer cannot
// deadlock, and restores os.Stdout before returning.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	original := os.Stdout
	os.Stdout = writer

	captured := make(chan string, 1)
	go func() {
		var out strings.Builder
		_, _ = bufio.NewReader(reader).WriteTo(&out)
		captured <- out.String()
	}()

	fn()

	os.Stdout = original
	require.NoError(t, writer.Close())

	select {
	case out := <-captured:
		return out
	case <-time.After(5 * time.Second):
		t.Fatal("timed out reading captured stdout")
		return ""
	}
}
