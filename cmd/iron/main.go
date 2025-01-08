package main

import (
	"context"

	"gitea.icts.kuleuven.be/coz/iron/cmd/iron/cli"
	"github.com/sirupsen/logrus"
)

func main() {
	app := cli.New(context.Background())

	defer app.Close()

	if err := app.Command().Execute(); err != nil {
		logrus.Fatal(err)
	}
}
