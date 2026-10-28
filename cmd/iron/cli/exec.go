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

const execDescription = `Read and execute an iron command file, one command per line. Commands are
submitted to one transfer worker in file order; transfers may run concurrently.
Use a line containing only "wait" to wait for all previously submitted work
before continuing, for example when a later command depends on an earlier one.

Blank lines and lines whose first non-space character is # are ignored. Quote
arguments containing spaces with single or double quotes. Each non-comment line
must contain exactly one command: shell operators such as ;, |, &&, redirection,
and command substitution are not supported. Errors identify the script file and
line number.

Available commands:

  cp <path> <target path> [flags]
      Copy an iRODS object or collection within the current environment.
      Collection paths use a trailing slash to distinguish copying the
      collection's contents from copying it as a subcollection. Add --remote
      to copy to the environment selected with --target-environment.

  upload|put <local file or directory> [target path] [flags]
      Upload a local file or directory to iRODS. If the target is omitted, the
      current iRODS working directory is used. A directory target must end in
      a slash.

  download|get <iRODS path> [local path] [flags]
      Download an iRODS object or collection. If the local target is omitted,
      the current local directory is used. A directory target must end in a
      path separator.

  mkdir <target path> [-p|--parents]
      Create an iRODS collection. Use --parents to create missing parent
      collections as needed.

  mv <path> <target path> [--preserve-mtime]
      Move or rename an iRODS object or collection. A target ending in a slash
      places a single source inside that collection. A source collection ending
      in a slash moves its contents into the target collection, which must also
      end in a slash. By default, moving updates modification time; use
      --preserve-mtime to retain it.

Transfer flags go on the command they affect. cp, upload/put, and download/get
accept --exclusive, --newer, --delete, --checksum, --verify-checksum,
--sync-modtime[=false], --include, and --ignore. The collection-only flags
--newer, --delete, --checksum, --include, and --ignore apply only when
transferring a collection or directory. cp and upload/put also accept
--delete-skip-trash and --no-update-in-place; cp additionally accepts
--metadata.

Run-wide options:

  --threads int          Number of transfer threads to use (default 5)
  --dry-run              Print planned actions without transferring or deleting
                         files. Server-side checksums may still be computed
                         and stored.
  --target-environment   Environment used by cp --remote. Other commands, and
                         cp without --remote, use the current environment.

Arguments to transfer commands may use paths relative to the relevant working
directory. Local upload and download paths are resolved relative to the local
process working directory. Examples:

  cp /zone/home/alice/source /zone/home/alice/copy
  cp --remote /zone/home/alice/source /zone/home/alice/copy
  upload "local files/report.csv" /zone/home/alice/
  download /zone/home/alice/report.csv "local files/"
  mkdir --parents /zone/home/alice/archive/2025
  mv /zone/home/alice/old.csv /zone/home/alice/archive/
  wait`

func (a *App) exec() *cobra.Command {
	var options transfer.Options

	var targetEnvironment string

	cmd := &cobra.Command{
		Use:   "exec <local file>",
		Short: "Run an iron command file containing multiple transfer statements",
		Long:  execDescription,
		Args:  cobra.ExactArgs(1),
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

	addTransferOptionFlags(cmd, &options)
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

	copyFlags := transfer.Flags{SyncModTime: true}
	uploadFlags := transfer.Flags{SyncModTime: true}
	downloadFlags := transfer.Flags{SyncModTime: true}

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

			paths.copy(ctx, worker, remoteCopy, copyFlags)

			return ctx.Err()
		},
	}
	copyCmd.Flags().BoolVar(&remoteCopy, "remote", false, "Copy to the target environment")
	addTransferFlags(copyCmd, &copyFlags)
	copyCmd.Flags().BoolVarP(&copyFlags.SkipTrash, "delete-skip-trash", "S", false, "Do not move deleted files to trash")
	copyCmd.Flags().BoolVarP(&copyFlags.CopyMetadata, "metadata", "m", false, "Copy AVU metadata on copy operations")
	copyCmd.Flags().BoolVar(&copyFlags.DisableUpdateInPlace, "no-update-in-place", false, "Replace existing objects instead of updating in place")
	root.AddCommand(copyCmd)

	uploadCmd := &cobra.Command{
		Use: "upload <local file> [target path]", Aliases: []string{uploadAlias}, Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			paths, err := a.uploadPaths(args)
			if err != nil {
				return err
			}

			paths.upload(ctx, worker, uploadFlags)

			return ctx.Err()
		},
	}
	addTransferFlags(uploadCmd, &uploadFlags)
	uploadCmd.Flags().BoolVarP(&uploadFlags.SkipTrash, "delete-skip-trash", "S", false, "Do not move deleted files to trash")
	uploadCmd.Flags().BoolVar(&uploadFlags.DisableUpdateInPlace, "no-update-in-place", false, "Replace existing objects instead of updating in place")
	root.AddCommand(uploadCmd)

	downloadCmd := &cobra.Command{
		Use: "download <path> [local file]", Aliases: []string{downloadAlias}, Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			paths, err := a.downloadPaths(ctx, args)
			if err != nil {
				return err
			}

			paths.download(ctx, worker, downloadFlags)

			return ctx.Err()
		},
	}
	addTransferFlags(downloadCmd, &downloadFlags)
	root.AddCommand(downloadCmd)

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
