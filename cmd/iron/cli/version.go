package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
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

	return semver.MustParse("9.0.0+dev")
}

func (a *App) CheckUpdate() {
	latest, err := a.LatestVersion()
	if err != nil {
		logrus.Debugf("failed to check for updates: %s", err)

		return
	}

	current := a.Version()

	if latest.LessThanEqual(current) {
		return
	}

	logrus.Infof("Currently running version %s of %s. Version %s has been released and is available for installation. Please update with `%s update`.", current, a.name, latest, a.name)
}

func (a *App) LatestVersion() (*semver.Version, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user cache dir: %w", err)
	}

	cacheDir = filepath.Join(cacheDir, "iron")

	releaseFile := filepath.Join(cacheDir, "latest-release")

	// Read from cache file
	if fi, err := os.Stat(releaseFile); err == nil && time.Since(fi.ModTime()) < 24*time.Hour {
		payload, err := os.ReadFile(releaseFile)
		if err != nil {
			return nil, err
		}

		return semver.NewVersion(string(payload))
	}

	// Retrieve latest release
	latest, found, err := a.updater.DetectLatest(a.Context, a.repo)
	if err != nil {
		return nil, fmt.Errorf("error occurred while detecting version: %w", err)
	}

	if !found {
		return nil, fmt.Errorf("latest version for %s/%s could not be found from github repository", runtime.GOOS, runtime.GOARCH)
	}

	// Write to cache file
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		logrus.Debugf("failed to create dir %s: %s", cacheDir, err)
	} else if err := os.WriteFile(releaseFile, []byte(latest.Version()), 0o600); err != nil {
		logrus.Debugf("failed to write %s: %s", releaseFile, err)
	}

	return semver.NewVersion(latest.Version())
}

func (a *App) Update(path string, allowDowngrade bool) error {
	latest, found, err := a.updater.DetectLatest(a.Context, a.repo)
	if err != nil {
		return fmt.Errorf("error occurred while detecting version: %w", err)
	}

	if !found {
		return fmt.Errorf("latest version for %s/%s could not be found from github repository", runtime.GOOS, runtime.GOARCH)
	}

	current := a.Version()

	fmt.Printf("Current version: %s\nLatest release:  %s\n", current, latest.Version())

	if latest.LessOrEqual(current.String()) && !allowDowngrade {
		fmt.Println("Nothing to update.")

		return nil
	}

	if err := a.updater.UpdateTo(a.Context, latest, path); errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("cannot update binary: path %s is not writable", path)
	} else if err != nil {
		return fmt.Errorf("error occurred while updating binary: %w", err)
	}

	fmt.Printf("Successfully updated to version %s.\n", latest.Version())

	return nil
}
