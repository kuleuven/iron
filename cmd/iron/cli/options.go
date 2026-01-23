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

	"github.com/creativeprojects/go-selfupdate"
	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/scramble"
)

// Loader is a function that loads an iRODS environment e.g. from a file.
// The requested zone will be autodetected from the command, if possible,
// and can be used by the loader in a multi-zone environments to determine
// the right connection parameters. If no zone can be detected, an empty string
// is passed. In this case, the Loader might fall back to a default zone,
// or return ErrZoneRequired.
type Loader func(ctx context.Context, zone string) (iron.Env, iron.DialFunc, error)

// ConfigStore is a function that can store an iRODS environment e.g. to a file,
// based on CLI parameters to generate the configuration. The function must return
// the configured zone or an error. The returned zone name will subsequentially be
// passed to the Loader, and using the resulting iRODS environment, a first iRODS
// connection can be established. Lastly, the password will be persisted using
// the PasswordStore function.
type ConfigStore func(ctx context.Context, args []string) (string, error)

// PasswordStore is a function that can persist an irods password, if wanted,
// for use in subsequent commands.
type PasswordStore func(ctx context.Context, env iron.Env, password string) error

// WorkdirStore is a function that can persist a workdir, if wanted,
// for use in subsequent commands.
type WorkdirStore func(ctx context.Context, workdir string) error

type Option func(*App)

// WithVersion sets the version for the App.
func WithVersion(version string) Option {
	return func(a *App) {
		a.releaseVersion = version
	}
}

// WithLoader sets the Loader for the App, which is used to load the environment
// from a file, or other source.
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

		if err := env.LoadFromFile(file); err != nil {
			return env, nil, err
		}

		if forceReauthentication, ok := ctx.Value(ForceReauthentication).(bool); ok && forceReauthentication {
			// Force reauthentication
			env.Password = ""
		} else if env.AuthScheme != "native" || env.Password == "" {
			// Try to read the password from the .irodsA file
			authFile := filepath.Join(filepath.Dir(file), ".irodsA")

			if f, ok := os.LookupEnv("IRODS_AUTHENTICATION_FILE"); ok {
				authFile = f
			}

			if password, err := ReadAuthFile(authFile, env.IrodsAuthenticationUID); err == nil {
				env.Password = password
				env.AuthScheme = "native"
			}
		}

		if env.AuthScheme == "pam_interactive" {
			env.PersistentState = &persistentState{
				file: filepath.Join(filepath.Dir(file), ".irodsA.json"),
			}
		}

		return env, iron.DefaultDialFunc, nil
	}
}

type persistentState struct {
	file string
}

func (p *persistentState) Load(m map[string]any) error {
	f, err := os.Open(p.file)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}

	defer f.Close()

	return json.NewDecoder(f).Decode(&m)
}

func (p *persistentState) Save(m map[string]any) error {
	f, err := os.OpenFile(p.file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewEncoder(f).Encode(m)
}

// WithConfigStore returns an Option that configures an App with a ConfigStore
// and positional arguments for the store. The ConfigStore is used to store
// the iRODS environment configuration, and the positional arguments are used
// to generate and parse the CLI command. At least 2 arguments are expected.
func WithConfigStore(store ConfigStore, argLabels []string) Option {
	return func(a *App) {
		if len(argLabels) < 2 {
			panic("argLabels must have at least 2 elements")
		}

		a.configStore = store
		a.configStoreArgs = argLabels
	}
}

func FileStore(file string, template iron.Env) ConfigStore {
	return func(ctx context.Context, args []string) (string, error) {
		env := template

		env.Username = args[0]
		env.Zone = args[1]
		env.Host = args[2]

		env.ApplyDefaults()

		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			return "", err
		}

		f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return "", err
		}

		defer f.Close()

		err = json.NewEncoder(f).Encode(env)
		if err != nil {
			defer os.Remove(f.Name())

			return "", err
		}

		return env.Zone, nil
	}
}

// ReadAuthFile reads the contents of a file and decodes it according to the
// iRODS scramble algorithm. If no uid is given, the uid of the file is used
// to decode the contents.
// The file is expected to have been created with the iRODS `iinit` command.
func ReadAuthFile(authFile string, uid *int) (string, error) {
	f, err := os.Open(authFile)
	if err != nil {
		return "", err
	}

	defer f.Close()

	var descrableUID int

	if uid == nil {
		fi, err := f.Stat()
		if err != nil {
			return "", err
		}

		descrableUID = uidOfFile(fi)
	} else {
		descrableUID = *uid
	}

	encoded, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return scramble.DecodeIrodsA(encoded, descrableUID)
}

// WriteAuthFile writes the given password to a file using the iRODS
// scramble algorithm. The file is expected to be the .irodsA file
// used by the iinit command. The file is created if it doesn't exist,
// and the permissions are set to 0600. If the file is already present,
// the content is overwritten. The uid of the file owner is used to
// encode the password.
func WriteAuthFile(authFile, password string, uid *int) error {
	f, err := os.OpenFile(authFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	defer f.Close()

	var scrableUID int

	if uid == nil {
		fi, err := f.Stat()
		if err != nil {
			return err
		}

		scrableUID = uidOfFile(fi)
	} else {
		scrableUID = *uid
	}

	_, err = f.Write(scramble.EncodeIrodsA(password, scrableUID, time.Now()))

	return err
}

// WithPasswordStore sets the password store function for the App.
// This function is used to store the password for the iRODS environment
// after the user has authenticated with the environment. The password
// store function is called with the password and the App's context.
// The password store function is expected to store the password securely
// and to be able to retrieve the password later.
func WithPasswordStore(store PasswordStore) Option {
	return func(a *App) {
		a.passwordStore = store
	}
}

func FilePasswordStore(file string) PasswordStore {
	return func(_ context.Context, env iron.Env, password string) error {
		authFile := filepath.Join(filepath.Dir(file), ".irodsA")

		return WriteAuthFile(authFile, password, env.IrodsAuthenticationUID)
	}
}

// WithName sets the name of the App. This name is used to generate the
// name of the command line tool when running the App.
// The name is used to generate the command line tool help text and to
// generate the command line tool completion. The name is also used
// to generate the configuration file name when persisting the App's
// configuration.
func WithName(name string) Option {
	return func(a *App) {
		a.name = name
	}
}

// WithDefaultWorkdir sets the default working directory for the App.
// This option is used to specify the default working directory
// when creating a new App. The default working directory is used
// when the user does not specify a working directory when running
// commands.
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
	} else if grandParent, err := findParentOf(os.Getppid()); err == nil {
		pidFile = fmt.Sprintf("%s.%d", file, grandParent)

		if _, err := os.Stat(pidFile); err == nil {
			file = pidFile
		}
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

func WithUpdater(updater *selfupdate.Updater, repo selfupdate.RepositorySlug) Option {
	return func(a *App) {
		a.updater = updater
		a.repo = repo
	}
}
