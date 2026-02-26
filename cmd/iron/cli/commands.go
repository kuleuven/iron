package cli

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/cmd/iron/tabwriter"
	"github.com/kuleuven/iron/transfer"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (a *App) version() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of iron",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(a.Version())
		},
	}
}

func (a *App) auth() *cobra.Command {
	use := "auth [flags] [zone]"
	args := cobra.MaximumNArgs(1)
	preRun := a.ResetInit

	if a.configStore != nil {
		use += "\n  " + a.name + " auth [flags] <" + strings.Join(a.configStoreArgs, "> <") + ">"

		args = func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 || len(args) == len(a.configStoreArgs) {
				return nil
			}

			return fmt.Errorf("accepts 0, 1 or %d arg(s), received %d", len(a.configStoreArgs), len(args))
		}

		preRun = func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return a.ResetInit(cmd, args)
			}

			return a.ResetInitConfigStore(cmd, args)
		}
	}

	cmd := &cobra.Command{
		Use:               use,
		Aliases:           []string{"authenticate", "iinit"},
		Short:             "Authenticate against the irods server.",
		Long:              "Authenticate against the irods server using the .irods/irods_environment.json file.",
		Args:              args,
		SilenceUsage:      true,
		PersistentPreRunE: preRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := a.Connect(cmd.Context())
			if err != nil {
				return err
			}

			password := conn.NativePassword()

			err = conn.Close()
			if err != nil {
				return err
			}

			return a.passwordStore(cmd.Context(), a.Client.Env(), password)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			// Reset the workdir if needed
			defaultDir := fmt.Sprintf("/%s", a.Client.Env().Zone)

			if a.Workdir != "/" && !strings.HasPrefix(a.Workdir, defaultDir+"/") {
				a.Workdir = defaultDir
			}
		},
	}

	cmd.Flags().BoolVar(&a.NonInteractive, "non-interactive", false, "Authenticate non-interactively, don't prompt any user input. Useful to fail gracefully in unattended scripts if the authentication method would unexpectedly request e.g. a password.")

	return cmd
}

func (a *App) mkdir() *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:               "mkdir <target path>",
		Short:             "Create a collection",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if recursive {
				return a.CreateCollectionAll(cmd.Context(), a.Path(args[0]))
			}

			return a.CreateCollection(cmd.Context(), a.Path(args[0]))
		},
	}

	cmd.Flags().BoolVarP(&recursive, "parents", "p", false, "Create parents if necessary")

	return cmd
}

func (a *App) rmdir() *cobra.Command {
	var skip bool

	cmd := &cobra.Command{
		Use:               "rmdir <collection path>",
		Short:             "Remove a empty collection",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.DeleteCollection(cmd.Context(), a.Path(args[0]), skip)
		},
	}

	cmd.Flags().BoolVarP(&skip, "skip-trash", "S", false, "Do not move to trash")

	return cmd
}

func (a *App) stat() *cobra.Command {
	var jsonFormat bool

	cmd := &cobra.Command{
		Use:               "stat <path>",
		Short:             "Get information about an object or collection",
		Long:              "Get information about an object or collection. For collections, the total size of all contained data objects is shown, but this count does not include any sub-collections.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := a.Path(args[0])

			record, err := a.GetRecord(cmd.Context(), path, api.FetchMetadata, api.FetchAccess, api.FetchCollectionSize)
			if err != nil {
				return err
			}

			var printer Printer = &TablePrinter{
				Writer: &tabwriter.TabWriter{
					Writer: cmd.OutOrStdout(),
				},
				Zone: a.Zone,
			}

			if jsonFormat {
				printer = &JSONPrinter{
					Writer: cmd.OutOrStdout(),
				}
			}

			printer.Setup(true, true, true)

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

			obj, err := a.GetRecord(cmd.Context(), path)
			if err != nil {
				return err
			}

			if obj.IsDir() {
				if !recursive {
					return a.DeleteCollection(cmd.Context(), path, skip)
				}

				opts := transfer.Options{
					MaxQueued:  10000,
					MaxThreads: 1,
					Output:     cmd.OutOrStdout(),
					SkipTrash:  skip,
				}

				return a.RemoveDir(cmd.Context(), path, opts)
			}

			return a.DeleteDataObject(cmd.Context(), path, skip)
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

			obj, err := a.GetRecord(cmd.Context(), src)
			if err != nil {
				return err
			}

			if obj.IsDir() {
				return a.RenameCollection(cmd.Context(), src, dest)
			}

			return a.RenameDataObject(cmd.Context(), src, dest)
		},
	}
}

const copyDescription = `Copy a file or directory to the target path.

When copying a collection, this command will compare the source and target, 
and only copy the missing parts. It can be repeated to keep the target up to date.
In this case, the target collection must end in a slash to avoid ambiguity.
If the source collection ends in a slash, files underneath will be placed directly
in the target collection. Otherwise, a subcollection with the same name will be created.`

func (a *App) cp() *cobra.Command {
	var (
		skip       bool
		maxThreads int
	)

	examples := []string{
		a.name + " cp /path/to/collection1/file.txt /path/to/collection2/file.txt  (target should not exist)",
		a.name + " cp /path/to/collection1/file.txt /path/to/collection2/          (target should not exist)",
		a.name + " cp /path/to/collection1 /path/to/collection2/                   (target may exist)",
		a.name + " cp /path/to/collection1/ /path/to/collection2/                  (target may exist)",
	}

	cmd := &cobra.Command{
		Use:               "cp <path> <target path>",
		Short:             "Copy a data object or a collection",
		Long:              copyDescription,
		Args:              cobra.ExactArgs(2),
		Example:           strings.Join(examples, "\n"),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			src := a.Path(args[0])
			dest := a.Path(args[1])

			if strings.HasSuffix(args[1], "/") && !strings.HasSuffix(args[0], "/") {
				dest = a.Path(args[1] + Name(src))
			}

			obj, err := a.GetRecord(cmd.Context(), src)
			if err != nil {
				return err
			}

			if obj.IsDir() {
				opts := transfer.Options{
					MaxQueued:  10000,
					MaxThreads: maxThreads,
					Output:     cmd.OutOrStdout(),
					SkipTrash:  skip,
				}

				if !strings.HasSuffix(args[1], "/") {
					return ErrAmbiguousTarget
				}

				return a.CopyDir(cmd.Context(), src, dest, opts)
			}

			return a.CopyDataObject(cmd.Context(), src, dest)
		},
	}

	cmd.Flags().BoolVarP(&skip, "delete-skip-trash", "S", false, "Do not move to trash (applies only when copying a collection)")
	cmd.Flags().IntVar(&maxThreads, "threads", 5, "Number of upload threads to use (applies only when copying a collection)")

	return cmd
}

func (a *App) create() *cobra.Command {
	return &cobra.Command{
		Use:               "create <target path>",
		Short:             "Create a data object without content",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := api.O_CREAT | api.O_WRONLY | api.O_EXCL

			h, err := a.CreateDataObject(cmd.Context(), a.Path(args[0]), mode)
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

			h, err := a.OpenDataObject(cmd.Context(), a.Path(args[0]), mode)
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
			checksum, err := a.Checksum(cmd.Context(), a.Path(args[0]), false)
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", hex.EncodeToString(checksum))

			return nil
		},
	}
}

func (a *App) checksums() *cobra.Command {
	var compute, verify bool

	cmd := &cobra.Command{
		Use:               "checksums <collection path>",
		Short:             "Compute all checksums of objects in a collection and all subcollections",
		Long:              `Compute the missing checksums of all data objects in a collection and all subcollections. This command will skip files that already have a checksum.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !compute && !verify {
				return errors.New("at least one of --compute or --verify must be set")
			}

			path := a.Path(args[0])

			opts := transfer.Options{
				MaxQueued:          10000,
				MaxThreads:         1,
				Output:             cmd.OutOrStdout(),
				IntegrityChecksums: compute,
				CompareChecksums:   verify,
			}

			return a.ComputeChecksums(cmd.Context(), path, opts)
		},
	}

	cmd.Flags().BoolVar(&compute, "compute", false, "Compute missing checksums")
	cmd.Flags().BoolVar(&verify, "verify", false, "Verify existing checksums")

	return cmd
}

var ErrAmbiguousTarget = errors.New("ambiguous command, please specify a target collection or directory with a trailing slash")

const uploadDescription = `Upload a file or directory to the target path.
This command will compare the source and target, and only upload the missing parts.
It can be repeated to keep the target up to date.

When uploading a directory, the target collection must end in a slash to avoid ambiguity.
If the source directory ends in a slash, files underneath will be placed directly
in the target collection. Otherwise, a subcollection with the same name will be created.`

func (a *App) upload() *cobra.Command { //nolint:funlen
	opts := transfer.Options{
		SyncModTime: true,
		MaxQueued:   10000,
	}

	examples := []string{
		a.name + " upload /local/file.txt",
		a.name + " upload /local/file.txt /path/to/collection/file.txt",
		a.name + " upload /local/file.txt /path/to/collection/",
		a.name + " upload /local/folder",
		a.name + " upload /local/folder /path/to/collection/",
		a.name + " upload /local/folder/ /path/to/collection/",
	}

	cmd := &cobra.Command{
		Use:               "upload <local file> [target path]",
		Aliases:           []string{"put"},
		Short:             "Upload a local file or directory to the destination path",
		Long:              uploadDescription,
		Example:           strings.Join(examples, "\n"),
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				args = append(args, a.Workdir+"/")
			}

			source := filepath.Clean(args[0])
			target := a.Path(args[1])

			if strings.HasSuffix(args[1], "/") && !localPathEndsWithSeparator(args[0]) {
				target = a.Path(args[1] + filepath.Base(source))
			}

			fi, err := os.Stat(source)
			if err != nil {
				return err
			}

			opts.Output = cmd.OutOrStdout()

			if !fi.IsDir() {
				opts.SyncModTime = false

				return a.Upload(cmd.Context(), source, target, opts)
			}

			if !strings.HasSuffix(args[1], "/") {
				return ErrAmbiguousTarget
			}

			return a.UploadDir(cmd.Context(), source, target, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Exclusive, "exclusive", false, "Do not overwrite existing files")
	cmd.Flags().BoolVar(&opts.Delete, "delete", false, "Delete files in the destination that no longer exist in the source")
	cmd.Flags().BoolVarP(&opts.SkipTrash, "delete-skip-trash", "S", false, "Do not move to trash when deleting")
	cmd.Flags().BoolVar(&opts.DisableUpdateInPlace, "no-update-in-place", false, "Do not update objects in place, delete old versions first")
	cmd.Flags().IntVar(&opts.MaxThreads, "threads", 5, "Number of upload threads to use")
	cmd.Flags().BoolVar(&opts.CompareChecksums, "checksum", false, "Compare checksums instead of size and modtime to select files to upload")
	cmd.Flags().BoolVar(&opts.IntegrityChecksums, "verify-checksum", false, "Compute checksums before and after uploading files, and verify equality to ensure transfer integrity")

	return cmd
}

const downloadDescription = `Download a data object or a collection to the local path.
This command will compare the source and target, and only download the missing parts.
It can be repeated to keep the target up to date.

When downloading a collection, the target folder must end in a slash to avoid ambiguity.
If the source collection ends in a slash, files underneath will be placed directly
in the target folder. Otherwise, a subfolder with the same name will be created.`

func (a *App) download() *cobra.Command { //nolint:funlen
	opts := transfer.Options{
		SyncModTime: true,
		MaxQueued:   10000,
	}

	examples := []string{
		a.name + " download /path/to/collection/file.txt",
		a.name + " download /path/to/collection/file.txt /local/file.txt",
		a.name + " download /path/to/collection/file.txt /local/folder/",
		a.name + " download /path/to/collection",
		a.name + " download /path/to/collection /local/folder/",
		a.name + " download /path/to/collection/ /local/folder/",
	}

	cmd := &cobra.Command{
		Use:               "download <path> [local file]",
		Aliases:           []string{"get"},
		Short:             "Download a data object or a collection to the local path",
		Long:              downloadDescription,
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
			target := filepath.Clean(args[1])

			if localPathEndsWithSeparator(args[1]) && !strings.HasSuffix(args[0], "/") {
				target = filepath.Join(target, Name(source))
			}

			record, err := a.GetRecord(cmd.Context(), source)
			if err != nil {
				return err
			}

			opts.Output = cmd.OutOrStdout()

			if !record.IsDir() {
				opts.SyncModTime = false

				return a.Download(cmd.Context(), target, source, opts)
			}

			if !localPathEndsWithSeparator(args[1]) {
				return ErrAmbiguousTarget
			}

			return a.DownloadDir(cmd.Context(), target, source, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Exclusive, "exclusive", false, "Do not overwrite existing files")
	cmd.Flags().BoolVar(&opts.Delete, "delete", false, "Delete files in the destination that no longer exist in the source")
	cmd.Flags().IntVar(&opts.MaxThreads, "threads", 5, "Number of download threads to use")
	cmd.Flags().BoolVar(&opts.CompareChecksums, "checksum", false, "Verify checksums instead of size and modtime")

	return cmd
}

func localPathEndsWithSeparator(path string) bool {
	return strings.HasSuffix(path, "/") || strings.HasSuffix(path, string(os.PathSeparator))
}

func (a *App) cat() *cobra.Command {
	var maxThreads int

	cmd := &cobra.Command{
		Use:               "cat <object path>",
		Short:             "Stream a data object to stdout",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			source := a.Path(args[0])

			var output io.Writer

			if !term.IsTerminal(int(os.Stdout.Fd())) {
				output = cmd.ErrOrStderr()
			}

			return a.Client.ToWriter(cmd.Context(), cmd.OutOrStdout(), source, transfer.Options{
				MaxThreads: maxThreads,
				Output:     output,
			})
		},
	}

	cmd.Flags().IntVar(&maxThreads, "threads", 5, "Number of download threads to use")

	return cmd
}

func (a *App) head() *cobra.Command {
	var n int

	cmd := &cobra.Command{
		Use:               "head <object path>",
		Short:             "Print the first lines of a data object to stdout",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			source := a.Path(args[0])

			f, err := a.OpenDataObject(cmd.Context(), source, api.O_RDONLY)
			if err != nil {
				return err
			}

			defer f.Close()

			r := bufio.NewReader(f)

			for range n {
				payload, err := r.ReadBytes('\n')

				_, err2 := cmd.OutOrStdout().Write(payload)

				if errors.Is(err, io.EOF) {
					break
				} else if err != nil {
					return err
				}

				if err2 != nil {
					return err2
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&n, "lines", 10, "Number of lines to print")

	return cmd
}

var saveDescription = `Stream the standard input to a data object.
If the data object does not exist, it will be created.
Use '--append' to append to an existing data object.`

func (a *App) save() *cobra.Command {
	var (
		appendFlag bool
		maxThreads int
	)

	eofMessage := "Press Ctrl+D to end input"

	if runtime.GOOS == "windows" {
		eofMessage = "Press Ctrl+Z followed by Enter to end input"
	}

	examples := []string{
		"  Read from standard input (" + eofMessage + "):\n\t" + a.name + " save object.txt",
		"  Pipe from other command (does not work inside `" + a.name + " shell`):\n\techo Test | " + a.name + " save --append object.txt",
		"  Write both to irods and standard output:\n\techo Test | " + a.name + " tee object.txt",
	}

	cmd := &cobra.Command{
		Use:               "save <object path>",
		Aliases:           []string{"tee"},
		Short:             "Stream the standard input to a data object.",
		Long:              saveDescription,
		Example:           strings.Join(examples, "\n"),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := a.Path(args[0])

			if a.inShell {
				fmt.Printf("[%s]\n", eofMessage)
			}

			var output io.Writer

			if !term.IsTerminal(int(os.Stdin.Fd())) {
				output = cmd.ErrOrStderr()
			}

			input := cmd.InOrStdin()

			if cmd.CalledAs() == "tee" {
				input = io.TeeReader(input, cmd.OutOrStdout())
			}

			return a.Client.FromReader(cmd.Context(), input, target, appendFlag, transfer.Options{
				MaxThreads: maxThreads,
				Output:     output,
			})
		},
	}

	cmd.Flags().BoolVarP(&appendFlag, "append", "a", false, "Append to existing data object")
	cmd.Flags().IntVar(&maxThreads, "threads", 5, "Number of upload threads to use")

	return cmd
}

var chmodDescription = `Modify access to data objects and collections.

By default, the original creator of a data object has 'own' permission.

The iRODS Permission Model is linear with 10 levels.
Access levels can be specified both to data objects and collections.
A granted access level also includes access to all lower levels.

    'own'
    'delete_object'
    'modify_object' (= 'write')
    'create_object'
    'delete_metadata'
    'modify_metadata'
    'create_metadata'
    'read_object'   (= 'read')
    'read_metadata'
    'null'

The iRODS Permission Model allows for multiple ownership.
Granting 'own' to another user or group will allow them to grant
permission to and revoke permission from others (including you).

Setting the access level to 'null' will remove access for that user or group.

Example Operations requiring permissions:

    irm - requires 'delete_object' or greater
    imv - requires 'delete_object' or greater, due to removal of old name
    iput - requires 'modify_object' or greater
    iget - requires 'read_object' or greater`

func (a *App) chmod() *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:   "chmod <access level> <user or group> <path>",
		Short: "Change permissions",
		Long:  chmodDescription,
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.ModifyAccess(cmd.Context(), a.Path(args[2]), args[1], args[0], recursive)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Change permissions recursively")

	return cmd
}

const inheritDescription = `Change permission inheritance for a collection.

The inherit/noinherit form sets or clears the inheritance attribute of
a collection. When collections have this attribute set,
new data objects and subcollections added to the collection inherit the
access permissions (ACLs) of the collection. 

The inherit status is shown by the + symbol when listing collections.`

func (a *App) inherit() *cobra.Command {
	var recursive, inherit bool

	examples := []string{
		"  Enable inheritance for a collection:\n\t" + a.name + " inherit collection",
		"  Disable inheritance for a collection:\n\t" + a.name + " inherit --enable=false collection",
	}

	cmd := &cobra.Command{
		Use:     "inherit <collection path>",
		Short:   "Change permission inheritance",
		Long:    inheritDescription,
		Example: strings.Join(examples, "\n"),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.SetCollectionInheritance(cmd.Context(), a.Path(args[0]), inherit, recursive)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Change inheritance recursively")
	cmd.Flags().BoolVar(&inherit, "enable", true, "Enable inheritance")

	return cmd
}

var listDescription = `List the contents of a collection or information about a data object.

The column STATUS shows for collections whether ACL inheritance is enabled (+).
For data objects it indicates the status of the replicas, as follows:

	✔	Good up-to-date replica
	✘	Stale replica
	⚿	Replica is write-locked, i.e. a process is currently writing to it
		or an earlier process did not finish properly.
	…	Replica is in intermediate state, another replica is write-locked.`

func (a *App) list() *cobra.Command {
	var (
		jsonFormat, listACL, listMeta, collectionSizes bool
		columns                                        []string
	)

	cmd := &cobra.Command{
		Use:               "ls <collection path>",
		Aliases:           []string{"list"},
		Short:             "List the contents of a collection or information about a data object",
		Long:              listDescription,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"."}
			}

			dir := a.Path(args[0])

			hideColumns, err := hiddenColumns(columns, "creator", "size", "date", "status", "name")
			if err != nil {
				return err
			}

			var printer Printer = &TablePrinter{
				Writer: &tabwriter.TabWriter{
					Writer:      cmd.OutOrStdout(),
					HideColumns: hideColumns,
				},
				Zone: a.Zone,
			}

			if jsonFormat {
				printer = &JSONPrinter{
					Writer: cmd.OutOrStdout(),
				}
			}

			printer.Setup(listACL, listMeta, collectionSizes)

			defer printer.Flush()

			return a.Walk(cmd.Context(), dir, listFunc(dir, printer), walkOptions(listACL, listMeta, collectionSizes)...)
		},
	}

	cmd.Flags().BoolVar(&jsonFormat, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&listACL, "acl", "a", false, "List ACLs")
	cmd.Flags().BoolVarP(&listMeta, "meta", "m", false, "List metadata")
	cmd.Flags().BoolVarP(&collectionSizes, "sizes", "s", false, "Show the total size of objects in a collection (this does not include sub-collections).")
	cmd.Flags().StringSliceVar(&columns, "columns", []string{"creator", "size", "date", "status", "name"}, "Columns to display. Available options: creator, size, date, status, name, all.")

	return cmd
}

func listFunc(dir string, printer Printer) func(path string, record api.Record, err error) error {
	return func(path string, record api.Record, err error) error {
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
	}
}

func walkOptions(listACL, listMeta, collectionSizes bool) []api.WalkOption {
	var opts []api.WalkOption

	if listACL {
		opts = append(opts, api.FetchAccess)
	}

	if listMeta {
		opts = append(opts, api.FetchMetadata)
	}

	if collectionSizes {
		opts = append(opts, api.FetchCollectionSize)
	}

	return opts
}

func hiddenColumns(columns []string, available ...string) ([]int, error) {
	if slices.Contains(columns, "all") {
		if len(columns) > 1 {
			return nil, fmt.Errorf("cannot use 'all' with other columns")
		}

		return nil, nil
	}

	for _, col := range columns {
		if !slices.Contains(available, col) {
			return nil, fmt.Errorf("unknown column: %s", col)
		}
	}

	var hidden []int

	for i, col := range available {
		if !slices.Contains(columns, col) {
			hidden = append(hidden, i)
		}
	}

	return hidden, nil
}

func (a *App) tree() *cobra.Command { //nolint:funlen
	var (
		jsonFormat      bool
		maxDepth        int
		columns         []string
		collectionSizes bool
	)

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

			hideColumns, err := hiddenColumns(columns, "creator", "size", "date", "status", "name")
			if err != nil {
				return err
			}

			var printer Printer = &TablePrinter{
				Writer: &tabwriter.StreamWriter{
					Writer:       cmd.OutOrStdout(),
					ColumnWidths: []int{20, 8, 13, 6},
					HideColumns:  hideColumns,
				},
				Zone: a.Zone,
			}

			if jsonFormat {
				printer = &JSONPrinter{
					Writer: cmd.OutOrStdout(),
				}
			}

			printer.Setup(false, false, collectionSizes)

			defer printer.Flush()

			opts := []api.WalkOption{api.LexographicalOrder}

			if maxDepth < 0 {
				opts = append(opts, api.NoSkip)
			}

			if collectionSizes {
				opts = append(opts, api.FetchCollectionSize)
			}

			return a.Walk(cmd.Context(), dir, treeFunc(dir, printer, maxDepth, jsonFormat), opts...)
		},
	}

	cmd.Flags().IntVarP(&maxDepth, "max-depth", "d", -1, "Max depth")
	cmd.Flags().BoolVar(&jsonFormat, "json", false, "Output as JSON")
	cmd.Flags().StringSliceVar(&columns, "columns", []string{"name"}, "Columns to display. Available options: creator, size, date, status, name, all.")
	cmd.Flags().BoolVarP(&collectionSizes, "sizes", "s", false, "Show the total size of objects in a collection (this does not include sub-collections).")

	return cmd
}

func treeFunc(dir string, printer Printer, maxDepth int, jsonFormat bool) func(path string, record api.Record, err error) error {
	return func(path string, record api.Record, err error) error {
		if err != nil {
			return err
		}

		depth := strings.Count(strings.TrimPrefix(path, dir), "/")

		printer.Print(indentString(path, depth, jsonFormat), record)

		if !record.IsDir() || maxDepth < 0 || depth < maxDepth-1 {
			return nil
		}

		if depth == maxDepth-1 {
			return api.SkipSubDirs
		}

		return api.SkipDir
	}
}

func indentString(s string, depth int, jsonFormat bool) string {
	if jsonFormat {
		return s
	}

	return fmt.Sprintf("%s%s", strings.Repeat("  ", depth), s)
}

func (a *App) meta() *cobra.Command {
	meta := &cobra.Command{
		Use:   "meta",
		Short: "Run a metadata command",
	}

	meta.AddCommand(
		a.metals(),
		a.metaop("add", "Add a single metadata triplet", func(client *api.API) func(context.Context, string, api.ObjectType, api.Metadata) error {
			return client.AddMetadata
		}),
		a.metaop("rm", "Delete a single metadata triplet", func(client *api.API) func(context.Context, string, api.ObjectType, api.Metadata) error {
			return client.RemoveMetadata
		}),
		a.metaop("set", "Set a metadata triplet and remove old metadata with the same key", func(client *api.API) func(context.Context, string, api.ObjectType, api.Metadata) error {
			return client.SetMetadata
		}),
		a.metaunset(),
	)

	return meta
}

func (a *App) metals() *cobra.Command {
	return &cobra.Command{
		Use:               "ls <path>",
		Short:             "List metadata",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := a.Path(args[0])

			stat, err := a.GetRecord(cmd.Context(), path, api.FetchMetadata)
			if err != nil {
				return err
			}

			out := &tabwriter.TabWriter{
				Writer: cmd.OutOrStdout(),
			}

			defer out.Flush()

			fmt.Fprintf(out, "%sKEY\tVALUE\tUNITS%s\n", Bold, Reset)

			for _, m := range stat.Metadata() {
				fmt.Fprintf(out, "%s\t%s\t%s\n", m.Name, m.Value, m.Units)
			}

			return nil
		},
	}
}

func (a *App) metaop(op, description string, fn func(*api.API) func(context.Context, string, api.ObjectType, api.Metadata) error) *cobra.Command {
	return &cobra.Command{
		Use:               op + " <path> <key> <value> [units]",
		Short:             description,
		Args:              cobra.RangeArgs(3, 4),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 4 {
				args = append(args, "")
			}

			path := a.Path(args[0])

			stat, err := a.GetRecord(cmd.Context(), path)
			if err != nil {
				return err
			}

			return fn(a.Client.API)(cmd.Context(), path, stat.Type(), api.Metadata{
				Name: args[1], Value: args[2], Units: args[3],
			})
		},
	}
}

func (a *App) metaunset() *cobra.Command {
	return &cobra.Command{
		Use:               "unset <path> <key>",
		Short:             "Remove all metadata triplets with the given key",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: a.CompleteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := a.Path(args[0])

			stat, err := a.GetRecord(cmd.Context(), path, api.FetchMetadata)
			if err != nil {
				return err
			}

			toremove := []api.Metadata{}

			for _, triplet := range stat.Metadata() {
				if triplet.Name == args[1] {
					toremove = append(toremove, triplet)
				}
			}

			return a.Client.ModifyMetadata(cmd.Context(), path, stat.Type(), nil, toremove)
		},
	}
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

			target := a.Path(args[0])

			// Check if the target is a collection
			if _, err := a.GetCollection(cmd.Context(), target); err != nil {
				return err
			}

			a.Workdir = target

			if a.workdirStore != nil {
				return a.workdirStore(cmd.Context(), a.Workdir)
			}

			return nil
		},
	}
}

func (a *App) local() *cobra.Command { //nolint:funlen
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
			Use:               "cd <local directory>",
			Short:             "Change the local working directory",
			Args:              cobra.MaximumNArgs(1),
			ValidArgsFunction: a.CompleteArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					args = []string{os.Getenv("HOME")}
				}

				return os.Chdir(args[0])
			},
		},
		&cobra.Command{
			Use:               "ls [local directory]",
			Short:             "List the local working directory",
			Args:              cobra.MaximumNArgs(1),
			ValidArgsFunction: a.CompleteArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					args = []string{"."}
				}

				entries, err := os.ReadDir(args[0])
				if err != nil {
					return err
				}

				slices.SortFunc(entries, func(a, b os.DirEntry) int {
					return strings.Compare(a.Name(), b.Name())
				})

				for _, entry := range entries {
					name := entry.Name()
					color := NoColor

					if entry.IsDir() {
						name += "/"
						color = Blue
					}

					Fprintcolorln(cmd.OutOrStdout(), color, name)
				}

				return nil
			},
		},
	)

	return local
}

// The main purpose of this command is to test context cancellation
func (a *App) sleep() *cobra.Command {
	return &cobra.Command{
		Use:    "sleep <seconds>",
		Short:  "Sleep for the given number of seconds",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return err
			}

			select {
			case <-cmd.Context().Done():
				return cmd.Context().Err()
			case <-time.After(time.Duration(n * float64(time.Second))):
				return nil
			}
		},
	}
}

func (a *App) query() *cobra.Command {
	examples := "  Print available column names:\n\t" + a.name + " query\n  Run a query:\n\t" + a.name + " query \"select DATA_NAME, DATA_SIZE\""

	cmd := &cobra.Command{
		Use:     "query [sql]",
		Short:   "Run a generic query",
		Example: examples,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				columns, err := a.GenericQueryColumns(cmd.Context())
				if err != nil {
					return err
				}

				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(columns, "\n"))

				return nil
			}

			results := a.GenericQuery(args[0]).Execute(cmd.Context())

			defer results.Close()

			if results.Err() != nil {
				return results.Err()
			}

			columns := guessColumns(args[0])

			out := &tabwriter.TabWriter{
				Writer: cmd.OutOrStdout(),
			}

			defer out.Flush()

			Fprintcolorln(out, Bold, strings.Join(columns, "\t"))

			ptrs := make([]any, len(columns))

			for i := range ptrs {
				ptrs[i] = new(string)
			}

			formatting := strings.Repeat("%s\t", len(columns)-1) + "%s\n"

			for results.Next() {
				if err := results.Scan(ptrs...); err != nil {
					return err
				}

				fmt.Fprintf(out, formatting, resolveValues(ptrs)...)
			}

			return results.Err()
		},
	}

	return cmd
}

func guessColumns(query string) []string {
	var columns []string

	for field := range strings.FieldsSeq(query) {
		switch strings.ToLower(field) {
		case "select":
		// Ignore
		case "where", "group", "order", "limit", "offset":
			return columns
		default:
			for col := range strings.SplitSeq(field, ",") {
				if col == "" {
					continue
				}

				columns = append(columns, col)
			}
		}
	}

	return columns
}

func resolveValues(ptrs []any) []any {
	values := make([]any, len(ptrs))

	for i, ptr := range ptrs {
		if casted, ok := ptr.(*string); ok {
			values[i] = *casted
		}
	}

	return values
}

func (a *App) ps() *cobra.Command {
	return &cobra.Command{
		Use:    "ps",
		Short:  "List processes",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			result := a.Procs(cmd.Context())

			defer result.Close()

			columns := []string{
				"PROCESS ID",
				"START TIME",
				"PROXY NAME",
				"PROXY ZONE",
				"CLIENT NAME",
				"CLIENT ZONE",
				"REMOTE ADDR",
				"SERVER ADDR",
				"PROGRAM",
			}

			out := &tabwriter.TabWriter{
				Writer: cmd.OutOrStdout(),
			}

			defer out.Flush()

			Fprintcolorln(out, Bold, strings.Join(columns, "\t"))

			values := make([]string, len(columns))
			ptrs := make([]any, len(values))

			for i := range values {
				ptrs[i] = &values[i]
			}

			for result.Next() {
				if err := result.Scan(ptrs...); err != nil {
					return err
				}

				fmt.Fprintf(out, "%s\n", strings.Join(values, "\t"))
			}

			return result.Err()
		},
	}
}

func Fprintcolorln(w io.Writer, color string, args ...any) {
	fmt.Fprintf(w, "%s%s%s\n", color, fmt.Sprint(args...), Reset)
}
