package cli

import (
	"slices"
	"strings"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/api"
	"github.com/spf13/cobra"
)

// CompleteArgs implements shell completion for the given command and arguments.
// It tries to find the zone of the previous arguments and detect the argument
// type of the last given argument. If the zone cannot be determined, or if
// there are at least two different zones involved in the arguments, it returns
// a default directive.
// See completeArgument for the actual completion logic.
func (a *App) CompleteArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var (
		zone    string
		argType ArgType
	)

	// Use zone of the client if we have one
	if a.Client != nil {
		zone = a.Zone
	}

	// Try to find the zone of the previous arguments
	// and detect the argument type of the last given argument
	for i, a := range a.ArgTypes(cmd) {
		if i == len(args) {
			argType = a

			break
		}

		if z := GetZone(args[i], a); zone == "" || z != "" && zone == z {
			zone = z
		} else if z != "" {
			// Don't proceed - there are at least two zones involved
			return nil, cobra.ShellCompDirectiveDefault
		}
	}

	if z := GetZone(a.Workdir, CollectionPath); zone == "" || z != "" && zone == z {
		zone = z
	} else if z != "" {
		// Don't proceed - there are at least two zones involved
		return nil, cobra.ShellCompDirectiveDefault
	}

	return a.completeArgument(zone, toComplete, argType)
}

// completeArgument provides shell completion for the specified argument type.
// It returns a list of completion candidates and a directive for the shell.
// Depending on the argType, it may return file, directory, or custom completions.
// If the argument is a path, it attempts to determine the zone from the path format
// and uses it to load the iRODS environment if necessary. If a client is available,
// it uses completeIrodsArgument to generate completions. Otherwise, it initializes
// a new client to perform completion.
func (a *App) completeArgument(zone, toComplete string, argType ArgType) ([]string, cobra.ShellCompDirective) {
	if argType == LocalFile {
		return nil, cobra.ShellCompDirectiveDefault
	}

	if argType == LocalDirectory {
		return nil, cobra.ShellCompDirectiveFilterDirs
	}

	if !slices.Contains(IrodsArguments, argType) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Get zone from argument that needs completion, but only if it is at least of thet format /zone/
	if zone == "" {
		if parts := strings.SplitN(toComplete, "/", 3); len(parts) == 3 {
			zone = parts[1]
		}
	}

	if a.Client != nil {
		return a.completeIrodsArgument(a.Client, toComplete, argType), cobra.ShellCompDirectiveNoFileComp
	}

	// Load client to complete the argument
	env, dialer, err := a.loadEnv(a.Context, zone)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client, err := iron.New(a.Context, env, iron.Option{
		ClientName:        a.name,
		Admin:             a.Admin,
		UseNativeProtocol: a.Native,
		DialFunc:          dialer,
	})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	defer client.Close()

	return a.completeIrodsArgument(client, toComplete, argType), cobra.ShellCompDirectiveNoSpace
}

// completeIrodsArgument provides shell completion for the specified argument type
// and completes the given path in the iRODS file system.
func (a *App) completeIrodsArgument(client *iron.Client, toComplete string, argType ArgType) []string {
	relativeBase, filePrefix := api.Split(toComplete)

	absoluteBase := a.Path(relativeBase)

	if absoluteBase == "" || absoluteBase == "/" {
		return []string{"/" + client.Zone + "/"}
	}

	if relativeBase != "" {
		relativeBase += "/"
	}

	var completions []string

	client.Walk(a.Context, absoluteBase, func(path string, info api.Record, err error) error { //nolint:errcheck
		if path == absoluteBase {
			return api.SkipSubDirs
		}

		if err != nil || !strings.HasPrefix(info.Name(), filePrefix) || argType == ObjectPath && info.IsDir() || argType == CollectionPath && !info.IsDir() || argType == TargetPath && !info.IsDir() {
			return err
		}

		name := info.Name()

		if info.IsDir() {
			name += "/"
		}

		completions = append(completions, relativeBase+name)

		return nil
	})

	slices.Sort(completions)

	return completions
}
