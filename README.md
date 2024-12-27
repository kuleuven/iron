# iRODS Native Interface in Go

Replacement for <https://github.com/cyverse/go-irodsclient> that provides a clean, simple and stable interface to iRODS.

## Implementation choices

* The client requires 4.3.2 or later.
* Simplified communication code: types of messages are defined in `msg/types.go`, and are marshaled using the right format (xml, json or binary) by `msg.Marshal`. The binary part (`Bs`) of messages is not marshaled by `msg.Marshal`/`msg.Unmarshal` but directly read or written to the provided buffers in `msg.Read`/`msg.Write`.
* Clients can choose between `iron.Conn` (one single connection) and `iron.Client` (a pool of connections) to use the provided API.
* The `Truncate` and `Touch` methods are only available on open file handles, to help identifying the right replica to adjust. Because irods only supports those operations when the file is closed, the operations are actually done on the replica when the file is closed.