package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

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

// digitsInMessage matches every run of digits in a diagnostic message, so
// diagKey can fold out embedded line numbers before comparing.
var digitsInMessage = regexp.MustCompile(`\d+`)

// diagKey is the identity firstNewError compares on: Code plus message
// SHAPE, not Range and not the raw message.
//
// Range is exactly what formatting is allowed to move: indent width shifts
// every nested token's column, and top-level declarations are forced onto a
// single blank line regardless of how many the author wrote, which can shift
// line numbers too. Keying on Range would make the guard fire on legitimate
// whitespace movement instead of on an actual new defect.
//
// The raw Message is not safe either, for the same reason one layer down:
// craft/sema/duplicate-name embeds the FIRST occurrence's line number in its
// own text ("already declared (first seen at line N)"), so a reformat that
// shifts an earlier line changes that text even though the error itself is
// unchanged. Folding every digit run to a single placeholder removes that
// whole class: two messages that differ only in which line number they quote
// still key the same, while two messages that differ in what they are
// actually saying (a different duplicated name, a different diagnostic
// shape) still key apart, because the name is not digits.
func diagKey(d craft.Diagnostic) string {
	return d.Code + "\x00" + digitsInMessage.ReplaceAllString(d.Message, "#")
}

// firstNewError returns the first error-severity diagnostic in diags whose
// count, keyed by diagKey, exceeds how many times that key appears in orig,
// or nil if formatting introduced nothing new.
//
// Counting rather than set membership matters if a single formatting pass
// ever multiplied an existing error (a walker bug duplicating a token, say):
// under set membership one occurrence in orig would make the same key look
// "already present" no matter how many more copies diags carried, and every
// one of them would be waved through.
func firstNewError(diags, orig []craft.Diagnostic) *craft.Diagnostic {
	budget := make(map[string]int, len(orig))
	for _, d := range orig {
		if d.Severity == craft.SeverityError {
			budget[diagKey(d)]++
		}
	}
	for i := range diags {
		if diags[i].Severity != craft.SeverityError {
			continue
		}
		key := diagKey(diags[i])
		if budget[key] > 0 {
			budget[key]--
			continue
		}
		return &diags[i]
	}
	return nil
}
