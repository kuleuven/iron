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

	inShell bool
}

func (a *App) Command() *cobra.Command {
	// Root command
	rootCmd := a.root(false)

	// Root to be used in shell
	rootShell := a.root(true)
	hiddenChild := a.root(true)
	hiddenChild.Hidden = true
	rootShell.AddCommand(hiddenChild)

	// Shell subcommand
	shellCmd := shell.New(rootShell, nil, prompt.OptionLivePrefix(a.prefix))
	run := shellCmd.Run

	shellCmd.Use = "shell [zone]"
	shellCmd.Args = cobra.MaximumNArgs(1)
	shellCmd.PersistentPreRunE = a.ShellInit
	shellCmd.Run = func(cmd *cobra.Command, args []string) {
		rootShell.ResetFlags()

		run(cmd, args)
	}

	rootCmd.AddCommand(shellCmd)

	return rootCmd
}

func (a *App) root(shellCommand bool) *cobra.Command {
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
		a.cat(),
		a.head(),
		a.chmod(),
		a.inherit(),
		a.list(),
		a.tree(),
		a.stat(),
		a.meta(),
		a.checksum(),
		a.version(),
	)

	if a.passwordStore != nil {
		rootCmd.AddCommand(a.auth())
	}

	if a.workdirStore != nil || shellCommand {
		rootCmd.AddCommand(a.pwd(), a.cd())
	}

	if shellCommand {
		rootCmd.AddCommand(a.local())
	}

	if !shellCommand && a.updater != nil {
		rootCmd.AddCommand(a.update())
	}

	return rootCmd
}

func (a *App) prefix() (string, bool) {
	return fmt.Sprintf("%s > %s > ", a.name, a.Workdir), true
}

// ResetClient closes the client and sets it to nil
// This is used for the shell in combination with the "auth" command,
// to switch between zones.
func (a *App) ResetClient() error {
	if a.Client == nil {
		return nil
	}

	if err := a.Client.Close(); err != nil {
		return err
	}

	a.Client = nil

	return nil
}

// Init sets up the client for most commands.
// It is used under the PersistentPreRunE hook.
// To override, either adjust SkipInit or implement your own PersistentPreRunE hook.
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

// ResetInit sets up the client for the "auth" command.
// It ensures a previous client is closed, useful for the shell.
func (a *App) ResetInit(cmd *cobra.Command, args []string) error {
	if err := a.ResetClient(); err != nil {
		return err
	}

	return a.Init(cmd, args)
}

// ResetInitConfigStore sets up the client for the "auth" command,
// in case two or more arguments are provided and a ConfigStore is configured.
func (a *App) ResetInitConfigStore(cmd *cobra.Command, args []string) error {
	if err := a.ResetClient(); err != nil {
		return err
	}

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

// ShellInit calls Init but does not fail on error,
// instead it writes an invitation to authenticate.
// Useful for the shell only.
func (a *App) ShellInit(cmd *cobra.Command, args []string) error {
	a.inShell = true

	err := a.Init(cmd, args)
	if err == nil || a.configStore == nil {
		return err
	}

	fmt.Println(err)

	a.Workdir = "not authenticated"

	return nil
}

func (a *App) init(cmd *cobra.Command, zone string) error {
	// Load zone and start client
	ctx := a.Context

	if strings.HasPrefix(cmd.Use, "auth ") {
		ctx = context.WithValue(ctx, ForceReauthentication, true)
	}

	env, dialer, err := a.loadEnv(ctx, zone)
	if err != nil {
		// Doesn't make sense to print usage here
		cmd.SilenceUsage = true

		return InitError{a, err}
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
	if err != nil {
		// Doesn't make sense to print usage here
		cmd.SilenceUsage = true

		return InitError{a, err}
	}

	if a.Workdir == "" {
		a.Workdir = fmt.Sprintf("/%s", env.Zone)
	}

	return nil
}

type InitError struct {
	App *App
	Err error
}

func (e InitError) Error() string {
	var instructions string

	if e.App.configStore != nil {
		if e.App.inShell {
			instructions = fmt.Sprintf("\nRun `auth <%s>` to authenticate.", strings.Join(e.App.configStoreArgs, "> <"))
		} else {
			instructions = fmt.Sprintf("\nRun `iron auth <%s>` to authenticate.", strings.Join(e.App.configStoreArgs, "> <"))
		}
	}

	if errors.Is(e.Err, os.ErrNotExist) {
		return fmt.Sprintf("%s%s", e.Err.Error(), instructions)
	}

	return fmt.Sprintf("failed to initialize client: %s%s", e.Err.Error(), instructions)
}

func SkipInit(cmd *cobra.Command) bool {
	if cmd.Use == "__complete [command-line]" || cmd.Use == "help [command]" || cmd.Use == "completion" || cmd.Use == "version" || cmd.Use == "update" || cmd.Use == "local" {
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
