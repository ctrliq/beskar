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
	"go.ciq.dev/beskar/pkg/orasfile"
	"go.ciq.dev/beskar/pkg/plugins/httpcodec"
	staticv1 "go.ciq.dev/beskar/pkg/plugins/static/api/v1"
)

func getBeskarStaticURL(path string) string {
	return "http://" + BeskarAddr + "/artifacts/static/" + path
}

func getBeskarStaticFileURL(repository, filename string) string {
	return getBeskarStaticURL(filepath.Join(repository, "file", filename))
}

func beskarStaticClient() *staticv1.HTTPClient {
	httpClient := &http.Client{
		Timeout: 20 * time.Second,
	}

	client, err := staticv1.NewHTTPClient(httpcodec.JSONCodec, httpClient, getBeskarStaticURL("api/v1"))
	Expect(err).To(BeNil())

	return client
}

var _ = Describe("Beskar Static Plugin", func() {
	Describe("Test", Ordered, func() {
		repositoryConfig, err := repoconfig.Parse(repoconfigData)
		Expect(err).To(BeNil())

		repositoryName := "vault-rocky-8.3-ha-debug"

		repo, ok := repositoryConfig[repositoryName]
		Expect(ok).To(BeTrue())

		downloadBaseURL := repo.URL
		repositoryAPIName := "artifacts/static/" + repositoryName

		testFiles := []string{
			"booth-debugsource-1.0-6.ac1d34c.git.el8.2.x86_64.rpm",
			"clufter-bin-debuginfo-0.77.1-5.el8.x86_64.rpm",
		}

		It("File Upload", func() {
			for _, filename := range testFiles {
				rc, err := util.DownloadFromURL(downloadBaseURL+"/"+filename, 10*time.Second)
				Expect(err).To(BeNil())

				pusher, err := orasfile.NewStaticFileStreamPusher(
					rc,
					filename,
					repositoryName,
					name.WithDefaultRegistry(BeskarAddr),
				)
				Expect(err).To(BeNil())

				err = oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
				Expect(err).To(BeNil())
			}
		})

		It("Query File List", func() {
			var files []*staticv1.RepositoryFile

			err := backoff.Retry(func() error {
				files, err = beskarStaticClient().ListRepositoryFiles(context.Background(), repositoryAPIName, nil)
				if err != nil {
					return err
				} else if len(files) != len(testFiles) {
					return fmt.Errorf("expected %d files, got %d", len(testFiles), len(files))
				}
				return nil
			})
			Expect(err).To(BeNil())
		})

		It("Query File By Filename", func() {
			for _, filename := range testFiles {
				info, ok := repo.Files[filename]
				Expect(ok).To(BeTrue())

				file, err := beskarStaticClient().GetRepositoryFileByName(context.Background(), repositoryAPIName, filename)
				Expect(err).To(BeNil())

				Expect(file).ToNot(BeNil())
				Expect(file.Size).To(Equal(info.Size))
				Expect(file.Tag).To(Equal(util.GetTagFromFilename(filename)))
			}
		})

		It("Query File By Tag", func() {
			for _, filename := range testFiles {
				info, ok := repo.Files[filename]
				Expect(ok).To(BeTrue())

				tag := util.GetTagFromFilename(filename)

				file, err := beskarStaticClient().GetRepositoryFileByTag(context.Background(), repositoryAPIName, tag)
				Expect(err).To(BeNil())

				Expect(file).ToNot(BeNil())
				Expect(file.Size).To(Equal(info.Size))
				Expect(file.Tag).To(Equal(tag))
			}
		})

		It("Access File Content", func() {
			for _, filename := range testFiles {
				info, ok := repo.Files[filename]
				Expect(ok).To(BeTrue())

				url := getBeskarStaticFileURL(repositoryName, filename)

				rc, err := util.DownloadFromURL(url, 10*time.Second)
				Expect(err).To(BeNil())

				h := sha256.New()
				n, err := io.Copy(h, rc)

				Expect(err).To(BeNil())
				Expect(uint64(n)).To(Equal(info.Size))
				Expect(hex.EncodeToString(h.Sum(nil))).To(Equal(info.SHA256))
			}
		})

		It("Delete Repository Failure", func() {
			err := beskarStaticClient().DeleteRepository(context.Background(), repositoryAPIName, false)
			Expect(err).ToNot(BeNil())
			Expect(gcode.HTTPStatusCode(err)).To(Equal(http.StatusBadRequest))
		})

		It("Remove File And List", func() {
			tag := util.GetTagFromFilename(testFiles[0])

			err := beskarStaticClient().RemoveRepositoryFile(context.Background(), repositoryAPIName, tag)
			Expect(err).To(BeNil())

			files, err := beskarStaticClient().ListRepositoryFiles(context.Background(), repositoryAPIName, nil)
			Expect(err).To(BeNil())
			Expect(len(files)).To(Equal(len(testFiles) - 1))
		})

		It("Check Repository Logs", func() {
			logs, err := beskarStaticClient().ListRepositoryLogs(context.Background(), repositoryAPIName, nil)
			Expect(err).To(BeNil())
			Expect(len(logs)).To(Equal(0))
		})

		It("Delete Repository With Files", func() {
			err := beskarStaticClient().DeleteRepository(context.Background(), repositoryAPIName, true)
			Expect(err).To(BeNil())

			storageDir := os.Getenv("BESKARSTATIC_STORAGE_FILESYSTEM_DIRECTORY")
			Expect(storageDir).ToNot(BeEmpty())

			// ensure it's cleaned up
			entries, err := os.ReadDir(filepath.Join(storageDir, repositoryAPIName))
			Expect(err).To(BeNil())
			Expect(len(entries)).To(Equal(0))
		})
	})
})
