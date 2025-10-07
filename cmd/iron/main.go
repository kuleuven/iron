package main

import (
	"context"
	"os"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/cmd/iron/cli"
	"github.com/sirupsen/logrus"
)

func main() {
	home := os.Getenv("HOME")

	if home == "" {
		home = "."
	}

	config := home + "/.irods/irods_environment.json"

	app := cli.New(
		context.Background(),
		cli.WithConfigStore(cli.FileStore(config, iron.Env{AuthScheme: "pam_interactive"}), []string{"user name", "zone name", "host"}),
		cli.WithLoader(cli.FileLoader(config)),
		cli.WithPasswordStore(cli.FilePasswordStore(config)),
		cli.WithDefaultWorkdirFromFile(config),
	)

	defer app.Close()

	if err := app.Command().Execute(); err != nil {
		logrus.Fatal(err)
	}
}
