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

/*
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
*/
