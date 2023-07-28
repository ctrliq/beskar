package yumplugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.ciq.dev/beskar/internal/pkg/yumplugin/pkg/yumdb"
	"gocloud.dev/blob"
)

func (p *Plugin) AddPackageToDatabase(ctx context.Context, id, repository, packageDir string, execute, keepDatabaseDir bool) (string, error) {
	if execute {
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		args := []string{
			"add-pkg",
			fmt.Sprintf("-config-dir=%s", p.beskarYumConfig.ConfigDirectory),
			fmt.Sprintf("-dir=%s", packageDir),
			fmt.Sprintf("-id=%s", id),
			fmt.Sprintf("-repository=%s", repository),
		}

		if keepDatabaseDir {
			args = append(args, "-keep-db-dir")
		}

		//nolint:gosec // ignore internal use only
		cmd := exec.CommandContext(ctx, os.Args[0], args...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		err := cmd.Run()
		if err != nil {
			return "", errors.New(stderr.String())
		}

		return strings.TrimSpace(stdout.String()), nil
	}

	return p.addPackageToDatabase(ctx, id, repository, packageDir, keepDatabaseDir)
}

func (p *Plugin) addPackageToDatabase(ctx context.Context, id, repository, packageDir string, keepDatabaseDir bool) (string, error) {
	key := filepath.Join("/", filepath.Dir(repository), "doltdb.tar.lz4")

	dbPath, err := os.MkdirTemp(p.beskarYumConfig.DataDir, "db-")
	if err != nil {
		return "", fmt.Errorf("while creating temporary database directory: %w", err)
	}
	if !keepDatabaseDir {
		defer os.RemoveAll(dbPath)
	}

	remoteReader, err := p.bucket.NewReader(ctx, key, &blob.ReaderOptions{})
	if err == nil {
		defer remoteReader.Close()

		if err := yumdb.Clone(dbPath, remoteReader); err != nil {
			return "", fmt.Errorf("while cloning DB: %w", err)
		}
	}

	db, err := yumdb.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("while opening dolt database: %w", err)
	}
	defer db.Close()

	err = db.AddPackage(
		ctx,
		id,
		filepath.Base(packageDir),
		filepath.Join(packageDir, primaryXMLFile),
		filepath.Join(packageDir, filelistsXMLFile),
		filepath.Join(packageDir, otherXMLFile),
	)
	if err != nil {
		return "", fmt.Errorf("while adding package: %w", err)
	}

	remoteWriter, err := p.bucket.NewWriter(ctx, key, &blob.WriterOptions{})
	if err != nil {
		return "", fmt.Errorf("while initializing s3 object writer: %w", err)
	}

	if err := yumdb.Push(dbPath, remoteWriter); err != nil {
		_ = remoteWriter.Close()
		return "", fmt.Errorf("while pushing database to s3 bucket: %w", err)
	}

	return dbPath, remoteWriter.Close()
}
