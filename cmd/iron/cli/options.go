package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/kuleuven/iron"
	"github.com/spf13/cobra"
)

// appContextKey is the context key for storing the App pointer.
type appContextKey struct{}

var appKey appContextKey

// EnvironmentFunc is a helper function that attempts to detect an iRODS environment
// label based on the provided command-line arguments and the current working directory.
// If not set, the environment will always be an empty string.
// The toComplete parameter represents the argument that is currently being completed by the shell.
type EnvironmentFunc func(ctx context.Context, cmd *cobra.Command, args []string, toComplete string) (string, error)

// Loader is a function that loads an iRODS environment e.g. from a file.
// The requested environment label is either based on the -e or --environment flag, or can
// be autodetected from the command with the EnvironmentFunc. It can be used by the loader
// in a multi-zone environment to determine the right connection parameters.
type Loader func(ctx context.Context, environment string) (iron.Env, iron.DialFunc, error)

// Configurator is an optional function that can store an iRODS environment e.g. to a file,
// based on CLI parameters to generate the configuration. The function must return
// the configured zone or an error. The returned zone name will subsequentially be
// passed to the Loader, and using the resulting iRODS environment, a first iRODS
// connection can be established.
type Configurator func(ctx context.Context, environment string, args []string) error

// WorkdirSaver is a function that can persist a workdir, if wanted,
// for use in subsequent commands.
// This should be combined with a corresponding call to the WithDefaultWorkdir function.
type WorkdirSaver func(ctx context.Context, environment string, workdir string) error

type Option func(*App)

// WithVersion sets the version for the App.
func WithVersion(version string) Option {
	return func(a *App) {
		a.releaseVersion = version
	}
}

// WithEnvironmentFlag returns an Option that adds an -e or --environment flag to the CLI command.
// The flag allows the user to specify the environment label to use.
func WithEnvironmentFlag() Option {
	return func(a *App) {
		a.environmentFlag = true
	}
}

// WithEnvironmentFunc sets the EnvironmentFunc for the App, which is used to
// detect the iRODS environment label based on the command-line arguments and
// the current working directory.
func WithEnvironmentFunc(environmentFunc EnvironmentFunc) Option {
	return func(a *App) {
		a.environmentFunc = environmentFunc
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
//
// FileLoader validates the configuration file exists, is readable, contains valid JSON,
// and has all required fields (irods_host, irods_port, irods_user_name, irods_zone_name).
// It extracts the App from the context to check if a configurator is available,
// and returns helpful error messages tailored to what tools are available.
func FileLoader(file string) Loader {
	loader := func(ctx context.Context, environment string) (iron.Env, iron.DialFunc, error) {
		var env iron.Env

		// Rewrite the file path based on the environment before loading it.
		environmentFile := RewriteConfigFileForEnvironment(file, environment)

		// Find the runtime configuration file based on the current process ID and its parent processes.
		environmentFile = GetRuntimeConfigFile(environmentFile)

		// Validate the configuration file before attempting to load it.
		// This provides early, actionable error messages if configuration is missing or broken.
		if err := validateConfigFile(environmentFile, ctx); err != nil {
			return env, nil, err
		}

		// Load the environment from the runtime configuration file.
		if err := env.LoadFromFile(environmentFile); err != nil {
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

// RewriteConfigFileForEnvironment rewrites the given file path for the specified environment.
// If the environment is the default environment, the original file path is returned.
// Otherwise, the file path is modified to include the environment as a suffix to the folder name.
func RewriteConfigFileForEnvironment(file, environment string) string {
	if environment == defaultEnvironment {
		return file
	}

	folder := filepath.Dir(file) + "_" + environment

	return filepath.Join(folder, filepath.Base(file))
}

// GetRuntimeConfigFile returns the runtime file for the given base file, considering the current process ID and its parent processes.
func GetRuntimeConfigFile(file string) string {
	pidFile := fmt.Sprintf("%s.%d", file, os.Getppid())

	if _, err := os.Stat(pidFile); err == nil { //nolint:gosec
		return pidFile
	}

	if grandParent, err := findParentOf(os.Getppid()); err == nil {
		pidFile = fmt.Sprintf("%s.%d", file, grandParent)

		if _, err := os.Stat(pidFile); err == nil { //nolint:gosec
			return pidFile
		}
	}

	return file
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
	return func(ctx context.Context, environment string, args []string) error {
		env := template

		env.Username = args[0]
		env.Zone = args[1]
		env.Host = args[2]

		env.ApplyDefaults()

		// Rewrite the file path based on the environment before storing the configuration.
		environmentFile := RewriteConfigFileForEnvironment(file, environment)

		if err := os.MkdirAll(filepath.Dir(environmentFile), 0o700); err != nil {
			return err
		}

		f, err := os.OpenFile(environmentFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}

		defer f.Close()

		err = json.NewEncoder(f).Encode(env) //nolint:gosec
		if err != nil {
			defer os.Remove(environmentFile)

			return err
		}

		// Remove all possible runtime files for this environment, as they may contain outdated information.
		return removeRuntimeFiles(file)
	}
}

func removeRuntimeFiles(file string) error {
	matches, err := filepath.Glob(fmt.Sprintf("%s.[0-9]*", file))

	for _, match := range matches {
		err = errors.Join(err, os.Remove(match))
	}

	return err
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

// WithWorkdirToConfigFile sets the function used to persist the working directory for the App
// based on the contents of an irods configuration file, or a copy of it identified by the process ID (PID).
// The working directory will be stored in the file using the configured WorkdirSaver.
func WithWorkdirToConfigFile(file string) Option {
	return func(a *App) {
		a.persistWorkdir = func(ctx context.Context, environment, workdir string) error {
			return StoreWorkdirInFile(ctx, RewriteConfigFileForEnvironment(file, environment), workdir)
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

// validateConfigFile checks if the config file exists, is readable, contains valid JSON,
// and has all required fields. Returns a helpful error message if validation fails.
// This function is called by FileLoader to provide early, actionable error messages.
// It extracts the App from the context to check if a configurator is available.
func validateConfigFile(configPath string, ctx context.Context) error {
	// Extract App from context to check if configurator is available
	app, _ := ctx.Value(appKey).(*App)
	hasConfigurator := app != nil && app.configurator != nil

	// Check if file exists
	// Note: gosec G703 warning is acceptable here - configPath comes from App initialization,
	// not from untrusted user input, and this is specifically config file validation.
	_, err := os.Stat(configPath) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			if hasConfigurator {
				return fmt.Errorf("configuration file not found at %s. To set up iRODS configuration, run the configurator command interactively", configPath)
			}

			return fmt.Errorf("configuration file not found at %s. Create the configuration file manually or use a configuration tool", configPath)
		}

		return fmt.Errorf("cannot access configuration file %s: %w", configPath, err)
	}

	// Check if file is readable
	// Note: gosec G703 warning is acceptable here - configPath comes from App initialization,
	// not from untrusted user input, and this is specifically config file validation.
	file, err := os.Open(configPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("cannot read configuration file %s: %w", configPath, err)
	}
	defer file.Close()

	// Validate JSON structure and check required fields
	var config map[string]any

	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return fmt.Errorf("invalid JSON in configuration file %s: %w", configPath, err)
	}

	// Check for required fields
	requiredFields := []string{"irods_host", "irods_port", "irods_user_name", "irods_zone_name"}

	var missingFields []string

	for _, field := range requiredFields {
		if _, ok := config[field]; !ok {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) > 0 {
		missing := strings.Join(missingFields, ", ")
		if hasConfigurator {
			return fmt.Errorf("configuration file is missing required fields: %s. To reconfigure iRODS, run the configurator command interactively", missing)
		}

		return fmt.Errorf("configuration file is missing required fields: %s. Update the configuration file with the missing fields", missing)
	}

	return nil
}
