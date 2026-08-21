package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/v2/internal/lsp"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

func fmtCmd() *cobra.Command {
	var check bool

	cmd := &cobra.Command{
		Use:   "fmt [files...]",
		Short: "Format .craft files to canonical form",
		Long: `Format .craft files in place.

Arguments are file paths or glob patterns, including **. Directories are not
walked, matching validate, inspect and generate; pass a glob such as
'**/*.craft' to cover a tree.

With --check nothing is written. The command lists every file that is not
already formatted and exits non-zero, which is the CI gate.

A file the parser cannot fully place is never rewritten, because re-rendering
an incomplete tree loses text. Such a file is reported as skipped, with the
diagnostic that blocked it, and counts as a failure in both modes.`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if info, err := os.Stat(arg); err == nil && info.IsDir() {
					return fmt.Errorf("%s is a directory; pass a glob such as %q",
						arg, filepath.Join(arg, "**", "*.craft"))
				}
			}

			files, err := resolveFiles(args)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				return fmt.Errorf("no files matched %v", args)
			}

			if runFmt(files, check, cmd.OutOrStdout(), cmd.ErrOrStderr()) != 0 {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&check, "check", false,
		"do not write; list files that are not formatted and exit non-zero")
	return cmd
}

// runFmt formats or checks each file and returns the process exit code: 0 when
// every file was already formatted or was rewritten successfully, 1 otherwise.
//
// It is split out of RunE so tests can assert the exit code and the reported
// filenames without the command calling os.Exit on the test process, the same
// split runValidate uses.
func runFmt(files []string, check bool, out, errOut io.Writer) int {
	failed := false

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			fmt.Fprintf(errOut, "%s: %v\n", file, err)
			failed = true
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(errOut, "%s: %v\n", file, err)
			failed = true
			continue
		}

		formatted, blocked := lsp.FormatDocumentChecked(string(content))
		if blocked != nil {
			// Reported rather than passed over in silence: the file is left on
			// disk exactly as it was, and a caller told only "nothing changed"
			// would reasonably read that as "already formatted".
			fmt.Fprintf(errOut, "%s: skipped, not formatted: %s [%s]\n",
				file, blocked.Message, blocked.Code)
			failed = true
			continue
		}
		if formatted == string(content) {
			continue
		}

		// Never write text that has not been read back. FormatDocument already
		// refuses to return output that changes anything but whitespace, and
		// this reparse is the second lock on the same door: an in-place bulk
		// rewriter shipping to Homebrew is the wrong place to trust a single
		// check. A file that would come back broken BY FORMATTING is left
		// exactly as it was.
		//
		// "By formatting" is load-bearing: the guard must compare against the
		// input's own diagnostics, not just inspect the output's. A file can
		// carry a pre-existing sema error (a duplicate name, say) that has
		// nothing to do with whitespace; refusing to format such a file
		// forever, under a message that says formatting broke it, would be a
		// lie, and would mean a single bad file blocks `craft fmt` from ever
		// normalising it. firstNewError is what tells "this was already
		// broken" apart from "formatting broke this".
		_, origDiags, _ := craft.Parse(file, content)
		if _, diags, err := craft.Parse(file, []byte(formatted)); err != nil {
			fmt.Fprintf(errOut, "%s: skipped, formatted output could not be reparsed: %v\n", file, err)
			failed = true
			continue
		} else if bad := firstNewError(diags, origDiags); bad != nil {
			fmt.Fprintf(errOut, "%s: skipped, formatting would have broken the file: %s [%s]\n",
				file, bad.Message, bad.Code)
			failed = true
			continue
		}

		if check {
			fmt.Fprintf(out, "%s\n", file)
			failed = true
			continue
		}
		if err := os.WriteFile(file, []byte(formatted), info.Mode().Perm()); err != nil {
			fmt.Fprintf(errOut, "%s: %v\n", file, err)
			failed = true
			continue
		}
		fmt.Fprintf(out, "%s\n", file)
	}

	if failed {
		return 1
	}
	return 0
}

// firstNewError returns the first error-severity diagnostic in diags that is
// not already present in orig, or nil if formatting introduced nothing new.
//
// "Present" is diagnostic identity, and identity here is Code+Message, not
// Range. Range is exactly what formatting is allowed to move: indent width
// shifts every nested token's column, and top-level declarations are forced
// onto a single blank line regardless of how many the author wrote, which can
// shift line numbers too. Keying on Range would make the guard fire on
// legitimate whitespace movement instead of on an actual new defect.
//
// Message is not perfectly stable either: craft/sema/duplicate-name embeds a
// line number in its own text ("already declared (first seen at line N)"),
// so a reformat that also shifts an EARLIER line could in principle change a
// pre-existing error's message and make this treat it as new. That failure
// mode is conservative, not unsafe: it refuses to format a file it could
// safely have formatted, rather than silently reporting formatting damage
// that was not there. That is the direction this guard is allowed to err in.
func firstNewError(diags, orig []craft.Diagnostic) *craft.Diagnostic {
	origErrs := make(map[string]bool, len(orig))
	for _, d := range orig {
		if d.Severity == craft.SeverityError {
			origErrs[d.Code+"\x00"+d.Message] = true
		}
	}
	for i := range diags {
		if diags[i].Severity != craft.SeverityError {
			continue
		}
		if origErrs[diags[i].Code+"\x00"+diags[i].Message] {
			continue
		}
		return &diags[i]
	}
	return nil
}
