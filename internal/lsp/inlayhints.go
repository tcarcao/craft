package lsp

import (
	"context"
	"encoding/json"
	"log/slog"

	"go.lsp.dev/jsonrpc2"

	"github.com/tcarcao/craft/internal/sema"
	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/internal/workspace"
)

// inlayHintParams mirrors the LSP 3.17 InlayHintParams. Defined locally because
// go.lsp.dev/protocol@v0.12.0 (LSP 3.15.3) predates the InlayHint feature.
type inlayHintParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

// inlayHint mirrors the LSP 3.17 InlayHint object.
type inlayHint struct {
	Position    inlayPosition `json:"position"`
	Label       string        `json:"label"`
	PaddingLeft bool          `json:"paddingLeft,omitempty"`
}

type inlayPosition struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

// withInlayHints returns a handler that intercepts two methods:
//   - "initialize": patches the response to advertise inlayHintProvider capability.
//   - "textDocument/inlayHint": handled directly; never forwarded to h.
func withInlayHints(h jsonrpc2.Handler, ws *workspace.Workspace, logger *slog.Logger) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		switch req.Method() {
		case "initialize":
			patchedReply := func(ctx context.Context, result interface{}, err error) error {
				if err != nil || result == nil {
					return reply(ctx, result, err)
				}
				b, jsonErr := json.Marshal(result)
				if jsonErr != nil {
					return reply(ctx, result, nil)
				}
				var obj map[string]interface{}
				if jsonErr = json.Unmarshal(b, &obj); jsonErr != nil {
					return reply(ctx, result, nil)
				}
				if caps, ok := obj["capabilities"].(map[string]interface{}); ok {
					caps["inlayHintProvider"] = true
				} else {
					logger.Warn("inlay hints: initialize response has no capabilities object; inlayHintProvider not advertised")
				}
				return reply(ctx, obj, nil)
			}
			return h(ctx, patchedReply, req)

		case "textDocument/inlayHint":
			return handleInlayHint(ctx, reply, req, ws, logger)

		default:
			return h(ctx, reply, req)
		}
	}
}

func handleInlayHint(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request, ws *workspace.Workspace, _ *slog.Logger) error {
	var params inlayHintParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	file := ws.Get(uri)
	if file == nil {
		return reply(ctx, []inlayHint{}, nil)
	}

	rm := ws.ResolutionMap()
	tree := syntax.Root(file.Green)
	li := file.LineIndex
	src := file.Content

	var hints []inlayHint
	for _, svc := range syntax.AsFile(tree).Services() {
		svcName := ""
		if tok := svc.Name(); tok != nil {
			svcName = tok.Text()
		}
		if svcName == "" {
			continue
		}
		for _, ctxTok := range svc.ContextTokens() {
			// ContextTokens() returns raw tokens (possibly SyntaxKindString);
			// the resolution map is keyed by unquoted names, so read content
			// via StringAwareText rather than the raw Text().
			ctxName := syntax.StringAwareText(ctxTok)
			domSym, ok := sema.ResolveServiceContext(rm, uri, svcName, ctxName)
			// Skip when unresolved, or when the context name IS the domain name
			// (direct-domain reference — the hint would just repeat the visible text).
			if !ok || domSym.Name == ctxName {
				continue
			}
			// Place hint at the character immediately after the context name token.
			endOffset := ctxTok.TextRange().End
			pos := lspPosFromOffset(li, src, endOffset)
			hints = append(hints, inlayHint{
				Position: inlayPosition{Line: pos.Line, Character: pos.Character},
				Label:    " ← " + domSym.Name,
			})
		}
	}
	return reply(ctx, hints, nil)
}
