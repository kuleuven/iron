package main

import (
	"context"
	"os"

	"gitea.icts.kuleuven.be/coz/iron"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.StandardLogger().SetLevel(logrus.DebugLevel)

	var env iron.Env

	home := os.Getenv("HOME")

	if home == "" {
		home = "."
	}

	if err := env.LoadFromFile(home + "/.irods/irods_environment.json"); err != nil {
		panic(err)
	}

	client, err := iron.New(env, "iron", 1)
	if err != nil {
		panic(err)
	}

	defer client.Close()

	conn, err := client.Connect(context.Background())
	if err != nil {
		panic(err)
	}

	defer conn.Close()
}
