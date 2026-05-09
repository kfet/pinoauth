package pinoauth

import (
	"crypto/rand"
	"fmt"
)

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
