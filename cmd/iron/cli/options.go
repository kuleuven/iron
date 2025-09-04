package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/scramble"
)

// Loader is a function that loads an iRODS environment from a file.
// The requested zone will be autodetected from the command, if possible,
// and can be used by the loader in a multi-zone environments to determine
// the right connection parameters. If no zone can be detected, an empty string
// is passed. In this case, the Loader might fall back to a default zone,
// or return ErrZoneRequired.
type Loader func(ctx context.Context, zone string) (iron.Env, iron.DialFunc, error)

// PasswordStore is a function that can persist an irods password, if wanted,
// for use in subsequent commands.
type PasswordStore func(ctx context.Context, password string) error

// WorkdirStore is a function that can persist a workdir, if wanted,
// for use in subsequent commands.
type WorkdirStore func(ctx context.Context, workdir string) error

type Option func(*App)

func WithLoader(loader Loader) Option {
	return func(a *App) {
		a.loadEnv = loader
	}
}

type ContextKey string

var ForceReauthentication ContextKey = "force_reauthentication"

// FileLoader loads an irods environment from a file and returns a Loader.
// The environment is loaded from the file, and the password is read from the
// .irodsA file in the same directory, or the file specified by the
// IRODS_AUTHENTICATION_FILE environment variable if set.
func FileLoader(file string) Loader {
	return func(ctx context.Context, _ string) (iron.Env, iron.DialFunc, error) {
		var env iron.Env

		err := env.LoadFromFile(file)

		if forceReauthentication, ok := ctx.Value(ForceReauthentication).(bool); ok && forceReauthentication {
			// Force reauthentication
			env.Password = ""
		} else if env.AuthScheme != "native" || env.Password == "" {
			// Try to read the password from the .irodsA file
			authFile := filepath.Join(filepath.Dir(file), ".irodsA")

			if f, ok := os.LookupEnv("IRODS_AUTHENTICATION_FILE"); ok {
				authFile = f
			}

			env.Password, err = ReadAuthFile(authFile)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			} else if err == nil {
				env.AuthScheme = "native"
			}
		}

		return env, iron.DefaultDialFunc, err
	}
}

// ReadAuthFile reads the contents of a file and decodes it according to the
// iRODS scramble algorithm. The uid of the file owner is used to decode
// the contents.
// The file is expected to have been created with the iRODS `iinit` command.
func ReadAuthFile(authFile string) (string, error) {
	f, err := os.Open(authFile)
	if err != nil {
		return "", err
	}

	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", err
	}

	encoded, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return scramble.DecodeIrodsA(encoded, uid(fi))
}

// WriteAuthFile writes the given password to a file using the iRODS
// scramble algorithm. The file is expected to be the .irodsA file
// used by the iinit command. The file is created if it doesn't exist,
// and the permissions are set to 0600. If the file is already present,
// the content is overwritten. The uid of the file owner is used to
// encode the password.
func WriteAuthFile(authFile, password string) error {
	f, err := os.OpenFile(authFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	_, err = f.Write(scramble.EncodeIrodsA(password, uid(fi), time.Now()))

	return err
}

func WithPasswordStore(store PasswordStore) Option {
	return func(a *App) {
		a.passwordStore = store
	}
}

func FilePasswordStore(file string) PasswordStore {
	return func(_ context.Context, password string) error {
		authFile := filepath.Join(filepath.Dir(file), ".irodsA")

		return WriteAuthFile(authFile, password)
	}
}

func WithName(name string) Option {
	return func(a *App) {
		a.name = name
	}
}

func WithDefaultWorkdir(workdir string) Option {
	return func(a *App) {
		a.Workdir = workdir
	}
}

func WithDefaultWorkdirFromFile(file string) Option {
	return func(a *App) {
		if wd, err := GetWorkdirFromFile(file); err == nil {
			a.Workdir = wd
		}

		a.workdirStore = func(_ context.Context, workdir string) error {
			return StoreWorkdirInFile(file, workdir)
		}
	}
}

// GetWorkdirFromFile returns the working directory as stored in an irods environment file.
// The file is expected to have been created with the iRODS `icd` command.
func GetWorkdirFromFile(file string) (string, error) {
	pidFile := fmt.Sprintf("%s.%d", file, os.Getppid())

	if _, err := os.Stat(pidFile); err == nil {
		file = pidFile
	}

	f, err := os.Open(file)
	if err != nil {
		return "", err
	}

	defer f.Close()

	var c struct {
		WorkingDirectory string `json:"irods_cwd"`
	}

	if json.NewDecoder(f).Decode(&c) != nil {
		return "", err
	}

	return c.WorkingDirectory, nil
}

// StoreWorkdirInFile stores the working directory in an irods environment file.
// The file is expected to have been created with the iRODS `icd` command.
func StoreWorkdirInFile(file, workdir string) error {
	pidFile := fmt.Sprintf("%s.%d", file, os.Getppid())

	s, err := os.Open(file)
	if err != nil {
		return err
	}

	defer s.Close()

	m := map[string]any{}

	if err := json.NewDecoder(s).Decode(&m); err != nil {
		return err
	}

	m["irods_cwd"] = workdir

	t, err := os.OpenFile(pidFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	defer t.Close()

	return json.NewEncoder(t).Encode(m)
}
