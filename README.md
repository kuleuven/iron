# iRODS Native Interface in Go

Replacement for <https://github.com/cyverse/go-irodsclient> that provides a clean, simple and stable interface to iRODS.

[![Quality Gate Status](https://sonarqube.icts.kuleuven.be/api/project_badges/measure?project=coz%3Airon%3Amain&metric=alert_status&token=sqb_f14f2e85edf4f52db70a1b133fb98a805ebe8372)](https://sonarqube.icts.kuleuven.be/dashboard?id=coz%3Airon%3Amain)

## Implementation choices

* The client requires 4.3.2 or later.
* Simplified communication code: types of messages are defined in `msg/types.go`, and are marshaled using the right format (xml, json or binary) by `msg.Marshal`. The binary part (`Bs`) of messages is not marshaled by `msg.Marshal`/`msg.Unmarshal` but directly read or written to the provided buffers in `msg.Read`/`msg.Write`.
* Clients can choose between `iron.Conn` (one single connection) and `iron.Client` (a pool of connections) to use the provided API.
* The `Truncate` and `Touch` methods are only available on open file handles, to help identifying the right replica to adjust. Because irods only supports those operations when the file is closed, the operations are actually done on the replica when the file is closed.

## Usage

```go
import (   
	"gitea.icts.kuleuven.be/coz/iron"
	"gitea.icts.kuleuven.be/coz/iron/api"
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