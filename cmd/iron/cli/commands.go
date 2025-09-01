package cli

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/transfer"
	"github.com/spf13/cobra"
)

func (a *App) mkdir() *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:               "mkdir <target path>",
		Short:             "Create a collection",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if recursive {
				return a.CreateCollectionAll(a.Context, a.Path(args[0]))
			}

			return a.CreateCollection(a.Context, a.Path(args[0]))
		},
	}

	cmd.Flags().BoolVarP(&recursive, "parents", "p", false, "Create parents if necessary")

	return cmd
}

func (a *App) rmdir() *cobra.Command {
	var skip, recursive bool

	cmd := &cobra.Command{
		Use:               "rmdir <collection path>",
		Short:             "Remove a collection",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if recursive {
				return a.DeleteCollectionAll(a.Context, a.Path(args[0]), skip)
			}

			return a.DeleteCollection(a.Context, a.Path(args[0]), skip)
		},
	}

	cmd.Flags().BoolVarP(&skip, "skip-trash", "S", false, "Do not move to trash")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Remove files in collection recursively")

	return cmd
}

func (a *App) stat() *cobra.Command {
	var jsonFormat bool

	cmd := &cobra.Command{
		Use:               "stat <path>",
		Short:             "Get information about an object or collection",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := a.Path(args[0])

			record, err := a.GetRecord(a.Context, path, api.FetchMetadata, api.FetchAccess)
			if err != nil {
				return err
			}

			var printer Printer

			if jsonFormat {
				printer = &JSONPrinter{
					Writer: os.Stdout,
				}
			} else {
				printer = &TablePrinter{
					Writer: tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0),
					Zone:   a.Zone,
				}
			}

			defer printer.Flush()

			printer.Print(path, record)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonFormat, "json", "j", false, "Output in JSON format")

	return cmd
}

func (a *App) rm() *cobra.Command {
	var recursive, skip bool

	cmd := &cobra.Command{
		Use:               "rm <path>",
		Short:             "Remove a data object or collection",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := a.Path(args[0])

			obj, err := a.GetRecord(a.Context, path)
			if err != nil {
				return err
			}

			if obj.IsDir() {
				if recursive {
					return a.DeleteCollectionAll(a.Context, path, skip)
				}

				return a.DeleteCollection(a.Context, path, skip)
			}

			return a.DeleteDataObject(a.Context, path, skip)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Remove files in collection recursively")
	cmd.Flags().BoolVarP(&skip, "skip-trash", "S", false, "Do not move to trash")

	return cmd
}

func (a *App) mv() *cobra.Command {
	return &cobra.Command{
		Use:               "mv <path> <target path>",
		Short:             "Move a data object or a collection",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			src := a.Path(args[0])
			dest := args[1]

			if strings.HasSuffix(dest, "/") {
				dest += Name(src)
			}

			dest = a.Path(dest)

			obj, err := a.GetRecord(a.Context, src)
			if err != nil {
				return err
			}

			if obj.IsDir() {
				return a.RenameCollection(a.Context, src, dest)
			}

			return a.RenameDataObject(a.Context, src, dest)
		},
	}
}

func (a *App) cp() *cobra.Command {
	return &cobra.Command{
		Use:               "cp <object path> <target path>",
		Short:             "Copy a data object",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			src := a.Path(args[0])
			dest := args[1]

			if strings.HasSuffix(dest, "/") {
				dest += Name(src)
			}

			dest = a.Path(dest)

			return a.CopyDataObject(a.Context, src, dest)
		},
	}
}

func (a *App) create() *cobra.Command {
	return &cobra.Command{
		Use:               "create <target path>",
		Short:             "Create a data object without content",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := api.O_CREAT | api.O_WRONLY | api.O_EXCL

			h, err := a.CreateDataObject(a.Context, a.Path(args[0]), mode)
			if err != nil {
				return err
			}

			return h.Close()
		},
	}
}

func (a *App) touch() *cobra.Command {
	var t int64

	cmd := &cobra.Command{
		Use:               "touch <target path>",
		Short:             "Touch a data object",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := api.O_CREAT | api.O_RDWR

			h, err := a.OpenDataObject(a.Context, a.Path(args[0]), mode)
			if err != nil {
				return err
			}

			if err := h.Touch(time.Unix(t, 0)); err != nil {
				defer h.Close()

				return err
			}

			return h.Close()
		},
	}

	cmd.Flags().Int64Var(&t, "time", time.Now().Unix(), "Unix timestamp")

	return cmd
}

func (a *App) checksum() *cobra.Command {
	return &cobra.Command{
		Use:               "checksum <object path>",
		Short:             "Compute or get the checksum of a file",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			checksum, err := a.Checksum(a.Context, a.Path(args[0]), false)
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", hex.EncodeToString(checksum))

			return nil
		},
	}
}

func (a *App) upload() *cobra.Command {
	opts := transfer.Options{
		SyncModTime: true,
		MaxQueued:   10000,
		Output:      os.Stdout,
	}

	examples := []string{
		"iron upload /local/file.txt /path/to/collection/file.txt",
		"iron upload /local/file.txt /path/to/collection/",
		"iron upload /local/folder /path/to/collection",
		"iron upload /local/folder /path/to/collection/",
	}

	cmd := &cobra.Command{
		Use:               "upload <local file> [target path]",
		Aliases:           []string{"put"},
		Short:             "Upload a local file or directory to the destination path",
		Example:           strings.Join(examples, "\n"),
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				args = append(args, a.Workdir+"/")
			}

			if strings.HasSuffix(args[1], "/") {
				args[1] += Name(args[0])
			}

			target := a.Path(args[1])

			fi, err := os.Stat(args[0])
			if err != nil {
				return err
			}

			if !fi.IsDir() {
				opts.SyncModTime = false

				return a.Upload(a.Context, args[0], target, opts)
			}

			return a.UploadDir(a.Context, args[0], target, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Exclusive, "exclusive", false, "Do not overwrite existing files")
	cmd.Flags().IntVar(&opts.MaxThreads, "threads", 5, "Number of upload threads to use")
	cmd.Flags().BoolVar(&opts.VerifyChecksums, "checksum", false, "Verify checksums instead of size and modtime")

	return cmd
}

func (a *App) download() *cobra.Command {
	opts := transfer.Options{
		SyncModTime: true,
		MaxQueued:   10000,
		Output:      os.Stdout,
	}

	examples := []string{
		"iron download /path/to/collection/file.txt /local/file.txt",
		"iron download /path/to/collection/file.txt /local/folder/",
		"iron download /path/to/collection /local/folder",
		"iron download /path/to/collection /local/folder/",
	}

	cmd := &cobra.Command{
		Use:               "download <path> [local file]",
		Aliases:           []string{"get"},
		Short:             "Download a data object or a collection to the local path",
		Example:           strings.Join(examples, "\n"),
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				dir, err := os.Getwd()
				if err != nil {
					return err
				}

				args = append(args, dir+"/")
			}

			source := a.Path(args[0])
			target := args[1]

			if strings.HasSuffix(target, "/") {
				target += Name(source)
			}

			record, err := a.GetRecord(a.Context, source)
			if err != nil {
				return err
			}

			if !record.IsDir() {
				opts.SyncModTime = false

				return a.Download(a.Context, target, source, opts)
			}

			return a.DownloadDir(a.Context, target, source, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Exclusive, "exclusive", false, "Do not overwrite existing files")
	cmd.Flags().IntVar(&opts.MaxThreads, "threads", 5, "Number of download threads to use")
	cmd.Flags().BoolVar(&opts.VerifyChecksums, "checksum", false, "Verify checksums instead of size and modtime")

	return cmd
}

func (a *App) chmod() *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:   "chmod <permission> <user> <path>",
		Short: "Change permissions",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.ModifyAccess(a.Context, a.Path(args[2]), args[1], args[0], recursive)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Change permissions recursively")

	return cmd
}

func (a *App) inherit() *cobra.Command {
	var recursive, inherit bool

	cmd := &cobra.Command{
		Use:   "inherit <collection path>",
		Short: "Change permission inheritance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.SetCollectionInheritance(a.Context, a.Path(args[0]), inherit, recursive)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Change inheritance recursively")
	cmd.Flags().BoolVar(&inherit, "enable", true, "Enable inheritance")

	return cmd
}

func (a *App) list() *cobra.Command {
	var jsonFormat, listACL, listMeta bool

	cmd := &cobra.Command{
		Use:               "ls <collection path>",
		Aliases:           []string{"list"},
		Short:             "List a collection",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"."}
			}

			dir := a.Path(args[0])

			var printer Printer

			if jsonFormat {
				printer = &JSONPrinter{
					Writer: os.Stdout,
				}
			} else {
				printer = &TablePrinter{
					Writer: tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0),
					Zone:   a.Zone,
				}
			}

			defer printer.Flush()

			return a.Walk(a.Context, dir, func(path string, record api.Record, err error) error {
				if err != nil {
					return err
				}

				if path == dir && record.IsDir() {
					return api.SkipSubDirs
				}

				printer.Print(record.Name(), record)

				if record.IsDir() {
					return api.SkipDir
				}

				return nil
			}, walkOptions(listACL, listMeta)...)
		},
	}

	cmd.Flags().BoolVar(&jsonFormat, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&listACL, "acl", "a", false, "List ACLs")
	cmd.Flags().BoolVarP(&listMeta, "meta", "m", false, "List metadata")

	return cmd
}

func walkOptions(listACL, listMeta bool) []api.WalkOption {
	var opts []api.WalkOption

	if listACL {
		opts = append(opts, api.FetchAccess)
	}

	if listMeta {
		opts = append(opts, api.FetchMetadata)
	}

	return opts
}

func (a *App) tree() *cobra.Command {
	var maxDepth int

	cmd := &cobra.Command{
		Use:               "tree <collection path>",
		Short:             "Print the full tree structure beneath a collection",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"."}
			}

			dir := a.Path(args[0])

			opts := []api.WalkOption{api.LexographicalOrder}

			if maxDepth < 0 {
				opts = append(opts, api.NoSkip)
			}

			return a.Walk(a.Context, dir, func(path string, record api.Record, err error) error {
				depth := strings.Count(strings.TrimPrefix(path, dir), "/")

				fmt.Printf("%s%s\n", strings.Repeat("  ", depth), path)

				if err != nil || !record.IsDir() || maxDepth < 0 {
					return err
				}

				if depth == maxDepth-1 {
					return api.SkipSubDirs
				}

				if depth >= maxDepth {
					return api.SkipDir
				}

				return nil
			}, opts...)
		},
	}

	cmd.Flags().IntVarP(&maxDepth, "max-depth", "d", -1, "Max depth")

	return cmd
}

func (a *App) pwd() *cobra.Command {
	return &cobra.Command{
		Use:   "pwd",
		Short: "Print the current working directory",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(a.Workdir)
		},
	}
}

func (a *App) cd() *cobra.Command {
	return &cobra.Command{
		Use:               "cd <collection path>",
		Short:             "Change the current working directory",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"/" + a.Zone}
			}

			a.Workdir = a.Path(args[0])

			return nil
		},
	}
}

func (a *App) local() *cobra.Command {
	local := &cobra.Command{
		Use:   "local",
		Short: "Run a local command",
	}

	local.AddCommand(
		&cobra.Command{
			Use:   "pwd",
			Short: "Print the local working directory",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				dir, err := os.Getwd()
				if err != nil {
					return err
				}

				fmt.Println(dir)

				return nil
			},
		},
		&cobra.Command{
			Use:   "cd <local directory>",
			Short: "Change the local working directory",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					args = []string{os.Getenv("HOME")}
				}

				return os.Chdir(args[0])
			},
		},
	)

	return local
}
