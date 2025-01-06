package main

import (
	"context"
	"os"

	"gitea.icts.kuleuven.be/coz/iron"
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

	client, err := iron.New(context.Background(), env, iron.Option{ClientName: "iron", UseNativeProtocol: true})
	if err != nil {
		panic(err)
	}

	defer client.Close()

	testFile := "/" + env.Zone + "/home/coz/testFile2"

	handle, err := client.CreateDataObject(context.Background(), testFile, os.O_RDWR)
	if err != nil {
		panic(err)
	}

	defer handle.Close()

	n, err := handle.Write([]byte("test"))
	if err != nil {
		panic(err)
	}

	logrus.Printf("wrote %d bytes", n)

	_, err = handle.Seek(0, 0)
	if err != nil {
		panic(err)
	}

	b := make([]byte, 4)

	n, err = handle.Read(b)
	if err != nil {
		panic(err)
	}

	logrus.Printf("read %d bytes: %s", n, string(b))
}
