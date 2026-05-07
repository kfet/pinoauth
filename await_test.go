package pinoauth

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestAwaitAuthCode_CallbackWins(t *testing.T) {
	ch := make(chan *CallbackResult, 1)
	ch <- &CallbackResult{Code: "c1", State: "s1"}

	var dismissed atomic.Bool
	code, state, err := AwaitAuthCode(context.Background(), ch, nil, func() {
		dismissed.Store(true)
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "c1" || state != "s1" {
		t.Errorf("got code=%q state=%q, want c1/s1", code, state)
	}
	if !dismissed.Load() {
		t.Error("onDismissManualInput not invoked")
	}
}

func TestAwaitAuthCode_ManualWins(t *testing.T) {
	ch := make(chan *CallbackResult, 1) // never written

	manual := func() (string, error) {
		return "https://example.com/cb?code=mc&state=ms", nil
	}

	var dismissed atomic.Bool
	code, state, err := AwaitAuthCode(context.Background(), ch, manual, func() {
		dismissed.Store(true)
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "mc" || state != "ms" {
		t.Errorf("got code=%q state=%q, want mc/ms", code, state)
	}
	if !dismissed.Load() {
		t.Error("onDismissManualInput not invoked")
	}
}

func TestAwaitAuthCode_ManualReturnsError(t *testing.T) {
	ch := make(chan *CallbackResult, 1) // never written
	wantErr := errors.New("user cancelled")
	manual := func() (string, error) { return "", wantErr }

	_, _, err := AwaitAuthCode(context.Background(), ch, manual, nil)
	if !errors.Is(err, wantErr) {
		t.Errorf("got err=%v, want %v", err, wantErr)
	}
}

func TestAwaitAuthCode_ContextCancelled(t *testing.T) {
	ch := make(chan *CallbackResult, 1) // never written
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := AwaitAuthCode(ctx, ch, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got err=%v, want context.Canceled", err)
	}
}

func TestAwaitAuthCode_CallbackChannelClosed(t *testing.T) {
	ch := make(chan *CallbackResult)
	close(ch)

	_, _, err := AwaitAuthCode(context.Background(), ch, nil, nil)
	if !errors.Is(err, ErrCallbackClosed) {
		t.Errorf("got err=%v, want ErrCallbackClosed", err)
	}
}

func TestAwaitAuthCode_NoDismissCallbackIsFine(t *testing.T) {
	ch := make(chan *CallbackResult, 1)
	ch <- &CallbackResult{Code: "x"}
	// onDismissManualInput == nil should not panic.
	code, _, err := AwaitAuthCode(context.Background(), ch, nil, nil)
	if err != nil || code != "x" {
		t.Fatalf("unexpected: code=%q err=%v", code, err)
	}
}

// Verifies that when the callback wins, a still-pending manualInput
// goroutine doesn't deadlock the call. The buffered channel makes the
// loser's send non-blocking; this test would hang if that contract
// regressed.
func TestAwaitAuthCode_ManualLoserDoesNotBlock(t *testing.T) {
	ch := make(chan *CallbackResult, 1)
	ch <- &CallbackResult{Code: "fast"}

	manualDone := make(chan struct{})
	manual := func() (string, error) {
		// Simulate slow user; finishes after callback wins.
		time.Sleep(20 * time.Millisecond)
		defer close(manualDone)
		return "ignored", nil
	}

	done := make(chan struct{})
	go func() {
		_, _, _ = AwaitAuthCode(context.Background(), ch, manual, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("AwaitAuthCode did not return — manual-loser send may have blocked")
	}

	// Drain the manual goroutine so it doesn't outlive the test.
	<-manualDone
}
