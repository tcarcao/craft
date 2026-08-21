package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/tcarcao/craft/v2/internal/lsp"
	"github.com/tcarcao/craft/v2/internal/workspace"
)

// TestRunFmtAndServerFormattingAgree is the one test on this branch that
// asserts, rather than assumes, the reason the whole formatting-configuration
// design exists: `craft fmt` and the language server's textDocument/formatting
// must produce byte-identical output for the same file under the same
// .craftfmt. Server.Formatting deliberately ignores params.Options (client
// tabSize/insertSpaces) specifically to protect this equivalence -- see the
// comment above that ignoring in internal/lsp/server.go -- but nothing before
// this test actually checked that the two call sites still agree; the
// guarantee held only because both happen to route through the same
// FormatDocumentCheckedWith function, which a future change to either call
// site could break without any test noticing.
//
// The .craftfmt below is deliberately non-default in every field a flag does
// not name (continuation_indent, both alignment scopes, the outlier guard),
// so this test cannot pass by both sides coincidentally agreeing on the
// built-in defaults, only by actually sharing configuration resolution.
func TestRunFmtAndServerFormattingAgree(t *testing.T) {
	dir := t.TempDir()
	craftfmt := "indent = 3\n" +
		"continuation_indent = 6\n" +
		"\n" +
		"[align]\n" +
		"trailing_comment = \"decl\"\n" +
		"op_annotation    = \"strict\"\n" +
		"outlier_ratio    = 0\n" +
		"outlier_min      = 5\n"
	if err := os.WriteFile(filepath.Join(dir, ".craftfmt"), []byte(craftfmt), 0o644); err != nil {
		t.Fatal(err)
	}

	src := "use_case \"Retry\" {\n" +
		"  when CRON detects a failed charge\n" +
		"    Subscriptions asks Billing for a fresh charge attempt [POST /v1/charges] // billed\n" +
		"    Billing asks Gateway to authorize the card [POST /pay/v2/authorize]\n" +
		"    Billing asks Ledger to record the entry // no annotation here\n" +
		"}\n\n" +
		"services {\n" +
		"F {\n" +
		"contexts: A,\n" +
		"B\n" +
		"}\n" +
		"}\n"

	path := filepath.Join(dir, "a.craft")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	// LSP side: format the ORIGINAL content through the real
	// Server.Formatting handler, resolving .craftfmt from the same path
	// runFmt below will use. This does not touch the file on disk; only
	// params.TextDocument.URI's path is used to find .craftfmt.
	ws := workspace.New(nil)
	docURI := string(uri.File(path))
	ws.Open(docURI, src)
	srv := lsp.NewServerForFormatting(ws, slog.Default())
	edits, err := srv.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	if err != nil {
		t.Fatalf("Server.Formatting: %v", err)
	}
	if len(edits) == 0 {
		t.Fatal("Server.Formatting returned no edits; the fixture is either already canonical or formatting was blocked")
	}
	lspFormatted := edits[0].NewText

	// CLI side: runFmt with no flags, so the whole configuration comes from
	// .craftfmt, exactly as it did for the LSP call above.
	var out, errOut bytes.Buffer
	if code := runFmt([]string{path}, false, fmtOverride{}, &out, &errOut); code != 0 {
		t.Fatalf("runFmt = %d, stderr=%s", code, errOut.String())
	}
	cliFormatted, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	if lspFormatted != string(cliFormatted) {
		t.Errorf("craft fmt and Server.Formatting disagree under the same .craftfmt\nLSP:\n%q\nCLI:\n%q", lspFormatted, cliFormatted)
	}
	if lspFormatted == src {
		t.Fatal("fixture did not need formatting under this .craftfmt; strengthen it so this test exercises something")
	}
}
