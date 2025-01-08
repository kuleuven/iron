package cli

import (
	"context"
	"os"

	"gitea.icts.kuleuven.be/coz/iron"
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
	ctx     context.Context
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

	rootCmd.AddCommand(a.mkdir(), a.rmdir())

	return rootCmd
}

func (a *App) mkdir() *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:   "mkdir",
		Short: "Create a directory",
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
	var force, recursive bool

	cmd := &cobra.Command{
		Use:   "rmdir",
		Short: "Remove a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if recursive {
				return a.Client.DeleteCollectionAll(a.ctx, args[0], force)
			}

			return a.Client.DeleteCollection(a.ctx, args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Do not move to trash")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Remove files in collection recursively")

	return cmd
}

func (a *App) Close() error {
	if a.Client == nil {
		return nil
	}

	return a.Client.Close()
}
