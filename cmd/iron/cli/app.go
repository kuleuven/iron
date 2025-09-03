package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/c-bata/go-prompt"
	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/cmd/iron/shell"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func New(ctx context.Context, options ...Option) *App {
	home := os.Getenv("HOME")

	if home == "" {
		home = "."
	}

	app := &App{
		Context: ctx,
		name:    "iron",
		loadEnv: FileLoader(home + "/.irods/irods_environment.json"),
	}

	for _, option := range options {
		option(app)
	}

	return app
}

type App struct {
	*iron.Client

	name          string
	loadEnv       Loader
	passwordStore PasswordStore
	workdirStore  WorkdirStore

	Context context.Context //nolint:containedctx

	Admin   bool
	Debug   int
	Native  bool
	Workdir string
}

func (a *App) Command() *cobra.Command { //nolint:funlen
	rootCmd := &cobra.Command{
		Use:   a.name,
		Short: "Golang client for iRODS",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if a.Debug > 0 {
				logrus.SetLevel(logrus.DebugLevel + logrus.Level(a.Debug-1))
			}

			if a.Client != nil || SkipInit(cmd) {
				return nil
			}

			return a.Init(cmd, args)
		},
	}

	rootCmd.PersistentFlags().CountVarP(&a.Debug, "debug", "v", "Enable debug output")
	rootCmd.PersistentFlags().BoolVar(&a.Admin, "admin", false, "Enable admin access")
	rootCmd.PersistentFlags().BoolVar(&a.Native, "native", false, "Use native protocol")
	rootCmd.PersistentFlags().StringVar(&a.Workdir, "workdir", a.Workdir, "Working directory")

	rootCmd.AddCommand(
		a.mkdir(),
		a.rmdir(),
		a.rm(),
		a.mv(),
		a.cp(),
		a.create(),
		a.touch(),
		a.upload(),
		a.download(),
		a.chmod(),
		a.inherit(),
		a.list(),
		a.tree(),
		a.stat(),
		a.checksum(),
	)

	sh := shell.New(rootCmd, nil, prompt.OptionLivePrefix(a.prefix))
	run := sh.Run

	sh.Use = "shell <zone>"
	sh.Args = cobra.MaximumNArgs(1)
	sh.Run = func(cmd *cobra.Command, args []string) {
		rootCmd.ResetFlags()

		if a.workdirStore == nil {
			rootCmd.AddCommand(a.pwd(), a.cd())
		}

		rootCmd.AddCommand(a.local())

		run(cmd, args)
	}

	rootCmd.AddCommand(sh)

	if a.passwordStore != nil {
		rootCmd.AddCommand(a.authenticate())
	}

	if a.workdirStore != nil {
		rootCmd.AddCommand(a.pwd(), a.cd())
	}

	return rootCmd
}

func (a *App) prefix() (string, bool) {
	return fmt.Sprintf("%s > %s > ", a.name, a.Workdir), true
}

func (a *App) Init(cmd *cobra.Command, args []string) error {
	var zone string

	// Get zone from arguments
	for i, argType := range ArgTypes(cmd) {
		if i >= len(args) {
			continue
		}

		if z := GetZone(args[i], argType); zone == "" || z != "" && zone == z {
			zone = z
		} else if z != "" {
			return errors.New("multiple zones found in arguments")
		}
	}

	if z := GetZone(a.Workdir, CollectionPath); zone == "" || z != "" && zone == z {
		zone = z
	} else if z != "" {
		return errors.New("multiple zones found in arguments")
	}

	// Load zone and start client
	env, dialer, err := a.loadEnv(a.Context, zone)
	if err != nil {
		return err
	}

	// If authenticating, erase password
	if cmd.Use == "authenticate <zone>" {
		env.Password = ""
	}

	a.Client, err = iron.New(a.Context, env, iron.Option{
		ClientName:        a.name,
		Admin:             a.Admin,
		UseNativeProtocol: a.Native,
		MaxConns:          16,
		DialFunc:          dialer,
	})

	if a.Workdir == "" {
		a.Workdir = fmt.Sprintf("/%s", env.Zone)
	}

	return err
}

func SkipInit(cmd *cobra.Command) bool {
	if cmd.Use == "__complete [command-line]" || cmd.Use == "help [command]" || cmd.Use == "completion" {
		return true
	}

	if parent := cmd.Parent(); parent != nil && SkipInit(parent) {
		return true
	}

	return false
}

func (a *App) Close() error {
	if a.Client == nil {
		return nil
	}

	return a.Client.Close()
}
