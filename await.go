package pinoauth

import (
	"context"
	"errors"
)

// ErrCallbackClosed is returned by [AwaitAuthCode] when the callback
// channel from [StartCallbackServer] is closed before a result arrives —
// for example because the underlying server failed or its parent context
// was cancelled.
var ErrCallbackClosed = errors.New("pinoauth: callback channel closed without a result")

// AwaitAuthCode waits for the OAuth authorization code to arrive via
// either the loopback callback or a manual paste, whichever happens
// first.
//
// resultCh is the channel returned by [StartCallbackServer]. manualInput,
// if non-nil, is invoked in a goroutine to collect a code pasted by the
// user (typical when the browser cannot reach the loopback address —
// e.g. SSH sessions). Its returned string is parsed via
// [ParseAuthorizationInput], so any of the formats that helper accepts
// will work. Pass nil for manualInput to wait only for the callback.
//
// onDismissManualInput, if non-nil, is invoked exactly once after a
// winner is decided so the caller can hide any visible paste prompt.
//
// AwaitAuthCode honours ctx: if ctx is cancelled before a winner, it
// returns ctx.Err(). It does not close the callback server or cancel the
// manualInput goroutine — the caller owns those. In particular,
// manualInput should be ctx-aware if it might block indefinitely on
// user input; otherwise the goroutine may outlive the call.
//
// AwaitAuthCode does not validate that state matches the value passed
// to [StartCallbackServer]; the loopback server already enforces that
// when expectedState is non-empty. For codes arriving via manualInput
// the caller must compare state itself.
func AwaitAuthCode(
	ctx context.Context,
	resultCh <-chan *CallbackResult,
	manualInput func() (string, error),
	onDismissManualInput func(),
) (code, state string, err error) {
	type winner struct {
		code, state string
		err         error
	}
	out := make(chan winner, 2)

	go func() {
		select {
		case res, ok := <-resultCh:
			if !ok || res == nil {
				out <- winner{err: ErrCallbackClosed}
				return
			}
			out <- winner{code: res.Code, state: res.State}
		case <-ctx.Done():
			out <- winner{err: ctx.Err()}
		}
	}()

	if manualInput != nil {
		go func() {
			input, err := manualInput()
			if err != nil {
				out <- winner{err: err}
				return
			}
			c, s := ParseAuthorizationInput(input)
			out <- winner{code: c, state: s}
		}()
	}

	w := <-out
	if onDismissManualInput != nil {
		onDismissManualInput()
	}
	return w.code, w.state, w.err
}
