package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/elk-language/go-prompt"
	"github.com/google/shlex"
	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/cmd/iron/shell"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func New(_ context.Context, options ...Option) *App {
	home := os.Getenv("HOME")

	if home == "" {
		home = "."
	}

	app := &App{
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

	Admin          bool
	Debug          int
	Native         bool
	Workdir        string
	PamTTL         time.Duration
	NonInteractive bool

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
	shellCmd := shell.New(rootShell, prompt.WithPrefixCallback(a.prefix))
	shellCmd.Use = "shell [zone]"
	shellCmd.Args = cobra.MaximumNArgs(1)
	shellCmd.PersistentPreRunE = a.ShellInit

	// Open subcommand
	openURLCmd := a.xopen()

	rootCmd.AddCommand(shellCmd, openURLCmd)

	return rootCmd
}

func (a *App) root(shellCommand bool) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               a.name,
		Short:             "Golang client for iRODS",
		PersistentPreRunE: a.Init,
	}

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
		a.save(),
		a.chmod(),
		a.inherit(),
		a.list(),
		a.tree(),
		a.stat(),
		a.meta(),
		a.checksum(),
		a.version(),
		a.sleep(),
		a.ps(),
		a.query(),
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

	if !shellCommand {
		rootCmd.PersistentFlags().CountVarP(&a.Debug, "debug", "v", "Enable debug output")
		rootCmd.PersistentFlags().BoolVar(&a.Admin, "admin", false, "Enable admin access")
		rootCmd.PersistentFlags().BoolVar(&a.Native, "native", false, "Use native protocol")
		rootCmd.PersistentFlags().StringVar(&a.Workdir, "workdir", a.Workdir, "Working directory")
		rootCmd.PersistentFlags().DurationVar(&a.PamTTL, "ttl", 168*time.Hour, "In case pam authentication is used, request a session that is valid for the given duration. This value is rounded down to the nearest hour.")
	}

	return rootCmd
}

func (a *App) xopen() *cobra.Command {
	return &cobra.Command{
		Use:   "x-open [url]",
		Short: "Open a special url, for browser-initiated commands.",
		Args:  cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			uri, err := url.Parse(args[0])
			if err != nil {
				return xopenError(cmd, fmt.Errorf("invalid url: %w", err))
			}

			if uri.Scheme != a.name {
				return xopenError(cmd, fmt.Errorf("invalid url, can only open %s:// urls", a.name))
			}

			minVersion, err := semver.NewVersion(uri.Host)
			if err != nil {
				return xopenError(cmd, fmt.Errorf("uri contains invalid minimum version: %w", err))
			}

			if curVersion := a.Version(); curVersion.LessThan(minVersion) {
				err = fmt.Errorf("script requires minimum version is %s, but current version is %s. Please update your installation of %s", minVersion, curVersion, a.name)

				return xopenError(cmd, err)
			}

			rootCmd := a.root(true)

			for line := range strings.SplitSeq(uri.Path, "/") {
				if err = a.executeCommand(rootCmd, line); err != nil {
					return xopenError(cmd, err)
				}
			}

			if uri.Query().Has("shell") {
				// Drop to shell
				hiddenChild := a.root(true)
				hiddenChild.Hidden = true
				rootCmd.AddCommand(hiddenChild)

				shell.New(rootCmd, prompt.WithPrefixCallback(a.prefix)).Run(rootCmd, nil)

				return nil
			}

			return xopenError(cmd, err)
		},
	}
}

func xopenError(cmd *cobra.Command, err error) error {
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err.Error())
	}

	fmt.Fprintf(cmd.OutOrStdout(), "[Press enter to exit]\n")
	fmt.Fscanln(cmd.InOrStdin()) //nolint:errcheck

	return err
}

func (a *App) executeCommand(cmd *cobra.Command, line string) error {
	if line == "" {
		return nil
	}

	line, err := url.PathUnescape(line)
	if err != nil {
		return err
	}

	prefix := fmt.Sprintf("%s > %s", a.name, a.Workdir)

	if a.Workdir == "" {
		prefix = a.name
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s%s >%s %s\n", Blue, prefix, Reset, line)

	args, err := shlex.Split(line)
	if err != nil {
		return err
	}

	shell.ResetArgs(cmd, args)

	cmd.SetArgs(args)

	return cmd.ExecuteContext(cmd.Context())
}

func (a *App) prefix() string {
	return fmt.Sprintf("%s > %s > ", a.name, a.Workdir)
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

	a.CheckUpdate(cmd.Context())

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

	zone, err := a.configStore(cmd.Context(), args)
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
	ctx := cmd.Context()

	if strings.HasPrefix(cmd.Use, "auth ") {
		ctx = context.WithValue(ctx, ForceReauthentication, true)
	}

	env, dialer, err := a.loadEnv(ctx, zone)
	if err != nil {
		// Doesn't make sense to print usage here
		cmd.SilenceUsage = true

		return InitError{a, env, err}
	}

	env.GeneratedPasswordTimeout = a.PamTTL

	clientName := a.name

	// Telemetry: send version, except for prereleases
	if version := a.Version(); version.Prerelease() == "" && version.Metadata() == "" {
		clientName = fmt.Sprintf("%s-%s", clientName, version.String())
	}

	var authPrompt iron.Prompt

	if a.NonInteractive {
		authPrompt = iron.Bot{}
	}

	a.Client, err = iron.New(cmd.Context(), env, iron.Option{
		ClientName:           clientName,
		Admin:                a.Admin,
		UseNativeProtocol:    a.Native,
		MaxConns:             16,
		DialFunc:             dialer,
		AuthenticationPrompt: authPrompt,
	})
	if err != nil {
		// Doesn't make sense to print usage here
		cmd.SilenceUsage = true

		return InitError{a, env, err}
	}

	if a.Workdir == "" {
		a.Workdir = fmt.Sprintf("/%s", env.Zone)
	}

	return nil
}

type InitError struct {
	App *App
	Env iron.Env
	Err error
}

func (e InitError) Error() string {
	var instructions string

	appPrefix := fmt.Sprintf("%s ", e.App.name)

	if e.App.inShell {
		appPrefix = ""
	}

	if e.Env.Zone != "" {
		instructions = fmt.Sprintf("\nRun `%sauth` to re-authenticate in zone %s.", appPrefix, e.Env.Zone)
	}

	if e.App.configStore != nil {
		if instructions != "" {
			instructions += fmt.Sprintf("\nOr run `%sauth <%s>` to authenticate to another zone.", appPrefix, strings.Join(e.App.configStoreArgs, "> <"))
		} else {
			instructions = fmt.Sprintf("\nRun `%sauth <%s>` to authenticate.", appPrefix, strings.Join(e.App.configStoreArgs, "> <"))
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
