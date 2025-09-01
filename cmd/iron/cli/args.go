package cli

import (
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

type ArgType int

const (
	Unknown ArgType = iota
	ObjectPath
	CollectionPath
	Path       // object or collection
	TargetPath // object or collection
	LocalFile
	LocalDirectory
	Zone
)

var ArgumentRE = regexp.MustCompile("(<[a-zA-Z0-9 _-]+>)")

func ArgTypes(cmd *cobra.Command) []ArgType {
	var args []ArgType

	use := strings.ReplaceAll(cmd.Use, "[", "<")
	use = strings.ReplaceAll(use, "]", ">")

	matches := ArgumentRE.FindAllStringSubmatch(use, -1)

	for _, match := range matches {
		switch match[1] {
		case "<object path>":
			args = append(args, ObjectPath)
		case "<collection path>":
			args = append(args, CollectionPath)
		case "<path>":
			args = append(args, Path)
		case "<target path>":
			args = append(args, TargetPath)
		case "<local file>":
			args = append(args, LocalFile)
		case "<local directory>":
			args = append(args, LocalDirectory)
		case "<zone>":
			args = append(args, Zone)
		default:
			args = append(args, Unknown)
		}
	}

	return args
}

var IrodsArguments = []ArgType{Path, ObjectPath, CollectionPath, TargetPath}

func GetZone(arg string, t ArgType) string {
	if t == Zone {
		return arg
	}

	if !slices.Contains(IrodsArguments, t) {
		return ""
	}

	if !strings.HasPrefix(arg, "/") {
		return ""
	}

	parts := strings.SplitN(arg, "/", 3)

	if len(parts) < 2 {
		return ""
	}

	return parts[1]
}
