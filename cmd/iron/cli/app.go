package cli

import (
	"context"
	"io"
	"os"

	"gitea.icts.kuleuven.be/coz/iron"
	"gitea.icts.kuleuven.be/coz/iron/api"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func New(ctx context.Context) *App {
	home := os.Getenv("HOME")

	if home == "" {
		home = "."
	}

	return &App{
		ctx:     ctx,
		envfile: home + "/.irods/irods_environment.json",
	}
}

type App struct {
	*iron.Client
	ctx     context.Context //nolint:containedctx
	envfile string
	admin   bool
	debug   int
}

func (a *App) Command() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "iron",
		Short: "Golang client for iRODS",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if a.debug > 0 {
				logrus.SetLevel(logrus.DebugLevel + logrus.Level(a.debug-1))
			}

			var env iron.Env

			err := env.LoadFromFile(a.envfile)
			if err != nil {
				return err
			}

			env.ApplyDefaults()

			a.Client, err = iron.New(a.ctx, env, iron.Option{
				ClientName: "iron",
				Admin:      a.admin,
			})

			return err
		},
	}

	rootCmd.PersistentFlags().CountVarP(&a.debug, "debug", "v", "Enable debug output")
	rootCmd.PersistentFlags().BoolVarP(&a.admin, "admin", "a", false, "Enable admin access")

	rootCmd.AddCommand(a.mkdir(), a.rmdir(), a.mvdir(), a.rm(), a.mv(), a.cp(), a.create(), a.put(), a.get(), a.chmod(), a.inherit())

	return rootCmd
}

func (a *App) mkdir() *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:   "mkdir <dir>",
		Short: "Create a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if recursive {
				return a.Client.CreateCollectionAll(a.ctx, args[0])
			}

			return a.Client.CreateCollection(a.ctx, args[0])
		},
	}

	cmd.Flags().BoolVarP(&recursive, "parents", "p", false, "Create parents if necessary")

	return cmd
}

func (a *App) rmdir() *cobra.Command {
	var skip, recursive bool

	cmd := &cobra.Command{
		Use:   "rmdir <dir>",
		Short: "Remove a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if recursive {
				return a.Client.DeleteCollectionAll(a.ctx, args[0], skip)
			}

			return a.Client.DeleteCollection(a.ctx, args[0], skip)
		},
	}

	cmd.Flags().BoolVarP(&skip, "skip-trash", "S", false, "Do not move to trash")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Remove files in collection recursively")

	return cmd
}

func (a *App) mvdir() *cobra.Command {
	return &cobra.Command{
		Use:   "mvdir <from> <to>",
		Short: "Move a collection",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Client.RenameCollection(a.ctx, args[0], args[1])
		},
	}
}

func (a *App) rm() *cobra.Command {
	var skip bool

	cmd := &cobra.Command{
		Use:   "rm <path>",
		Short: "Remove a data object",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Client.DeleteDataObject(a.ctx, args[0], skip)
		},
	}

	cmd.Flags().BoolVarP(&skip, "skip-trash", "S", false, "Do not move to trash")

	return cmd
}

func (a *App) mv() *cobra.Command {
	return &cobra.Command{
		Use:   "mv <from> <to>",
		Short: "Move a data object",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Client.RenameDataObject(a.ctx, args[0], args[1])
		},
	}
}

func (a *App) cp() *cobra.Command {
	return &cobra.Command{
		Use:   "cp <from> <to>",
		Short: "Copy a data object",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Client.CopyDataObject(a.ctx, args[0], args[1])
		},
	}
}

func (a *App) create() *cobra.Command {
	return &cobra.Command{
		Use:   "create <path>",
		Short: "Create a data object",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := api.O_CREAT | api.O_WRONLY | api.O_EXCL

			h, err := a.CreateDataObject(a.ctx, args[0], mode)
			if err != nil {
				return err
			}

			return h.Close()
		},
	}
}

func (a *App) put() *cobra.Command {
	var exclusive bool

	cmd := &cobra.Command{
		Use:   "put <local> <remote>",
		Short: "Upload a file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := api.O_CREAT | api.O_WRONLY | api.O_TRUNC

			if exclusive {
				mode |= api.O_EXCL
			}

			r, err := os.Open(args[0])
			if err != nil {
				return err
			}

			defer r.Close()

			w, err := a.OpenDataObject(a.ctx, args[1], mode)
			if err != nil {
				return err
			}

			defer w.Close()

			buffer := make([]byte, 32*1024*1024)

			_, err = io.CopyBuffer(w, r, buffer)

			return err
		},
	}

	cmd.Flags().BoolVar(&exclusive, "exclusive", false, "Do not overwrite existing files")

	return cmd
}

func (a *App) get() *cobra.Command {
	var exclusive bool

	cmd := &cobra.Command{
		Use:   "get <remote> <local>",
		Short: "Download a file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC

			if exclusive {
				mode |= os.O_EXCL
			}

			w, err := os.OpenFile(args[1], mode, 0o600)
			if err != nil {
				return err
			}

			defer w.Close()

			r, err := a.OpenDataObject(a.ctx, args[1], api.O_RDONLY)
			if err != nil {
				return err
			}

			defer r.Close()

			buffer := make([]byte, 32*1024*1024)

			_, err = io.CopyBuffer(w, r, buffer)

			return err
		},
	}

	cmd.Flags().BoolVar(&exclusive, "exclusive", false, "Do not overwrite existing files")

	return cmd
}

func (a *App) chmod() *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:   "chmod <permission> <user> <path>",
		Short: "Change permissions",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.ModifyAccess(a.ctx, args[2], args[1], args[0], recursive)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Change permissions recursively")

	return cmd
}

func (a *App) inherit() *cobra.Command {
	var recursive, inherit bool

	cmd := &cobra.Command{
		Use:   "inherit <path>",
		Short: "Change permission inheritance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.SetCollectionInheritance(a.ctx, args[0], inherit, recursive)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Change inheritance recursively")
	cmd.Flags().BoolVar(&inherit, "enable", true, "Enable inheritance")

	return cmd
}

func (a *App) Close() error {
	if a.Client == nil {
		return nil
	}

	return a.Client.Close()
}
