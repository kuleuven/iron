package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/kuleuven/iron"
	"github.com/spf13/cobra"
)

// Loader is a function that loads an iRODS environment e.g. from a file.
// The requested zone will be autodetected from the command, if possible,
// and can be used by the loader in a multi-zone environments to determine
// the right connection parameters. If no zone can be detected, an empty string
// is passed. In this case, the Loader might fall back to a default zone,
// or return ErrZoneRequired.
type Loader func(ctx context.Context, zone string) (iron.Env, iron.DialFunc, error)

// Configurator is an optional function that can store an iRODS environment e.g. to a file,
// based on CLI parameters to generate the configuration. The function must return
// the configured zone or an error. The returned zone name will subsequentially be
// passed to the Loader, and using the resulting iRODS environment, a first iRODS
// connection can be established.
type Configurator func(ctx context.Context, args []string) (string, error)

// WorkdirSaver is a function that can persist a workdir, if wanted,
// for use in subsequent commands.
// This should be combined with a corresponding call to the WithDefaultWorkdir function.
type WorkdirSaver func(ctx context.Context, workdir string) error

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
		a.loader = loader
	}
}

type ContextKey string

var ForceReauthentication ContextKey = "force_reauthentication"

// FileLoader loads an irods environment from a file and returns a Loader.
// The environment is loaded from the file, and the password is read from the
// .irodsA file in the same directory, or the file specified by the
// IRODS_AUTHENTICATION_FILE environment variable if set.
//
// If the SOCKS_PROXY environment variable is set, the returned Loader's
// DialFunc routes iRODS connections through that SOCKS5 proxy. See
// WrapLoaderWithSOCKS5 for the accepted proxy URL formats.
func FileLoader(file string) Loader {
	loader := func(ctx context.Context, _ string) (iron.Env, iron.DialFunc, error) {
		var env iron.Env

		if err := env.LoadFromFile(file); err != nil {
			return env, nil, err
		}

		// Allow to override the authentication file location with the IRODS_AUTHENTICATION_FILE environment variable.
		if f, ok := os.LookupEnv("IRODS_AUTHENTICATION_FILE"); ok {
			env.PersistentState = &iron.FilePersistentState{
				AuthenticationFile: f,
				AuthenticationUID:  env.AuthenticationUID,
				StateFile:          f + ".json",
			}
		}

		return env, iron.DefaultDialFunc, nil
	}

	return WrapLoaderWithSOCKS5(loader, socksProxyFromEnv())
}

// WithConfigurator returns an Option that configures an App with a ConfigureFunc
// and positional arguments for the store. The ConfigureFunc is used to store
// the iRODS environment configuration, and the positional arguments are used
// to generate and parse the CLI command. At least 2 arguments are expected.
func WithConfigurator(store Configurator, argLabels []string) Option {
	return func(a *App) {
		if len(argLabels) < 2 {
			panic("argLabels must have at least 2 elements")
		}

		a.configurator = store
		a.configurationArguments = argLabels
	}
}

func ConfigurationTemplate(file string, template iron.Env) Configurator {
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

		err = json.NewEncoder(f).Encode(env) //nolint:gosec
		if err != nil {
			defer os.Remove(f.Name())

			return "", err
		}

		return env.Zone, nil
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

// WithWorkdirSaver sets the function used to persist the working directory for the App.
// This option is used to specify a custom function that determines how the working
// directory is stored. The function should return an error if the saving fails.
func WithWorkdirSaver(saver WorkdirSaver) Option {
	return func(a *App) {
		a.persistWorkdir = saver
	}
}

// WithDefaultWorkdirFromFile sets the default working directory for the App based on the contents
// of a irods configuration file, or a copy of it identified by the process ID (PID).
// The working directory will also be persisted to the file using the configured WorkdirSaver.
func WithDefaultWorkdirFromFile(file string) Option {
	return func(a *App) {
		if wd, err := GetWorkdirFromFile(file); err == nil {
			a.Workdir = wd
		}

		a.persistWorkdir = func(ctx context.Context, workdir string) error {
			return StoreWorkdirInFile(ctx, file, workdir)
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
func StoreWorkdirInFile(ctx context.Context, file, workdir string) error {
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

	// Don't store the working directory if it's the absolute default
	if defaultDir := fmt.Sprintf("/%s/home/%s", m["irods_zone"], m["irods_user_name"]); workdir == defaultDir {
		delete(m, "irods_cwd")
	}

	pidFile := fmt.Sprintf("%s.%d", file, os.Getppid())

	// If ran after authenticating, set the workdir globally
	if ctx.Value(ForceReauthentication) == true {
		pidFile = file
	}

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

func WithCommandHook(hook func(*cobra.Command, bool)) Option {
	return func(a *App) {
		a.commandHook = hook
	}
}

// WithoutAuthentication disables the auth command for the App.
func WithoutAuthentication() Option {
	return func(a *App) {
		a.disableAuthentication = true
	}
}
