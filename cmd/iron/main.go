package main

import (
	"context"
	"os"

	"gitea.icts.kuleuven.be/coz/iron"
	"gitea.icts.kuleuven.be/coz/iron/query"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.StandardLogger().SetLevel(logrus.TraceLevel)

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

	logrus.Print("/" + env.Zone)

	results := query.Query(query.ICAT_COLUMN_COLL_NAME).Where(query.ICAT_COLUMN_COLL_PARENT_NAME, "= '/"+env.Zone+"/home'").Limit(1).Execute(context.Background(), conn)

	defer results.Close()

	for results.Next() {
		var id string

		if err := results.Scan(&id); err != nil {
			panic(err)
		}
	}

	if err := results.Err(); err != nil {
		panic(err)
	}
}
