// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package libostree

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

/*
The testdata directory contains a simple ostree repo that can be used for testing. It was created using the generate-testdata.sh
script. testdata has been committed to this git repo so that it remains static. If you need to regenerate the testdata you can,
however, keep in mind that newer versions of ostree may produce different results and may cause tests to fail. The version
of ostree used to generate the testdata is 2023.7.
*/
func TestMain(m *testing.M) {
	_, err := os.Stat("testdata/repo/summary")
	if os.IsNotExist(err) {
		log.Fatalln("testdata/repo does not exist: please run ./generate-testdata.sh")
	}

	if err := os.MkdirAll("/tmp/libostree-pull_test", 0o755); err != nil {
		log.Fatalf("failed to create test directory: %s", err.Error())
	}

	os.Exit(m.Run())
}

func TestRepo_Pull(t *testing.T) {
	fmt.Println(os.Getwd())
	svr := httptest.NewServer(http.FileServer(http.Dir("testdata/repo")))
	defer svr.Close()

	remoteName := "local"
	remoteURL := svr.URL
	//refs := []string{
	//	"test1",
	//	"test2",
	//}

	modes := []RepoMode{
		RepoModeArchive,
		RepoModeArchiveZ2,
		RepoModeBare,
		RepoModeBareUser,
		RepoModeBareUserOnly,
		// RepoModeBareSplitXAttrs,
	}

	// Test pull for each mode
	for _, mode := range modes {
		mode := mode
		repoName := fmt.Sprintf("repo-%s", mode)
		repoPath := fmt.Sprintf("/tmp/libostree-pull_test/%s", repoName)

		t.Run(repoName, func(t *testing.T) {
			t.Cleanup(func() {
				_ = os.RemoveAll(repoPath)
			})

			t.Run(fmt.Sprintf("should create repo in %s mode", mode), func(t *testing.T) {
				repo, err := Init(repoPath, mode)
				assert.NotNil(t, repo)
				assert.NoError(t, err)
				if err != nil {
					assert.Failf(t, "failed to initialize repo", "err: %s", err.Error())
				}

				t.Run("should not fail to init twice", func(t *testing.T) {
					repo, err := Init(repoPath, mode)
					assert.NotNil(t, repo)
					assert.NoError(t, err)
				})
			})

			var repo *Repo
			t.Run("should open repo", func(t *testing.T) {
				var err error
				repo, err = Open(repoPath)
				assert.NotNil(t, repo)
				assert.NoError(t, err)
				if err != nil {
					assert.Failf(t, "failed to open repo", "err: %s", err.Error())
				}
			})

			t.Run("should create remote", func(t *testing.T) {
				err := repo.AddRemote(remoteName, remoteURL, NoGPGVerify())
				assert.NoError(t, err)

				// Manually check the config file to ensure the remote was added
				configData, err := os.ReadFile(fmt.Sprintf("%s/config", repoPath))
				if err != nil {
					t.Errorf("failed to read config file: %s", err.Error())
				}
				assert.Contains(t, string(configData), fmt.Sprintf(`[remote "%s"]`, remoteName))
				assert.Contains(t, string(configData), fmt.Sprintf(`url=%s`, remoteURL))
			})

			t.Run("should error - remote already exists", func(t *testing.T) {
				err := repo.AddRemote(remoteName, remoteURL)
				assert.Error(t, err)
			})

			t.Run("should create then delete remote", func(t *testing.T) {
				remoteToDelete := "delete-me"
				err := repo.AddRemote(remoteToDelete, remoteURL, NoGPGVerify())
				assert.NoError(t, err)

				err = repo.DeleteRemote(remoteToDelete)
				assert.NoError(t, err)
			})

			t.Run("should list remotes", func(t *testing.T) {
				remotes := repo.ListRemotes()
				assert.Equal(t, remotes, []string{remoteName})
			})

			t.Run("should cancel pull", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				err := repo.Pull(
					ctx,
					remoteName,
					Flags(Mirror|TrustedHTTP),
				)
				assert.Error(t, err)
				if err == nil {
					assert.Failf(t, "failed to cancel pull", "err: %s", err.Error())
				}
			})

			// TODO: Repeat the following tests for only a specific ref
			t.Run("should pull entire repo", func(t *testing.T) {
				err := repo.Pull(
					context.TODO(),
					remoteName,
					Flags(Mirror|TrustedHTTP),
				)
				assert.NoError(t, err)
				if err != nil {
					assert.Failf(t, "failed to pull repo", "err: %s", err.Error())
				}
			})

			t.Run("should list refs from original repo", func(t *testing.T) {
				repoHeadsPrefix := "testdata/repo/refs/heads"
				branch1Name := "test1"
				branch2Name := "long/branch/name/test2"
				branch1Data, err := os.ReadFile(filepath.Join(repoHeadsPrefix, branch1Name))
				branch2Data, err := os.ReadFile(filepath.Join(repoHeadsPrefix, branch2Name))
				if err != nil {
					t.Errorf("failed to read refs file: %s", err.Error())
				}
				expectedBranches := map[string]string{
					branch1Name: strings.TrimRight(string(branch1Data), "\n"),
					branch2Name: strings.TrimRight(string(branch2Data), "\n"),
				}

				refs, err := repo.ListRefsExt(ListRefsExtFlagsNone)
				assert.NoError(t, err)
				if err != nil {
					assert.Failf(t, "failed to list refs", "err: %s", err.Error())
				}
				assert.Len(t, refs, 2)

				// This could be a static list of assert statements but the loop captures unexpected extra refs.
				for _, ref := range refs {
					assert.NotEmpty(t, ref.Name)
					assert.NotEmpty(t, ref.Checksum)

					// Ensure the ref is one of the expected branches
					expectedChecksum, exists := expectedBranches[ref.Name]
					assert.True(t, exists, "unexpected ref: %s", ref.Name)
					assert.Equal(t, expectedChecksum, ref.Checksum)
				}
			})

			t.Run("should generate summary file", func(t *testing.T) {
				err := repo.RegenerateSummary()
				assert.NoError(t, err)
				_, err = os.Stat(fmt.Sprintf("%s/summary", repoPath))
				assert.NoError(t, err)
				if err != nil {
					assert.Failf(t, "failed to stat summary file", "err: %s", err.Error())
				}
			})
		})
	}
}
