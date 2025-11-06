package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/creativeprojects/go-selfupdate"
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

	name            string
	loadEnv         Loader
	configStore     ConfigStore
	configStoreArgs []string
	passwordStore   PasswordStore
	workdirStore    WorkdirStore

	releaseVersion string
	updater        *selfupdate.Updater
	repo           selfupdate.RepositorySlug

	Context context.Context //nolint:containedctx

	Admin   bool
	Debug   int
	Native  bool
	Workdir string
	PamTTL  time.Duration
}

func (a *App) Command() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               a.name,
		Short:             "Golang client for iRODS",
		PersistentPreRunE: a.Init,
	}

	rootCmd.PersistentFlags().CountVarP(&a.Debug, "debug", "v", "Enable debug output")
	rootCmd.PersistentFlags().BoolVar(&a.Admin, "admin", false, "Enable admin access")
	rootCmd.PersistentFlags().BoolVar(&a.Native, "native", false, "Use native protocol")
	rootCmd.PersistentFlags().StringVar(&a.Workdir, "workdir", a.Workdir, "Working directory")
	rootCmd.PersistentFlags().DurationVar(&a.PamTTL, "ttl", 168*time.Hour, "TTL in case pam authentication is used")

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
		a.meta(),
		a.checksum(),
		a.version(),
	)

	sh := shell.New(rootCmd, nil, prompt.OptionLivePrefix(a.prefix))
	run := sh.Run

	sh.Use = "shell [zone]"
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

	a.AddUpdateCommand(rootCmd)

	if a.passwordStore != nil {
		rootCmd.AddCommand(a.auth())
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
	if a.Debug > 0 {
		logrus.SetLevel(logrus.DebugLevel + logrus.Level(a.Debug-1))
	}

	if a.Client != nil || SkipInit(cmd) {
		return nil
	}

	a.CheckUpdate()

	var zone string

	// Get zone from arguments
	for i, argType := range a.ArgTypes(cmd) {
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

	return a.init(cmd, zone)
}

func (a *App) InitConfigStore(cmd *cobra.Command, args []string) error {
	zone, err := a.configStore(a.Context, args)
	if err != nil {
		return err
	}

	if a.Debug > 0 {
		logrus.SetLevel(logrus.DebugLevel + logrus.Level(a.Debug-1))
	}

	if a.Client != nil {
		return nil
	}

	return a.init(cmd, zone)
}

func (a *App) init(cmd *cobra.Command, zone string) error {
	// Load zone and start client
	ctx := a.Context

	if strings.HasPrefix(cmd.Use, "auth ") {
		ctx = context.WithValue(ctx, ForceReauthentication, true)
	}

	env, dialer, err := a.loadEnv(ctx, zone)
	if err != nil {
		return err
	}

	env.PamTTL = int(a.PamTTL.Hours())

	clientName := a.name

	// Telemetry: send version, except for prereleases
	if version := a.Version(); version.Prerelease() == "" && version.Metadata() == "" {
		clientName = fmt.Sprintf("%s-%s", clientName, version.String())
	}

	a.Client, err = iron.New(a.Context, env, iron.Option{
		ClientName:        clientName,
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
	if cmd.Use == "__complete [command-line]" || cmd.Use == "help [command]" || cmd.Use == "completion" || cmd.Use == "version" || cmd.Use == "update" {
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
