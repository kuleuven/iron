package cli

import (
	"context"
	"fmt"
)

func (a *App) setDefaultWorkdir(ctx context.Context) error {
	if a.Client == nil {
		return nil
	}

	homedir := fmt.Sprintf("/%s/home/%s", a.Client.Env().Zone, a.Client.Env().Username)

	// Check if the homedir exists
	if _, err := a.Client.GetCollection(ctx, homedir); err == nil {
		a.Workdir = homedir
	} else {
		// Fall back to "/<zone>"
		a.Workdir = fmt.Sprintf("/%s", a.Client.Env().Zone)
	}

	return a.saveWorkdir(ctx)
}

func (a *App) saveWorkdir(ctx context.Context) error {
	if a.Client == nil {
		return nil
	}

	if a.workdirStore != nil {
		return a.workdirStore(ctx, a.Workdir)
	}

	return nil
}
