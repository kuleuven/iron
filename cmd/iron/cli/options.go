package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/scramble"
)

// FileLoader loads an irods environment from a file and returns a Loader.
// The environment is loaded from the file, and the password is read from the
// .irodsA file in the same directory, or the file specified by the
// IRODS_AUTHENTICATION_FILE environment variable if set.
func FileLoader(file string) Loader {
	return func(_ context.Context, _ string) (iron.Env, iron.DialFunc, error) {
		var env iron.Env

		err := env.LoadFromFile(file)

		env.UseModernAuth = true

		if env.Password == "" && err == nil {
			authFile := filepath.Join(filepath.Dir(file), ".irodsA")

			if f, ok := os.LookupEnv("IRODS_AUTHENTICATION_FILE"); ok {
				authFile = f
			}

			env.Password, err = ReadAuthFile(authFile)
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

// Loader is a function that loads an iRODS environment from a file.
// The requested zone will be autodetected from the command, if possible,
// and can be used by the loader in a multi-zone environments to determine
// the right connection parameters. If no zone can be detected, an empty string
// is passed. In this case, the Loader might fall back to a default zone,
// or return ErrZoneRequired.
type Loader func(ctx context.Context, zone string) (iron.Env, iron.DialFunc, error)

type Option func(*App)

func WithLoader(loader Loader) Option {
	return func(a *App) {
		a.loadEnv = loader
	}
}

func WithName(name string) Option {
	return func(a *App) {
		a.name = name
	}
}
