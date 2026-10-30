package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kuleuven/iron"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// PreRun sets up the client for most commands.
// It is used under the PersistentPreRunE hook.
// To override, either adjust SkipInit or implement your own PersistentPreRunE hook.
func (a *App) PreRun(cmd *cobra.Command, args []string) error {
	// If a.Debug is zero, this sets logrus to Info
	logrus.SetLevel(logrus.DebugLevel + logrus.Level(a.Debug-1))

	if a.Client != nil || SkipInit(cmd) {
		return nil
	}

	ctx := cmd.Context()

	// Check for updates before initializing the client.
	a.CheckUpdate(ctx)

	// Doesn't make sense to print usage here
	cmd.SilenceUsage = true

	// Detect environment using the environment function if available.
	if err := a.DetectEnvironment(cmd, args, ""); err != nil {
		return err
	}

	return a.Init(ctx)
}

// PreRunAuth sets up the client for the "auth" command.
// It ensures a previous client is closed, useful for the shell.
func (a *App) PreRunAuth(cmd *cobra.Command, args []string) error {
	if err := a.ResetClient(); err != nil {
		return err
	}

	// Update the current environment if the environment argument is given.
	if len(args) == 1 {
		a.Environment = args[0]
	}

	if !a.inTesting {
		// Stamp the context with ForceReauthentication=true to bypass cached credentials.
		cmd.SetContext(context.WithValue(cmd.Context(), ForceReauthentication, true))
	}

	return a.PreRun(cmd, args)
}

// PreRunAuthConfigurator sets up the client for the "auth" command,
// in case two or more arguments are provided and a ConfigStore is configured.
func (a *App) PreRunAuthConfigurator(cmd *cobra.Command, args []string) error {
	if err := a.ResetClient(); err != nil {
		return err
	}

	if err := a.DetectEnvironment(cmd, args, ""); err != nil {
		return err
	}

	if err := a.configurator(cmd.Context(), a.Environment, args); err != nil {
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

	if err := a.Init(ctx); err != nil {
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

	// Update the current environment if the environment argument is given.
	if len(args) == 1 {
		a.Environment = args[0]
	}

	err := a.PreRun(cmd, args)
	if err == nil || a.configurator == nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStderr(), "Error: %v\n", err)

	a.Workdir = "not authenticated"

	return nil
}

// DetectEnvironment determines the environment to use based on the provided arguments and the current configuration.
// If an environment argument is given, it takes precedence. Otherwise, it falls back to the default environment.
func (a *App) DetectEnvironment(cmd *cobra.Command, args []string, toComplete string) error {
	// If the environment is already set, there's nothing to detect.
	if a.Environment != "" {
		// Detect
		return nil
	}

	// If no environment function is configured, we cannot detect the environment.
	if a.environmentFunc == nil {
		a.Environment = defaultEnvironment

		return nil
	}

	// Otherwise, call the environment function to determine the environment.
	environment, err := a.environmentFunc(cmd.Context(), cmd, args, toComplete)
	if err != nil {
		return err
	}

	a.Environment = environment

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
func (a *App) Init(ctx context.Context) error {
	// Add the App to the context so loaders and other components can access it
	ctx = context.WithValue(ctx, appKey, a)

	env, dialer, err := a.loader(ctx, a.Environment)
	if err != nil {
		return InitError{a, env, err}
	}

	env.GeneratedPasswordTimeout = a.PamTTL

	options := iron.Option{
		ClientName:           a.name,
		Admin:                a.Admin,
		UseNativeProtocol:    a.Native,
		MaxConns:             16,
		DialFunc:             dialer,
		AuthenticationPrompt: a.Prompt,
	}

	// Telemetry: send version, except for prereleases
	if version := a.Version(); version.Prerelease() == "" && version.Metadata() == "" {
		options.ClientName = fmt.Sprintf("%s-%s", options.ClientName, version.String())
	}

	if a.Prompt == nil && a.NonInteractive {
		options.AuthenticationPrompt = iron.Bot{}
	}

	if force, ok := ctx.Value(ForceReauthentication).(bool); ok && force {
		options.ForceReauthentication = true
	}

	a.Client, err = iron.New(ctx, env, options)
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

	if e.App.configurator != nil {
		if instructions != "" {
			instructions += fmt.Sprintf("\nOr run `%sauth <%s>` to authenticate to another zone.", appPrefix, strings.Join(e.App.configurationArguments, "> <"))
		} else {
			instructions = fmt.Sprintf("\nRun `%sauth <%s>` to authenticate.", appPrefix, strings.Join(e.App.configurationArguments, "> <"))
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
