package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/kuleuven/iron/transfer"
)

const (
	downloadAlias = "get"
	uploadAlias   = "put"
)

type transferPaths struct {
	source, target string
	directory      bool
}

func (a *App) uploadPaths(args []string) (transferPaths, error) {
	targetArg := a.Workdir + "/"
	if len(args) == 2 {
		targetArg = args[1]
	}

	source := filepath.Clean(args[0])
	target := a.Path(targetArg)

	if strings.HasSuffix(targetArg, "/") && !localPathEndsWithSeparator(args[0]) {
		target = a.Path(targetArg + filepath.Base(source))
	}

	info, err := os.Stat(source)
	if err != nil {
		return transferPaths{}, err
	}

	if info.IsDir() && !strings.HasSuffix(targetArg, "/") {
		return transferPaths{}, ErrAmbiguousTarget
	}

	return transferPaths{source, target, info.IsDir()}, nil
}

func (a *App) downloadPaths(ctx context.Context, args []string) (transferPaths, error) {
	var targetArg string

	if len(args) == 2 {
		targetArg = args[1]
	} else {
		dir, err := os.Getwd()
		if err != nil {
			return transferPaths{}, err
		}

		targetArg = dir + "/"
	}

	source := a.Path(args[0])
	target := filepath.Clean(targetArg)

	if localPathEndsWithSeparator(targetArg) && !strings.HasSuffix(args[0], "/") {
		target = filepath.Join(target, Name(source))
	}

	info, err := a.GetRecord(ctx, source)
	if err != nil {
		return transferPaths{}, err
	}

	if info.IsDir() && !localPathEndsWithSeparator(targetArg) {
		return transferPaths{}, ErrAmbiguousTarget
	}

	return transferPaths{source, target, info.IsDir()}, nil
}

func (a *App) copyPaths(ctx context.Context, args []string, workdir string) (transferPaths, error) {
	source := a.Path(args[0])
	targetArg := args[1]

	if strings.HasSuffix(targetArg, "/") && !strings.HasSuffix(args[0], "/") {
		targetArg += Name(source)
	}

	info, err := a.GetRecord(ctx, source)
	if err != nil {
		return transferPaths{}, err
	}

	if info.IsDir() && !strings.HasSuffix(args[1], "/") {
		return transferPaths{}, ErrAmbiguousTarget
	}

	return transferPaths{source, a.PathIn(targetArg, workdir), info.IsDir()}, nil
}

func (paths transferPaths) upload(ctx context.Context, worker *transfer.Worker) {
	if paths.directory {
		worker.UploadDir(ctx, paths.source, paths.target)
	} else {
		worker.Upload(ctx, paths.source, paths.target)
	}
}

func (paths transferPaths) download(ctx context.Context, worker *transfer.Worker) {
	if paths.directory {
		worker.DownloadDir(ctx, paths.target, paths.source)
	} else {
		worker.Download(ctx, paths.target, paths.source)
	}
}

func (paths transferPaths) copy(ctx context.Context, worker *transfer.Worker, remote bool) {
	switch {
	case remote && paths.directory:
		worker.RemoteCopyDir(ctx, paths.source, paths.target)
	case remote:
		worker.RemoteCopy(ctx, paths.source, paths.target)
	case paths.directory:
		worker.CopyDir(ctx, paths.source, paths.target)
	default:
		worker.Copy(ctx, paths.source, paths.target)
	}
}
