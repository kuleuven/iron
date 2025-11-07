package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/cmd/iron/cli"
	"github.com/spf13/cobra"
)

// Version embedded from ldflags
var version string

// Source of updates. Embed empty string in ldflags to disable.
var updateSlug = "kuleuven/iron"

func main() {
	home := os.Getenv("HOME")

	if home == "" {
		home = "."
	}

	config := home + "/.irods/irods_environment.json"

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	defer stop()

	app := cli.New(
		ctx,
		cli.WithVersion(version),
		cli.WithConfigStore(cli.FileStore(config, iron.Env{
			AuthScheme:      "pam_interactive",
			DefaultResource: "default",
		}), []string{"user name", "zone name", "host"}),
		cli.WithLoader(cli.FileLoader(config)),
		cli.WithPasswordStore(cli.FilePasswordStore(config)),
		cli.WithDefaultWorkdirFromFile(config),
	)

	if updateSlug != "" {
		cli.WithUpdater(selfupdate.DefaultUpdater(), selfupdate.ParseSlug(updateSlug))(app)
	}

	defer app.Close()

	cmd := app.Command()

	if len(os.Args) < 2 {
		cmd.SetArgs([]string{"shell"})
	}

	// Disable Cobra's mouse trap, we have our own shell to fall back to
	cobra.MousetrapHelpText = ""

	cmd.Execute() //nolint:errcheck
}
