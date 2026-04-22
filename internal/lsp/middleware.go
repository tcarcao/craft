package lsp

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"go.lsp.dev/jsonrpc2"
)

// withPanicRecovery wraps h so that any panic inside a handler is caught,
// logged via slog, and turned into an LSP InternalError response.
// This is the Q17 "handler tier" recovery: a panicking handler degrades only
// that one request; other requests continue normally.
func withPanicRecovery(h jsonrpc2.Handler, logger *slog.Logger) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logger.Error("craft lsp: handler panic recovered",
					"method", req.Method(),
					"panic", fmt.Sprintf("%v", r),
					"stack", string(stack),
				)
				retErr = reply(ctx, nil, fmt.Errorf("internal error: %v", r))
			}
		}()
		return h(ctx, reply, req)
	}
}
