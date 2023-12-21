package integration_test

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/crane"
	"go.ciq.dev/beskar/integration/pkg/backoff"
	"go.ciq.dev/beskar/integration/pkg/repoconfig"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	BeskarAddr     = "127.0.0.1:5100"
	BeskarUsername = "beskar"
	BeskarPassword = "beskar"

	SwaggerYAMLPath = "api/v1/doc/swagger.yaml"

	integrationDir = "/tmp/integration"
)

//go:embed testdata/repoconfig.yaml
var repoconfigData []byte

func TestSubstrateIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Beskar Integration Test Suite")
}

var _ = SynchronizedBeforeSuite(
	// NOTE: This runs *only* on process #1 when run in parallel
	func() []byte {
		err := os.RemoveAll(integrationDir)
		Expect(err).To(BeNil())

		err = os.Mkdir(integrationDir, 0o700)
		Expect(err).To(BeNil())

		repositoryConfig, err := repoconfig.Parse(repoconfigData)
		Expect(err).To(BeNil())

		mux := http.NewServeMux()
		server := &http.Server{
			Addr:    "127.0.0.1:8080",
			Handler: mux,
		}

		for name, repo := range repositoryConfig {
			if repo.LocalPath != "" {
				fs := http.FileServer(http.Dir(repo.LocalPath))
				pathPrefix := fmt.Sprintf("/%s/", name)
				mux.Handle(pathPrefix, http.StripPrefix(pathPrefix, fs))
			}
		}

		go server.ListenAndServe()

		f, err := os.Create("/tmp/logs")
		Expect(err).To(BeNil())

		beskarCmd := exec.Command("/app/beskar")
		beskarCmd.Stdout = f
		beskarCmd.Stderr = f

		err = beskarCmd.Start()
		Expect(err).To(BeNil())

		err = storeBeskarCredentials()
		Expect(err).To(BeNil())

		err = checkBeskar()
		Expect(err).To(BeNil())

		err = checkBeskarYUMAPI()
		Expect(err).To(BeNil())

		err = checkBeskarStaticAPI()
		Expect(err).To(BeNil())

		DeferCleanup(func() {
			defer os.RemoveAll(integrationDir)

			err := beskarCmd.Process.Signal(syscall.SIGINT)
			Expect(err).To(BeNil())

			err = beskarCmd.Wait()
			Expect(err).To(BeNil())
		})

		return nil
	},
	// NOTE: This runs on *all* processes when run in parallel
	func(_ []byte) {
		time.Sleep(1 * time.Second)
	},
)

func storeBeskarCredentials() error {
	cf, err := config.Load("")
	if err != nil {
		return err
	}
	creds := cf.GetCredentialsStore(BeskarAddr)

	if err := creds.Store(types.AuthConfig{
		ServerAddress: BeskarAddr,
		Username:      BeskarUsername,
		Password:      BeskarPassword,
	}); err != nil {
		return err
	}

	return cf.Save()
}

func checkBeskar() error {
	return backoff.Retry(func() error {
		_, err := crane.Catalog(BeskarAddr, crane.Insecure)
		return err
	})
}

func checkBeskarYUMAPI() error {
	httpClient := &http.Client{
		Timeout: time.Second,
	}

	req, err := http.NewRequest("GET", getBeskarYUMURL(SwaggerYAMLPath), nil)
	if err != nil {
		return err
	}

	return backoff.Retry(func() error {
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status code: %d", resp.StatusCode)
		}

		return nil
	})
}

func checkBeskarStaticAPI() error {
	httpClient := &http.Client{
		Timeout: time.Second,
	}

	req, err := http.NewRequest("GET", getBeskarStaticURL(SwaggerYAMLPath), nil)
	if err != nil {
		return err
	}

	return backoff.Retry(func() error {
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status code: %d", resp.StatusCode)
		}

		return nil
	})
}
