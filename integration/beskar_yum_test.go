package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/RussellLuo/kun/pkg/werror/gcode"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.ciq.dev/beskar/integration/pkg/backoff"
	"go.ciq.dev/beskar/integration/pkg/repoconfig"
	"go.ciq.dev/beskar/integration/pkg/util"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
	"go.ciq.dev/beskar/pkg/plugins/httpcodec"
	yumv1 "go.ciq.dev/beskar/pkg/plugins/yum/api/v1"
)

func getBeskarYUMURL(path string) string {
	return "http://" + BeskarAddr + "/artifacts/yum/" + path
}

func getBeskarYUMRPMURL(repository, filename string) string {
	return getBeskarYUMURL(filepath.Join(repository, "repo", filename))
}

func beskarYUMClient() *yumv1.HTTPClient {
	httpClient := &http.Client{
		Timeout: 20 * time.Second,
	}

	client, err := yumv1.NewHTTPClient(httpcodec.JSONCodec, httpClient, getBeskarYUMURL("api/v1"))
	Expect(err).To(BeNil())

	return client
}

var _ = Describe("Beskar YUM Plugin", func() {
	repositoryConfig, err := repoconfig.Parse(repoconfigData)
	Expect(err).To(BeNil())

	configRepo := "vault-rocky-8.3-ha-debug"
	repo, ok := repositoryConfig[configRepo]
	Expect(ok).To(BeTrue())

	downloadBaseURL := repo.URL

	Describe("Test Non Mirror", Ordered, func() {
		repositoryName := configRepo + "-non-mirror"
		repositoryAPIName := "artifacts/yum/" + repositoryName

		testPackages := map[string]string{
			"booth-debugsource-1.0-6.ac1d34c.git.el8.2.x86_64.rpm": "",
			"clufter-bin-debuginfo-0.77.1-5.el8.x86_64.rpm":        "",
		}

		It("Create Repository", func() {
			properties := &yumv1.RepositoryProperties{
				GPGKey: []byte(repo.GPGKey),
			}

			err = beskarYUMClient().CreateRepository(context.Background(), repositoryAPIName, properties)
			Expect(err).To(BeNil())
		})

		It("RPM Upload", func() {
			for filename := range testPackages {
				rc, err := util.DownloadFromURL(downloadBaseURL+"/"+filename, 10*time.Second)
				Expect(err).To(BeNil())

				pusher, _, err := orasrpm.NewRPMStreamPusher(
					rc,
					repositoryName,
					name.WithDefaultRegistry(BeskarAddr),
				)
				Expect(err).To(BeNil())

				err = oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
				Expect(err).To(BeNil())
			}
		})

		It("Query Package List", func() {
			var packages []*yumv1.RepositoryPackage

			err := backoff.Retry(func() error {
				packages, err = beskarYUMClient().ListRepositoryPackages(context.Background(), repositoryAPIName, nil)
				if err != nil {
					return err
				} else if len(packages) != len(testPackages) {
					return fmt.Errorf("expected %d packages, got %d", len(testPackages), len(packages))
				}
				return nil
			})
			Expect(err).To(BeNil())

			for _, pkg := range packages {
				_, ok := testPackages[pkg.RPMName()]
				Expect(ok).To(BeTrue())
				testPackages[pkg.RPMName()] = pkg.ID
			}
		})

		It("Query Package By ID", func() {
			for filename, id := range testPackages {
				pkg, err := beskarYUMClient().GetRepositoryPackage(context.Background(), repositoryAPIName, id)
				Expect(err).To(BeNil())

				Expect(pkg).ToNot(BeNil())
				Expect(pkg.ID).To(Equal(id))
				Expect(pkg.Tag).To(Equal(util.GetTagFromFilename(filename)))
				Expect(pkg.RPMName()).To(Equal(filename))
				Expect(pkg.Verified).To(BeTrue())
			}
		})

		It("Query File By Tag", func() {
			for filename, id := range testPackages {
				tag := util.GetTagFromFilename(filename)

				pkg, err := beskarYUMClient().GetRepositoryPackageByTag(context.Background(), repositoryAPIName, tag)
				Expect(err).To(BeNil())

				Expect(pkg).ToNot(BeNil())
				Expect(pkg.ID).To(Equal(id))
				Expect(pkg.Tag).To(Equal(util.GetTagFromFilename(filename)))
				Expect(pkg.RPMName()).To(Equal(filename))
				Expect(pkg.Verified).To(BeTrue())
			}
		})

		It("Access Repository Artifacts", func() {
			for filename := range testPackages {
				info, ok := repo.Files[filename]
				Expect(ok).To(BeTrue())

				url := getBeskarYUMRPMURL(repositoryName, filename)

				rc, err := util.DownloadFromURL(url, 10*time.Second)
				Expect(err).To(BeNil())

				h := sha256.New()
				n, err := io.Copy(h, rc)

				Expect(err).To(BeNil())
				Expect(uint64(n)).To(Equal(info.Size))
				Expect(hex.EncodeToString(h.Sum(nil))).To(Equal(info.SHA256))
			}

			url := getBeskarYUMRPMURL(repositoryName, "repodata/repomd.xml")
			_, err := util.DownloadFromURL(url, 10*time.Second)
			Expect(err).To(BeNil())
		})

		It("Delete Repository Failure", func() {
			err := beskarYUMClient().DeleteRepository(context.Background(), repositoryAPIName, false)
			Expect(err).ToNot(BeNil())
			Expect(gcode.HTTPStatusCode(err)).To(Equal(http.StatusBadRequest))
		})

		It("Remove Package And List", func() {
			tag := ""
			for filename := range testPackages {
				tag = util.GetTagFromFilename(filename)
				break
			}

			err := beskarYUMClient().RemoveRepositoryPackageByTag(context.Background(), repositoryAPIName, tag)
			Expect(err).To(BeNil())

			packages, err := beskarYUMClient().ListRepositoryPackages(context.Background(), repositoryAPIName, nil)
			Expect(err).To(BeNil())
			Expect(len(packages)).To(Equal(len(testPackages) - 1))
		})

		It("Check Repository Logs", func() {
			logs, err := beskarYUMClient().ListRepositoryLogs(context.Background(), repositoryAPIName, nil)
			Expect(err).To(BeNil())
			Expect(len(logs)).To(Equal(0))
		})

		It("Delete Repository With Packages", func() {
			err := beskarYUMClient().DeleteRepository(context.Background(), repositoryAPIName, true)
			Expect(err).To(BeNil())

			storageDir := os.Getenv("BESKARYUM_STORAGE_FILESYSTEM_DIRECTORY")
			Expect(storageDir).ToNot(BeEmpty())

			// ensure it's cleaned up
			entries, err := os.ReadDir(filepath.Join(storageDir, repositoryAPIName))
			Expect(err).To(BeNil())
			Expect(len(entries)).To(Equal(0))
		})
	})

	Describe("Test Mirror", Ordered, func() {
		repositoryName := configRepo + "-mirror"
		repositoryAPIName := "artifacts/yum/" + repositoryName

		It("Create Repository", func() {
			mirror := true
			properties := &yumv1.RepositoryProperties{
				Mirror: &mirror,
				MirrorURLs: []string{
					repo.MirrorURL,
				},
				GPGKey: []byte(repo.GPGKey),
			}

			err := beskarYUMClient().CreateRepository(context.Background(), repositoryAPIName, properties)
			Expect(err).To(BeNil())
		})

		It("Get Repository", func() {
			repository, err := beskarYUMClient().GetRepository(context.Background(), repositoryAPIName)
			Expect(err).To(BeNil())
			Expect(repository).ToNot(BeNil())
			Expect(repository.Mirror).ToNot(BeNil())
			Expect(*repository.Mirror).To(BeTrue())
			Expect(repository.MirrorURLs).To(Equal([]string{repo.MirrorURL}))
			Expect(repository.GPGKey).To(Equal([]byte(repo.GPGKey)))
		})

		It("Sync Repository", func() {
			err := beskarYUMClient().SyncRepository(context.Background(), repositoryAPIName, true)
			Expect(err).To(BeNil())
		})

		It("Sync Status", func() {
			status, err := beskarYUMClient().GetRepositorySyncStatus(context.Background(), repositoryAPIName)
			Expect(err).To(BeNil())
			Expect(status.Syncing).To(BeFalse())
			Expect(status.SyncError).To(BeEmpty())
			Expect(status.SyncedPackages).To(Equal(len(repo.Files)))
			Expect(status.TotalPackages).To(Equal(len(repo.Files)))
			Expect(status.StartTime).ToNot(BeEmpty())
			Expect(status.EndTime).ToNot(BeEmpty())
		})

		It("Sync Repository With URL", func() {
			err := beskarYUMClient().SyncRepositoryWithURL(context.Background(), repositoryAPIName, repo.AuthMirrorURL, true)
			Expect(err).To(BeNil())
		})

		It("Sync Status", func() {
			status, err := beskarYUMClient().GetRepositorySyncStatus(context.Background(), repositoryAPIName)
			Expect(err).To(BeNil())
			Expect(status.Syncing).To(BeFalse())
			Expect(status.SyncError).To(BeEmpty())
			Expect(status.SyncedPackages).To(Equal(len(repo.Files)))
			Expect(status.TotalPackages).To(Equal(len(repo.Files)))
			Expect(status.StartTime).ToNot(BeEmpty())
			Expect(status.EndTime).ToNot(BeEmpty())
		})

		It("Access Repository Artifacts", func() {
			for filename := range repo.Files {
				info, ok := repo.Files[filename]
				Expect(ok).To(BeTrue())

				url := getBeskarYUMRPMURL(repositoryName, filename)

				rc, err := util.DownloadFromURL(url, 10*time.Second)
				Expect(err).To(BeNil())

				h := sha256.New()
				n, err := io.Copy(h, rc)

				Expect(err).To(BeNil())
				Expect(uint64(n)).To(Equal(info.Size))
				Expect(hex.EncodeToString(h.Sum(nil))).To(Equal(info.SHA256))
			}

			url := getBeskarYUMRPMURL(repositoryName, "repodata/repomd.xml")
			_, err := util.DownloadFromURL(url, 10*time.Second)
			Expect(err).To(BeNil())
		})

		It("Delete Repository Failure", func() {
			err := beskarYUMClient().DeleteRepository(context.Background(), repositoryAPIName, false)
			Expect(err).ToNot(BeNil())
			Expect(gcode.HTTPStatusCode(err)).To(Equal(http.StatusBadRequest))
		})

		It("Remove Mirror Package Failure", func() {
			tag := ""
			for filename := range repo.Files {
				tag = util.GetTagFromFilename(filename)
				break
			}

			err := beskarYUMClient().RemoveRepositoryPackageByTag(context.Background(), repositoryAPIName, tag)
			Expect(gcode.HTTPStatusCode(err)).To(Equal(http.StatusBadRequest))
		})

		It("Check Repository Logs", func() {
			logs, err := beskarYUMClient().ListRepositoryLogs(context.Background(), repositoryAPIName, nil)
			Expect(err).To(BeNil())
			Expect(len(logs)).To(Equal(0))
		})

		It("Delete Repository With Packages", func() {
			err := beskarYUMClient().DeleteRepository(context.Background(), repositoryAPIName, true)
			Expect(err).To(BeNil())

			storageDir := os.Getenv("BESKARYUM_STORAGE_FILESYSTEM_DIRECTORY")
			Expect(storageDir).ToNot(BeEmpty())

			// ensure it's cleaned up
			entries, err := os.ReadDir(filepath.Join(storageDir, repositoryAPIName))
			Expect(err).To(BeNil())
			Expect(len(entries)).To(Equal(0))
		})
	})
})
