package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/v2/internal/lsp"
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
