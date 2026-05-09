package pinoauth

import (
	"fmt"
	"net"
	"net/http"
)

// mustServeLoopback runs srv.Serve(ln) and panics on any error other
// than http.ErrServerClosed.
//
// After a successful net.Listen on a loopback address owned by this
// package, http.Server.Serve only returns once the listener is closed
// (yielding ErrServerClosed via srv.Close) or the accept loop fails
// fatally — the latter being unreachable for a TCP listener that
// nothing else can poison. Treating an unexpected Serve error as a
// panic keeps the caller free of an impossible "channel closed
// because Serve died" branch.
func mustServeLoopback(srv *http.Server, ln net.Listener) {
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		panic(fmt.Errorf("pinoauth: serve loopback: %w", err))
	}
}
