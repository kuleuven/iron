package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/cmd/iron/cli"
)

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
		cli.WithConfigStore(cli.FileStore(config, iron.Env{
			AuthScheme:      "pam_interactive",
			DefaultResource: "default",
		}), []string{"user name", "zone name", "host"}),
		cli.WithLoader(cli.FileLoader(config)),
		cli.WithPasswordStore(cli.FilePasswordStore(config)),
		cli.WithDefaultWorkdirFromFile(config),
	)

	defer app.Close()

	app.Command().Execute() //nolint:errcheck
}
