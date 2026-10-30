package cli

import (
	"context"
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
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func New(_ context.Context, options ...Option) *App {
	home := os.Getenv("HOME")

	if home == "" {
		home = "."
	}

	app := &App{
		name:   "iron",
		loader: FileLoader(home + "/.irods/irods_environment.json"),
	}

	for _, option := range options {
		option(app)
	}

	return app
}

type App struct {
	*iron.Client

	name                   string
	environmentFunc        EnvironmentFunc
	loader                 Loader
	configurator           Configurator
	configurationArguments []string
	environmentFlag        bool
	disableAuthentication  bool
	persistWorkdir         WorkdirSaver
	commandHook            func(*cobra.Command, bool)

	releaseVersion string
	updater        *selfupdate.Updater
	repo           selfupdate.RepositorySlug

	Admin          bool
	Debug          int
	Native         bool
	Environment    string
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

const (
	defaultTerminalWidth   = 80
	promptInputWidth       = 20
	promptSeparatorsWidth  = 6
	minimumPromptPathWidth = 12
)

// Name returns the application name (used for client telemetry, prompts, etc.).
func (a *App) Name() string { return a.name }

// Loader returns the configured environment Loader, or nil if none.
func (a *App) Loader() Loader { return a.loader }

// Configurator returns the configured Configurator, or nil if none.
func (a *App) Configurator() Configurator { return a.configurator }

// ConfigurationArguments returns the positional argument labels expected by the
// ConfigStore (e.g. ["user name", "zone name", "host"]). Returns nil if no
// ConfigStore is configured.
func (a *App) ConfigurationArguments() []string { return a.configurationArguments }

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
	shellCmd.Use = "shell [environment]"
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

func (a *App) root(shellCommand bool) *cobra.Command { //nolint:funlen
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
		a.whoami(),
		a.sleep(),
		a.ps(),
		a.query(),
	)

	if !a.disableAuthentication {
		rootCmd.AddCommand(a.auth())
	}

	if a.persistWorkdir != nil || shellCommand {
		rootCmd.AddCommand(a.pwd(), a.cd())
	}

	if shellCommand {
		rootCmd.AddCommand(a.local())
	}

	if !shellCommand && a.updater != nil {
		rootCmd.AddCommand(a.update())
	}

	if !shellCommand {
		rootCmd.AddCommand(a.exec())
		a.addRootFlags(rootCmd)
	}

	if a.commandHook != nil {
		a.commandHook(rootCmd, shellCommand)
	}

	return rootCmd
}

func (a *App) addRootFlags(rootCmd *cobra.Command) {
	overrideEnvCommands := "'cd'"

	if a.configurator != nil {
		overrideEnvCommands += " or 'auth [flags] <" + strings.Join(a.configurationArguments, "> <") + ">'"
	}

	rootCmd.PersistentFlags().CountVarP(&a.Debug, "debug", "v", "Enable debug output")
	rootCmd.PersistentFlags().BoolVar(&a.Admin, "admin", false, "Enable admin access")
	rootCmd.PersistentFlags().BoolVar(&a.Native, "native", false, "Use native protocol")
	rootCmd.PersistentFlags().StringVar(&a.Workdir, "workdir", a.Workdir, fmt.Sprintf("Override the working directory derived from the environment for this command. Using this flag does not prevent the application from changing the environment's working directory when combined with %s.", overrideEnvCommands))
	rootCmd.PersistentFlags().DurationVar(&a.PamTTL, "ttl", 168*time.Hour, "In case pam authentication is used, request a session that is valid for the given duration. This value is rounded down to the nearest hour.")

	if a.environmentFlag {
		rootCmd.PersistentFlags().StringVarP(&a.Environment, "environment", "e", "", "Environment label to use. Useful for multi-zone environments to specify which iRODS configuration to use (e.g., 'production', 'staging'). If not set, uses automatic detection.")
	}
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
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = defaultTerminalWidth
	}

	pathWidth := max(width-len([]rune(a.name))-promptInputWidth-promptSeparatorsWidth, minimumPromptPathWidth)

	return fmt.Sprintf("%s > %s > ", a.name, abbreviatePath(a.Workdir, pathWidth))
}

func abbreviatePath(path string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}

	pathRunes := []rune(path)
	if len(pathRunes) <= maxLength {
		return path
	}

	absolute := strings.HasPrefix(path, "/")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) == 1 && parts[0] == "" {
		return path
	}

	for i := range len(parts) - 1 {
		runes := []rune(parts[i])
		if len(runes) > 1 {
			parts[i] = string(runes[:1])
		}
	}

	shortened := strings.Join(parts, "/")
	if absolute {
		shortened = "/" + shortened
	}

	if len([]rune(shortened)) <= maxLength {
		return shortened
	}

	prefix := ".../"
	if absolute {
		prefix = "/.../"
	}

	remaining := maxLength - len([]rune(prefix))
	if remaining <= 0 {
		return string([]rune(prefix)[:maxLength])
	}

	base := []rune(parts[len(parts)-1])
	if len(base) > remaining {
		base = base[len(base)-remaining:]
	}

	return prefix + string(base)
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

const defaultEnvironment = "default"

func (a *App) Close() error {
	if a.Client == nil {
		return nil
	}

	return a.Client.Close()
}
