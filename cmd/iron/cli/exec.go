package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/cmd/iron/shell"
	"github.com/kuleuven/iron/transfer"
	"github.com/spf13/cobra"
)

func (a *App) exec() *cobra.Command {
	options := transfer.Options{SyncModTime: true}

	var targetEnvironment string

	cmd := &cobra.Command{
		Use:   "exec <local file>",
		Short: "Run an iron command file containing multiple transfer statements",
		Long:  "Read commands in order and stage transfers on one worker. Use a line containing wait before dependent commands. Transfer flags apply to the entire run, not individual lines.",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, word string) ([]string, cobra.ShellCompDirective) {
			if a.inShell {
				return a.CompleteArgs(cmd, args, word)
			}

			return nil, cobra.ShellCompDirectiveDefault
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := os.Open(args[0])
			if err != nil {
				return err
			}

			options.Output = cmd.OutOrStdout()
			err = a.execFile(cmd.Context(), options, targetEnvironment, args[0], file)

			return errors.Join(err, file.Close())
		},
	}

	addTransferFlags(cmd, &options)
	cmd.Flags().BoolVarP(&options.SkipTrash, "delete-skip-trash", "S", false, "Do not move deleted files to trash")
	cmd.Flags().BoolVarP(&options.CopyMetadata, "metadata", "m", false, "Copy AVU metadata on copy operations")
	cmd.Flags().BoolVar(&options.DisableUpdateInPlace, "no-update-in-place", false, "Replace existing objects instead of updating in place")
	cmd.Flags().StringVar(&targetEnvironment, "target-environment", "", "Target environment for cp --remote commands")

	return cmd
}

func (a *App) execFile(ctx context.Context, options transfer.Options, targetEnvironment, filename string, input io.Reader) error {
	var target *iron.Client

	targetWorkdir := a.Workdir

	if targetEnvironment != "" {
		env, dialer, err := a.loader(ctx, targetEnvironment)
		if err != nil {
			return err
		}

		env.GeneratedPasswordTimeout = a.PamTTL
		clientOptions := a.Client.Option()
		clientOptions.DialFunc = dialer

		target, err = iron.New(ctx, env, clientOptions)
		if err != nil {
			return err
		}

		targetWorkdir = env.WorkingDirectory
	}

	var submissionErr error

	err := a.Client.RunWorker(options, func(worker *transfer.Worker) {
		if target != nil {
			worker.RemoteIndexPool = target.API
			worker.RemoteTransferPool = target.API
		}

		submissionErr = a.readCommands(ctx, worker, filename, input, targetWorkdir, target != nil)
	})
	err = errors.Join(submissionErr, err)

	if target != nil {
		err = errors.Join(err, target.Close())
	}

	return err
}

func (a *App) readCommands(ctx context.Context, worker *transfer.Worker, filename string, input io.Reader, targetWorkdir string, remote bool) error {
	reader := bufio.NewReader(input)
	for lineNumber := 1; ; lineNumber++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		line, readErr := reader.ReadString('\n')
		location := fmt.Sprintf("%s:%d", filename, lineNumber)

		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("%s: %w", location, readErr)
		}

		if err := a.runLine(transfer.WithLabel(ctx, location), worker, line, targetWorkdir, remote); err != nil {
			return fmt.Errorf("%s: %w", location, err)
		}

		if errors.Is(readErr, io.EOF) {
			return ctx.Err()
		}
	}
}

func (a *App) runLine(ctx context.Context, worker *transfer.Worker, line, targetWorkdir string, remote bool) error {
	if trimmed := strings.TrimSpace(line); trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil
	}

	args, err := shell.SplitScriptTokens(strings.TrimRight(line, "\r\n"))
	if err != nil {
		return err
	}

	command := a.scriptCommand(ctx, worker, targetWorkdir, remote)
	command.SetArgs(args)

	return command.ExecuteContext(ctx)
}

//nolint:funlen // Keep script command definitions and their handlers together.
func (a *App) scriptCommand(ctx context.Context, worker *transfer.Worker, targetWorkdir string, remote bool) *cobra.Command {
	root := &cobra.Command{Use: a.name, SilenceUsage: true, SilenceErrors: true}
	root.SetContext(ctx)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.AddCommand(&cobra.Command{
		Use: "wait", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return worker.Drain() },
	})

	var remoteCopy bool

	copyCmd := &cobra.Command{
		Use: "cp <path> <target path>", Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			workdir := a.Workdir
			if remoteCopy {
				if !remote {
					return errors.New("cp --remote requires exec --target-environment")
				}

				workdir = targetWorkdir
			}

			paths, err := a.copyPaths(ctx, args, workdir)
			if err != nil {
				return err
			}

			paths.copy(ctx, worker, remoteCopy)

			return ctx.Err()
		},
	}
	copyCmd.Flags().BoolVar(&remoteCopy, "remote", false, "Copy to the target environment")
	root.AddCommand(copyCmd)
	root.AddCommand(&cobra.Command{
		Use: "upload <local file> [target path]", Aliases: []string{uploadAlias}, Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			paths, err := a.uploadPaths(args)
			if err != nil {
				return err
			}

			paths.upload(ctx, worker)

			return ctx.Err()
		},
	})
	root.AddCommand(&cobra.Command{
		Use: "download <path> [local file]", Aliases: []string{downloadAlias}, Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			paths, err := a.downloadPaths(ctx, args)
			if err != nil {
				return err
			}

			paths.download(ctx, worker)

			return ctx.Err()
		},
	})

	root.AddCommand(
		a.scriptWrapAction(ctx, worker, a.mkdir(), transfer.CreateDirectory), //nolint:contextcheck // Cobra constructors inherit ctx through ExecuteContext.
		a.scriptWrapAction(ctx, worker, a.mv(), transfer.TransferFile),       //nolint:contextcheck // Cobra constructors inherit ctx through ExecuteContext.
	)

	return root
}

func (a *App) scriptWrapAction(ctx context.Context, worker *transfer.Worker, cmd *cobra.Command, relabel transfer.Action) *cobra.Command {
	cmd.SetContext(ctx)

	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		worker.Action(ctx, transfer.Task{
			Action: relabel, Source: a.Path(args[0]), Target: a.Path(args[len(args)-1]),
		}, func() error { return run(cmd, args) })

		return ctx.Err()
	}

	return cmd
}
