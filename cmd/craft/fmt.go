package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/v2/internal/fmtconfig"
	"github.com/tcarcao/craft/v2/internal/lsp"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

// fmtOverride carries the subset of formatting fields the user actually set
// on the command line (cmd.Flags().Changed), one field at a time. This is
// deliberately not a bare fmtconfig.Config: a Config cannot tell "the user
// set this to the default value" apart from "the user did not touch this
// field at all", and resolution must be able to tell those apart, because a
// single flag must only ever override the field it names. Every other field
// still comes from .craftfmt (or the built-in defaults, if there is no
// .craftfmt), never from a flag the user did not pass.
type fmtOverride struct {
	indent    int
	hasIndent bool

	continuationIndent    int
	hasContinuationIndent bool

	trailingComment    fmtconfig.Scope
	hasTrailingComment bool
}

// any reports whether at least one flag was set.
func (o fmtOverride) any() bool {
	return o.hasIndent || o.hasContinuationIndent || o.hasTrailingComment
}

// apply layers the fields the user set on top of cfg (the per-file resolved
// configuration), field by field, leaving every unset field exactly as cfg
// had it. cfg is passed by value, so this never mutates the caller's copy.
func (o fmtOverride) apply(cfg fmtconfig.Config) fmtconfig.Config {
	if o.hasIndent {
		cfg.Indent = o.indent
	}
	if o.hasContinuationIndent {
		cfg.ContinuationIndent = o.continuationIndent
	}
	if o.hasTrailingComment {
		cfg.Align.TrailingComment = o.trailingComment
	}
	return cfg
}

func fmtCmd() *cobra.Command {
	var check bool
	var indent, contIndent int
	var trailingComment string

	cmd := &cobra.Command{
		Use:   "fmt [files...]",
		Short: "Format .craft files to canonical form",
		Long: `Format .craft files in place.

Arguments are file paths or glob patterns, including **. Directories are not
walked, matching validate, inspect and generate; pass a glob such as
'**/*.craft' to cover a tree.

Each file is formatted according to the nearest .craftfmt found by walking up
from that file's own directory, or the built-in defaults if there is none.
Resolution happens per field, per file: --indent, --continuation-indent and
--align-trailing-comment each override only the field they name, and only
when the flag is actually passed. A flag left at its default is not
considered "set" and never overrides .craftfmt; every field a flag does not
name still comes from that file's own .craftfmt, or the defaults if it has
none.

A single invocation can span files from more than one workspace (a glob
covering two directory trees, say), and each file's .craftfmt is resolved
independently, so they are free to disagree, flags aside.

With --check nothing is written. The command lists every file that is not
already formatted and exits non-zero, which is the CI gate.

A file the parser cannot fully place is never rewritten, because re-rendering
an incomplete tree loses text. Such a file is reported as skipped, with the
diagnostic that blocked it, and counts as a failure in both modes.

A malformed .craftfmt, an invalid value, or an unknown key is reported
per-file, on stderr, and counts as a failure rather than silently falling
back to the defaults. This is checked for every file, whether or not a flag
was also passed, since a flag only ever overrides the field it names.`,
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

			var override fmtOverride
			if cmd.Flags().Changed("indent") {
				override.indent, override.hasIndent = indent, true
			}
			if cmd.Flags().Changed("continuation-indent") {
				override.continuationIndent, override.hasContinuationIndent = contIndent, true
			}
			if cmd.Flags().Changed("align-trailing-comment") {
				override.trailingComment, override.hasTrailingComment = fmtconfig.Scope(trailingComment), true
			}
			if override.any() {
				// Validate the flag values in isolation, against the
				// defaults, before touching any file: fmtconfig.Validate
				// checks each field independently (no cross-field rules),
				// so this catches a bad flag value exactly as reliably as
				// validating the field's eventual per-file merge would,
				// without waiting to resolve a single .craftfmt first.
				if err := override.apply(fmtconfig.Defaults()).Validate(); err != nil {
					return err
				}
			}

			if runFmt(files, check, override, cmd.OutOrStdout(), cmd.ErrOrStderr()) != 0 {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&check, "check", false,
		"do not write; list files that are not formatted and exit non-zero")
	cmd.Flags().IntVar(&indent, "indent", 0,
		"indent width in spaces; overrides .craftfmt")
	cmd.Flags().IntVar(&contIndent, "continuation-indent", 0,
		"extra indent for a wrapped list continuation; overrides .craftfmt")
	cmd.Flags().StringVar(&trailingComment, "align-trailing-comment", "",
		"trailing comment alignment scope: off, strict, block, file, decl; overrides .craftfmt")
	return cmd
}

// runFmt formats or checks each file and returns the process exit code: 0 when
// every file was already formatted or was rewritten successfully, 1 otherwise.
//
// It is split out of RunE so tests can assert the exit code and the reported
// filenames without the command calling os.Exit on the test process, the same
// split runValidate uses.
//
// override is the CLI-flag half of resolution. Every file's .craftfmt (or the
// defaults, if it has none) is resolved unconditionally -- a malformed
// .craftfmt must be visible regardless of whether any flag was passed -- and
// then override.apply layers only the fields the user actually set on top,
// per file. This is deliberately per field, not "override wins outright":
// a single --indent must not silently discard a .craftfmt's unrelated
// align.trailing_comment setting.
func runFmt(files []string, check bool, override fmtOverride, out, errOut io.Writer) int {
	failed := false

	// resolveCache memoizes fmtconfig.Resolve by directory so a large glob
	// with many files per directory does not re-walk the filesystem looking
	// for .craftfmt once per file. It is local to this call, so it cannot
	// outlive a single invocation or leak configuration between them.
	type cacheEntry struct {
		cfg fmtconfig.Config
		err error
	}
	resolveCache := make(map[string]cacheEntry)
	resolveCfg := func(file string) (fmtconfig.Config, error) {
		dir := filepath.Dir(file)
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		if entry, ok := resolveCache[dir]; ok {
			return entry.cfg, entry.err
		}
		cfg, err := fmtconfig.Resolve(file)
		resolveCache[dir] = cacheEntry{cfg, err}
		return cfg, err
	}

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

		resolved, err := resolveCfg(file)
		if err != nil {
			// A malformed .craftfmt must be visible per-file, not silently
			// treated as "no config" and defaulted -- even when a flag was
			// also passed, since the flag only overrides the field it
			// names, and every other field still needs this file's own
			// .craftfmt.
			fmt.Fprintf(errOut, "%s: %v\n", file, err)
			failed = true
			continue
		}
		cfg := override.apply(resolved)

		formatted, blocked := lsp.FormatDocumentCheckedWith(string(content), cfg)
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
		//
		// The parse error from this first call is deliberately discarded.
		// If parsing the ORIGINAL content fails outright, origDiags comes
		// back empty, so firstNewError's budget starts at zero and any
		// error-severity diagnostic surfaced by parsing the formatted output
		// below is reported as new -- which collapses this guard back to the
		// old any-error-fails behaviour for that one file. That is the safe
		// direction (this can only make the guard more conservative, never
		// less), so it is left as a silent fallback rather than surfaced as
		// its own failure mode.
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
// now key the same.
//
// This is not airtight the other way: a Craft identifier may itself contain
// digits ("svc1" declared twice vs. "svc2" declared twice both fold to
// "svc# already declared ..."), so diagKey can, in principle, key two
// genuinely different duplicate-name errors together. That does not matter
// here. firstNewError only ever compares diagnostics the parser produced,
// and this function's caller (runFmt) never introduces or renames an
// identifier -- formatting changes whitespace and nothing else, guarded
// separately by contentDrift -- so "two different names, coincidentally
// folding to the same key" is not a shape a reformat can manufacture. The
// safety this guard needs is narrower than "keys apart the way messages
// differ in general," and that narrower property still holds.
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
