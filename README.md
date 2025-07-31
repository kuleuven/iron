# iRODS Native Interface in Go

Replacement for <https://github.com/cyverse/go-irodsclient> that provides a clean, simple and stable interface to iRODS.

It is both a golang library as well as a command line client.

[![Quality Gate Status](https://sonarqube.icts.kuleuven.be/api/project_badges/measure?project=coz%3Airon%3Amain&metric=alert_status&token=sqb_f14f2e85edf4f52db70a1b133fb98a805ebe8372)](https://sonarqube.icts.kuleuven.be/dashboard?id=coz%3Airon%3Amain)

## Implementation choices

* The client currently requires 4.3.2 or later. It is relatively easy to extend support to 4.2.9 - 4.3.1, if the truncate and touch operations are not needed.
* Simplified communication code: types of messages are defined in `msg/types.go`, and are marshaled using the right format (xml, json or binary) by `msg.Marshal`. The binary part (`Bs`) of messages is not marshaled by `msg.Marshal`/`msg.Unmarshal` but directly read or written to the provided buffers in `msg.Read`/`msg.Write`.
* Clients can choose between `iron.Conn` (one single connection) and `iron.Client` (a pool of connections) to use the provided API.
* The `Truncate` and `Touch` methods are only available on open file handles, to help identifying the right replica to adjust. Because irods only supports those operations when the file is closed, the operations are actually done on the replica when the file is closed.
* This client also attempts to support the native protocol, but this should be considered experimental.

## CLI usage

The CLI expects a `~.irods/irods_environment.json` file being present, with native or pam_password authentication. The password should either be given in this file under the `pam_password` key, or the irods authentication file `.irods/.irodsA` must be present.

```shell
$ go install github.com/kuleuven/iron/cmd/iron@latest
$ iron
Golang client for iRODS

Usage:
  iron [command]

Available Commands:
  checksum    Compute or get the checksum of a file
  chmod       Change permissions
  completion  Generate the autocompletion script for the specified shell
  cp          Copy a data object
  create      Create a data object
  download    Sync a collection to a local directory
  get         Download a file
  help        Help about any command
  inherit     Change permission inheritance
  ls          List a collection
  mkdir       Create a collection
  mv          Move a data object or collection
  put         Upload a file
  rm          Remove a data object or collection
  rmdir       Remove a collection
  shell       Start an interactive shell.
  stat        Get information about an object or collection
  tree        Print the full tree structure beneath a collection
  upload      Sync a local directory to a collection

Flags:
      --admin            Enable admin access
  -v, --debug count      Enable debug output
  -h, --help             help for iron
      --native           Use native protocol
      --workdir string   Working directory

Use "iron [command] --help" for more information about a command.
$ iron shell
iron > /set > ls
rods  0 B  Jul 07 18:31    home/
rods  0 B  Nov 11  2022    projects/
iron > /set > ls home/
rods          0 B  Jul 07 09:50     coz/
rods          0 B  Aug 08  2024     public/
set_demo      0 B  Feb 02 09:38  +  set_demo/
set_pilot013  0 B  Jan 01 19:02     set_pilot013/
iron > /set > cd home/coz/
iron > /set/home/coz > ls
peter  0 B     Jul 07 11:11     peter/
iron > /set/home/coz > ls peter
peter  0 B     Jul 07 11:11     peter.txt
iron > /set/home/coz > local pwd
/home/peter
iron > /set/home/coz > local cd sub
iron > /set/home/coz > local pwd
/home/peter/sub
iron > /set/home/coz > download peter localdir
localdir/peter.txt
iron > /set/home/coz > exit
$ ls /home/peter/sub/localdir
peter.txt
```

## Library usage

```go
import (   
	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/api"
)

func example() error {
    var env iron.Env

    err := env.LoadFromFile(".irods/irods_environment.json")
    if err != nil {
        return err
    }

    env.Password = "my_password"

    ctx := context.Background()

    client, err := iron.New(ctx, env, iron.Option{
        ClientName:        "iron",
        Admin:             false, // Set to true to do all operations as admin, bypassing any ACLs
        MaxConns:          5,
    })
    if err != nil {
        return err
    }

    defer client.Close()

    objects, err := client.ListDataObjectsInCollection(ctx, "/path/to/data")
    if err != nil {
        return err
    }

    for _, object := range objects {
        fmt.Println(object.Path)
    }

    // Recursive walk through the tree, displaying access and metadata
    fn := func(path string, info api.Record, err error) error {
        if err != nil {
            return nil
        }

        fmt.Println(path)
        fmt.Printf("%v", info.Access())
        fmt.Printf("%v", info.Metadata())

        return nil
    }

    return client.Walk(ctx, "/path/to/more/data", fn, api.FetchAccess, api.FetchMetadata)
}