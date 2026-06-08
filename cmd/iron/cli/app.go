package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
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

	// Prompt overrides the iron.Prompt used during authentication. When
	// nil, Init falls back to iron.Bot{} for NonInteractive runs and to
	// iron.StdPrompt otherwise (the default chosen by iron.New).
	Prompt iron.Prompt

	inShell   bool
	inTesting bool
}

// Name returns the application name (used for client telemetry, prompts, etc.).
func (a *App) Name() string { return a.name }

// Loader returns the configured environment Loader, or nil if none.
func (a *App) Loader() Loader { return a.loadEnv }

// ConfigStore returns the configured ConfigStore, or nil if none.
func (a *App) ConfigStore() ConfigStore { return a.configStore }

// ConfigStoreArgs returns the positional argument labels expected by the
// ConfigStore (e.g. ["user name", "zone name", "host"]). Returns nil if no
// ConfigStore is configured.
func (a *App) ConfigStoreArgs() []string { return a.configStoreArgs }

// PasswordStore returns the configured PasswordStore, or nil if none.
func (a *App) PasswordStore() PasswordStore { return a.passwordStore }

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
	shellCmd.PersistentPreRunE = a.PreRunShell

	// Open subcommand
	openURLCmd := a.xopen()

	rootCmd.AddCommand(shellCmd, openURLCmd)

	return rootCmd
}

// Exec runs a single iron CLI command (e.g. "mkdir", "-p", "/zone/home/peter")
// against the App's already-initialized Client. It is intended for embedding
// callers (such as iron-gui) that drive the App outside of cobra. The
// command's stdout / stderr are routed to the supplied writers (nil falls
// back to os.Stdout / os.Stderr). The App.Client must already be set.
func (a *App) Exec(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	if stdout == nil {
		stdout = os.Stdout
	}

	if stderr == nil {
		stderr = os.Stderr
	}

	// Build a fresh shell-style root each call so that command flag parser
	// state (e.g. -p on `mkdir`) is reset between invocations.
	rootCmd := a.root(true)     //nolint:contextcheck
	hiddenChild := a.root(true) //nolint:contextcheck
	hiddenChild.Hidden = true
	rootCmd.AddCommand(hiddenChild)

	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs(args)

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	a.inShell = true

	return rootCmd.ExecuteContext(ctx)
}

func (a *App) root(shellCommand bool) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               a.name,
		Short:             "Golang client for iRODS",
		PersistentPreRunE: a.PreRun,
	}

	rootCmd.AddCommand(
		a.mkdir(),
		a.rmdir(),
		a.rm(),
		a.unlock(),
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
		a.find(),
		a.tree(),
		a.stat(),
		a.meta(),
		a.checksum(),
		a.checksums(),
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
		Hidden:        true,
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

// PreRun sets up the client for most commands.
// It is used under the PersistentPreRunE hook.
// To override, either adjust SkipInit or implement your own PersistentPreRunE hook.
func (a *App) PreRun(cmd *cobra.Command, args []string) error {
	// If a.Debug is zero, this sets logrus to Info
	logrus.SetLevel(logrus.DebugLevel + logrus.Level(a.Debug-1))

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

	ctx := cmd.Context()

	if err := a.Init(ctx, zone); err != nil {
		// Doesn't make sense to print usage here
		cmd.SilenceUsage = true

		return err
	}

	return nil
}

// PreRunAuth sets up the client for the "auth" command.
// It ensures a previous client is closed, useful for the shell.
func (a *App) PreRunAuth(cmd *cobra.Command, args []string) error {
	if err := a.ResetClient(); err != nil {
		return err
	}

	if !a.inTesting {
		// Stamp the context with ForceReauthentication=true to bypass cached credentials.
		cmd.SetContext(context.WithValue(cmd.Context(), ForceReauthentication, true))
	}

	return a.PreRun(cmd, args)
}

// PreRunAuthConfigStore sets up the client for the "auth" command,
// in case two or more arguments are provided and a ConfigStore is configured.
func (a *App) PreRunAuthConfigStore(cmd *cobra.Command, args []string) error {
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

	ctx := cmd.Context()

	if !a.inTesting {
		ctx = context.WithValue(ctx, ForceReauthentication, true)
	}

	if err := a.Init(ctx, zone); err != nil {
		// Doesn't make sense to print usage here
		cmd.SilenceUsage = true

		return err
	}

	return nil
}

// PreRunShell calls PreRun but does not fail on error,
// instead it writes an invitation to authenticate.
// Useful for the shell only.
func (a *App) PreRunShell(cmd *cobra.Command, args []string) error {
	a.inShell = true

	err := a.PreRun(cmd, args)
	if err == nil || a.configStore == nil {
		return err
	}

	fmt.Println(err)

	a.Workdir = "not authenticated"

	return nil
}

// Init loads the iRODS environment for the given zone (empty string =
// the Loader's default) and constructs the underlying iron.Client. It is
// the cobra-independent core of Init: callers that drive the App outside
// of cobra (e.g. iron-gui) can use it directly. Stamp the context with
// ForceReauthentication=true to bypass cached credentials.
//
// On failure, returns InitError wrapping the underlying error (and any
// partially-loaded env, when available). On success, App.Client is set
// and App.Workdir is defaulted if it was empty.
func (a *App) Init(ctx context.Context, zone string) error {
	env, dialer, err := a.loadEnv(ctx, zone)
	if err != nil {
		return InitError{a, env, err}
	}

	env.GeneratedPasswordTimeout = a.PamTTL

	clientName := a.name

	// Telemetry: send version, except for prereleases
	if version := a.Version(); version.Prerelease() == "" && version.Metadata() == "" {
		clientName = fmt.Sprintf("%s-%s", clientName, version.String())
	}

	authPrompt := a.Prompt

	if authPrompt == nil && a.NonInteractive {
		authPrompt = iron.Bot{}
	}

	a.Client, err = iron.New(ctx, env, iron.Option{
		ClientName:           clientName,
		Admin:                a.Admin,
		UseNativeProtocol:    a.Native,
		MaxConns:             16,
		DialFunc:             dialer,
		AuthenticationPrompt: authPrompt,
	})
	if err != nil {
		return InitError{a, env, err}
	}

	if a.Workdir != "" {
		return nil
	}

	return a.setDefaultWorkdir(ctx)
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
	if cmd.Use == "__complete [command-line]" || cmd.Use == "help [command]" || cmd.Use == "completion" || cmd.Use == versionCommandName || cmd.Use == updateCommandName || cmd.Use == "local" || cmd.Use == "exit" {
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
