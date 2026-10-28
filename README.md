# iRODS Native Interface in Go

The `iron` utility is both a golang library and a command line client, that provides a clean, simple and stable interface to iRODS.

[![Quality Gate Status](https://sonarqube.icts.kuleuven.be/api/project_badges/measure?project=coz%3Airon%3Amain&metric=alert_status&token=sqb_f14f2e85edf4f52db70a1b133fb98a805ebe8372)](https://sonarqube.icts.kuleuven.be/dashboard?id=coz%3Airon%3Amain)

## CLI Installation

> This documentation page contains all technical information about the library and contains only generic installation instructions.
> KU Leuven or VSC users should consult the [RDM documentation pages](https://rdm-docs.icts.kuleuven.be/mango/clients/iron.html) instead for installation instructions.
> Users from other institutes should contact their respective support staff.

The CLI binary can be downloaded directly from <https://github.com/kuleuven/iron/releases/latest>. E.g. on a Linux system or Windows WSL:

```bash
VERSION=$(curl -Ls -w %{url_effective} -o /dev/null https://github.com/kuleuven/iron/releases/latest | sed 's/.*\/v//')
mkdir -p ~/.local/bin/
curl -L -s "https://github.com/kuleuven/iron/releases/download/v${VERSION}/iron_${VERSION}_linux_amd64.tar.gz" | tar zxvf - -C ~/.local/bin/
```

Alternatively, it can be installed with `go install`:

```bash
go install github.com/kuleuven/iron/cmd/iron@latest
```

Note that the released windows binaries are not signed. Windows users may alternatively use an installer for a signed `iron.exe` binary that we publish on <https://icts-p-coz-iron-releases.cloud.icts.kuleuven.be/iron/>. Linux users can find deb and rpm packages at the same location.

## CLI usage

![screencast](assets/screencast.gif)

The CLI expects a `~.irods/irods_environment.json` file being present, with native or pam_password authentication. The password should either be given in this file under the `pam_password` key, or the irods authentication file `.irods/.irodsA` must be present.

```shell
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

### Command files

Use `iron exec workflow.iron` to feed multiple commands to one transfer worker,
connection pool, and progress display. If no file is given, `iron exec` reads
commands from standard input, for example `iron exec < workflow.iron` or
`printf 'mkdir -p /zone/home/alice/archive\nwait\n' | iron exec`.
Input is read one line at a time.
Transfers can overlap; submission blocks when the worker's transfer capacity is
occupied.

```text
# workflow.iron
upload './results/part 1.csv' /zone/home/alice/incoming/part1.csv
upload ./results/part2.csv /zone/home/alice/incoming/part2.csv
wait
mkdir -p /zone/home/alice/archive
cp /zone/home/alice/incoming/part1.csv /zone/home/alice/archive/
cp /zone/home/alice/incoming/part2.csv /zone/home/alice/archive/
wait
mv /zone/home/alice/archive/part1.csv /zone/home/alice/archive/final.csv
```

Supported commands are `cp`, `mv`, `mkdir`, `upload`/`put`, `download`/`get`,
and `wait`. A `wait` line waits for all preceding work, including verification
and timestamp updates, without closing the progress display. End-of-file also
waits. **Command order alone does not guarantee completion order.** Insert
`wait` before dependent commands or operations on overlapping paths. Synchronous
`mkdir` and `mv` commands do not implicitly wait for earlier transfers.

Transfer flags apply to the individual commands in the file. For example:

```text
upload --exclusive ./results/part1.csv /zone/home/alice/incoming/part1.csv
wait
upload --include '*.csv' --ignore '*.tmp' ./results/ /zone/home/alice/incoming/
wait
cp --metadata /zone/home/alice/incoming/part1.csv /zone/home/alice/archive/
wait
cp --remote --metadata /zone/home/alice/archive/part1.csv /archive/home/alice/
```

Each line starts with fresh flag defaults; flags do not carry over to subsequent
commands. `cp` supports `--metadata`, `--delete-skip-trash`, and
`--no-update-in-place`; `upload` supports `--delete-skip-trash` and
`--no-update-in-place`. The common transfer flags, such as `--exclusive`,
`--delete`, `--checksum`, `--verify-checksum`, `--include`, and `--ignore`, are
available on `cp`, `upload`/`put`, and `download`/`get`. Operation-specific flags
such as `mkdir -p` and `mv --preserve-mtime` remain available.

Worker options `--threads` and `--dry-run`, and the destination selection
`--target-environment`, remain on `exec` and cannot be set on individual lines:

```sh
iron exec --threads 5 workflow.iron
iron exec --target-environment archive workflow.iron
iron exec --dry-run workflow.iron
```

`--target-environment` selects the destination for per-line `cp --remote`
commands. Plain `cp` continues to copy within the source environment, so both
types of copy can be mixed in one command file. `cp --remote` requires
`--target-environment`; uploads, downloads, mkdir, and moves use the source
environment.

Relative copy targets use the source environment's working directory for `cp`
and the target environment's working directory for `cp --remote`.
Unlike standalone `cp`, command files default to
`--sync-modtime=true` for all transfers; disable it on a command line with
`--sync-modtime=false`. The worker reserves `--threads` connections in addition
to an indexing connection, so library callers must configure enough client
connections.

Blank lines and whole-line `#` comments are allowed. Paths can be quoted,
including paths containing spaces; Windows path backslashes are preserved.
Relative local paths use the invocation directory, not the command file's
directory. Remote paths use the selected environment's working directory.
The usual trailing-slash source/target rules still apply.

There is no shell execution, wildcard expansion, variable substitution,
command chaining, or control flow. Generate concrete `.iron` commands with an
external script when wildcard selection is needed.

The existing progress display stays indeterminate while commands are being read
and files discovered, since its totals can grow. The same display stays open
across `wait` barriers.

Input and command errors stop submission; already submitted work is drained.
Transfer errors use the worker's existing error handler and are collected by
`wait` or end-of-file when the handler returns an error. Errors identify the
command file (or `stdin`) and line. Earlier changes are not rolled back;
in-flight operations may finish, and a syntax error on a later line can be
reported after earlier operations have executed.

Dry-run previews operations without copying, moving, deleting, creating
destinations, or updating metadata/timestamps. As with existing transfer
dry-runs, checksum comparison may compute and store missing server checksums.
Dry-run does not simulate files or directories an earlier command would create:
a later dependent preview can fail because those paths do not exist yet.

## GUI

A Fyne-based desktop GUI is available as a separate project <https://github.com/kuleuven/iron-gui>. It reuses the same `~/.irods/irods_environment.json`, `.irodsA` and `.irodsA.json` files as the CLI, supports `pam_interactive` authentication from a login form, and lets you browse collections, transfer files edit metadata/permissions, and compute checksums.

The GUI project should be considered as a Proof of Concept only at this time. For that reason, the source code is currently private, but binary releases are available at <https://icts-p-coz-iron-releases.cloud.icts.kuleuven.be/iron/>.

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
```

## Implementation choices

Transfer configuration is split between `transfer.Options` (worker resources,
dry-run mode, and reporting) and `transfer.Flags` (per-operation behavior).
Use `Client.RunWorker` to create a worker and wait for its submitted operations.
Pass flags as the last argument to each operation:

```go
err := client.RunWorker(transfer.Options{MaxThreads: 5}, func(worker *transfer.Worker) {
    worker.DownloadDir(ctx, local, remote, transfer.Flags{Exclusive: true})
})
```

For standalone checksum verification, use `transfer.VerifyLocalToRemote`.
Each asynchronous operation retains its own flags, so subsequent submissions
can use different settings without changing work already in progress.

Transfer callbacks receive the operation's context, including values attached by
the caller: `ProgressHandler(context.Context, transfer.Progress)` and
`ErrorHandler(context.Context, string, string, error) error`. This also applies
to checksum progress callbacks and synchronization error handlers. Callbacks may
run concurrently. The progress bar's `Handler` and `ErrorHandler` methods accept
the same context-first arguments and can be assigned directly to these callbacks.
`transfer.ProgressLabel` also accepts the operation context as its first argument.
Use `transfer.WithLabel(ctx, "label")` to annotate progress and error output
through `transfer.ProgressLabel`. Custom callbacks can retrieve it with `transfer.Label(ctx)`.
Command files use the filename and line number as the operation label.
Worker errors are reported asynchronously; `Drain` and `Wait` wait for pending
error callbacks and return any errors those callbacks propagate.

* The client currently requires 4.3.2 or later. We currently use it against 5.0.2 production services. 
* New features might require a recent version of irods, i.e. support for older versions is not guaranteed.
* Breaking changes in further versions are avoided as much as possible, but the API might change in the future if it is beneficial.
* Simplified communication code: types of messages are defined in `msg/types.go`, and are marshaled using the right format (xml, json or binary) by `msg.Marshal`. The binary part (`Bs`) of messages is not marshaled by `msg.Marshal`/`msg.Unmarshal` but directly read or written to the provided buffers in `msg.Read`/`msg.Write`.
* Clients can choose between `iron.Conn` (one single connection) and `iron.Client` (a pool of connections) to use the provided API.
* The `Truncate` and `Touch` methods are only available on open file handles, to help identifying the right replica to adjust. Because irods only supports those operations when the file is closed, the operations are actually done on the replica when the file is closed.
* This client also attempts to support the native protocol, but this should be considered experimental.
