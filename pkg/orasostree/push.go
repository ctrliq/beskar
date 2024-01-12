package orasostree

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.ciq.dev/beskar/pkg/oras"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type OSTreeRepositoryPusher struct {
	ctx        context.Context
	dir        string
	repo       string
	jobCount   int
	nameOpts   []name.Option
	remoteOpts []remote.Option
	logger     *slog.Logger
}

func NewOSTreeRepositoryPusher(ctx context.Context, dir, repo string, jobCount int) *OSTreeRepositoryPusher {
	return &OSTreeRepositoryPusher{
		ctx:      ctx,
		dir:      dir,
		repo:     repo,
		jobCount: jobCount,
	}
}

func (p *OSTreeRepositoryPusher) WithNameOptions(opts ...name.Option) *OSTreeRepositoryPusher {
	p.nameOpts = opts
	return p
}

func (p *OSTreeRepositoryPusher) WithRemoteOptions(opts ...remote.Option) *OSTreeRepositoryPusher {
	p.remoteOpts = opts
	return p
}

func (p *OSTreeRepositoryPusher) WithLogger(logger *slog.Logger) *OSTreeRepositoryPusher {
	p.logger = logger
	return p
}

// Push walks a local ostree repository and pushes each file to the given registry.
// dir is the root directory of the ostree repository, i.e., the directory containing the summary file.
// repo is the name of the ostree repository.
// registry is the registry to push to.
func (p *OSTreeRepositoryPusher) Push() error {
	// Prove that we were given the root directory of an ostree repository
	// by checking for the existence of the config file.
	// Typically, libostree will check for the "objects" directory, but this will do just the same.
	fileInfo, err := os.Stat(filepath.Join(p.dir, FileConfig))
	if os.IsNotExist(err) || fileInfo.IsDir() {
		return fmt.Errorf("%s file not found in %s: you may need to call ostree init", FileConfig, p.dir)
	} else if err != nil {
		return fmt.Errorf("error accessing %s in %s: %w", FileConfig, p.dir, err)
	}

	// Create a worker pool to push each file in the repository concurrently.
	// ctx will be cancelled on error, and the error will be returned.
	eg, ctx := errgroup.WithContext(p.ctx)
	eg.SetLimit(p.jobCount)

	// Walk the directory tree, skipping directories and pushing each file.
	if err := filepath.WalkDir(p.dir, func(path string, d os.DirEntry, err error) error {
		// If there was an error with the file, return it.
		if err != nil {
			return fmt.Errorf("while walking %s: %w", path, err)
		}

		// Skip directories.
		if d.IsDir() {
			return nil
		}

		if ctx.Err() != nil {
			// Skip remaining files because our context has been cancelled.
			// We could return the error here, but we want to exclusively handle that error in our call to eg.Wait().
			// This is because we would never be able to handle an error returned from the last job.
			return filepath.SkipAll
		}

		eg.Go(func() error {
			if err := p.push(path); err != nil {
				return fmt.Errorf("while pushing %s: %w", path, err)
			}
			return nil
		})

		return nil
	}); err != nil {
		// We should only receive here if filepath.WalkDir() returns an error.
		// Push errors are handled below.
		return fmt.Errorf("while walking %s: %w", p.dir, err)
	}

	// Wait for all workers to finish.
	// If any worker returns an error, eg.Wait() will return that error.
	return eg.Wait()
}

func (p *OSTreeRepositoryPusher) push(path string) error {
	pusher, err := NewOSTreeFilePusher(p.dir, path, p.repo, p.nameOpts...)
	if err != nil {

		return fmt.Errorf("while creating OSTree pusher: %w", err)
	}

	if p.logger != nil {
		path = strings.TrimPrefix(path, p.dir)
		path = strings.TrimPrefix(path, "/")
		p.logger.Debug("pushing file to beskar", "file", path, "reference", pusher.Reference())
	}

	return oras.Push(pusher, p.remoteOpts...)
}
