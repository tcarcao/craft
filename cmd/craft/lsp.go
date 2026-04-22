package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/internal/lsp"
)

func lspCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "Start the Craft language server (LSP) on stdio",
		Long: `Start the Craft Language Server Protocol server.

The server communicates over stdin/stdout using the LSP JSON-RPC 2.0 protocol.
Logs are written to stderr. Use --log-file to redirect them.

This subcommand is intended to be spawned by LSP clients such as VS Code.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			logFile, _ := cmd.Flags().GetString("log-file")
			if logFile != "" {
				f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
				if err != nil {
					return fmt.Errorf("opening log file: %w", err)
				}
				defer f.Close()
				slog.SetDefault(slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			slog.Info("craft lsp: starting", "pid", os.Getpid())
			if err := lsp.Serve(ctx, os.Stdin, os.Stdout); err != nil {
				slog.Error("craft lsp: server stopped", "error", err)
				return err
			}
			return nil
		},
	}

	cmd.Flags().String("log-file", "", "Redirect log output to a file instead of stderr")
	return cmd
}
