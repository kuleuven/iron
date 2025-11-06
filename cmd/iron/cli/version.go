package cli

import (
	"runtime/debug"

	"github.com/Masterminds/semver/v3"
)

func (a *App) Version() *semver.Version {
	// If version was passed as ld flag, use that.
	if a.releaseVersion != "" {
		if parsed, err := semver.NewVersion(a.releaseVersion); err == nil {
			return parsed
		}
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		if version, err := semver.NewVersion(info.Main.Version); err == nil {
			return version
		}
	}

	return semver.MustParse("0.0.0+dev")
}
