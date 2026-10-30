package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/inconshreveable/mousetrap"
	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/cmd/iron/cli"
	"github.com/spf13/cobra"
)

// Version embedded from ldflags
var version string

// Source of updates. Embed empty string in ldflags to disable.
var updateSlug = "kuleuven/iron"

func main() {
	exitCode := 0

	defer func() {
		os.Exit(exitCode)
	}()

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: unable to determine home directory: %v\n", err)

		exitCode = 1

		return
	}

	if home == "" {
		home = "."
	}

	config := home + "/.irods/irods_environment.json"

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGPIPE)

	defer stop()

	app := cli.New(
		ctx,
		cli.WithVersion(version),
		cli.WithEnvironmentFlag(),
		cli.WithLoader(cli.FileLoader(config)),
		cli.WithConfigurator(cli.ConfigurationTemplate(config, iron.Env{
			AuthScheme:      "pam_interactive",
			DefaultResource: "default",
		}), []string{"user name", "zone name", "host"}),
		cli.WithWorkdirToConfigFile(config),
	)

	if updateSlug != "" {
		cli.WithUpdater(selfupdate.DefaultUpdater(), selfupdate.ParseSlug(updateSlug))(app)
	}

	defer app.Close()

	cmd := app.Command()

	// Default to interactive shell when no command is specified
	if len(os.Args) < 2 {
		cmd.SetArgs([]string{"shell"})
	}

	handleExplorerStartup(home)

	if err := cmd.ExecuteContext(ctx); err != nil {
		exitCode = 1
	}
}

// handleExplorerStartup handles special behavior when started by Windows Explorer.
// This includes changing to the home directory.
func handleExplorerStartup(home string) {
	if !mousetrap.StartedByExplorer() {
		return
	}

	cobra.MousetrapHelpText = ""

	if err := os.Chdir(home); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not change to home directory: %v\n", err)
	}
}
