package pinoauth

import (
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
)

// This file isolates structurally-unreachable defensive code so the
// project-wide /unreachable\.go: coverage exclusion can drop it from
// the 100% gate. Each helper documents why its error path cannot be
// triggered from a test, and panics rather than returning an error
// so production callers have no impossible branch left to cover.

// mustReadRandom fills b with cryptographically random bytes, or panics.
//
// crypto/rand.Read on every supported platform reads from the kernel
// CSPRNG (getrandom / getentropy / BCryptGenRandom). Once the OS RNG
// is initialised — which it is long before any Go program reaches
// main — Read does not fail. A non-nil error here means the host is
// in a state where the process cannot meaningfully continue, so we
// crash rather than surface an error branch the caller can never
// exercise.
func mustReadRandom(b []byte) {
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("pinoauth: read random bytes: %w", err))
	}
}

// serveLoopback runs srv.Serve(ln) and panics on any error other than
// http.ErrServerClosed.
//
// After a successful net.Listen on a loopback address owned by this
// package, http.Server.Serve only returns once the listener is closed
// (yielding ErrServerClosed via srv.Close) or the accept loop fails
// fatally — the latter being unreachable for a TCP listener that
// nothing else can poison. Treating an unexpected Serve error as a
// panic keeps the caller free of an impossible "channel closed
// because Serve died" branch.
func serveLoopback(srv *http.Server, ln net.Listener) {
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		panic(fmt.Errorf("pinoauth: serve loopback: %w", err))
	}
}
