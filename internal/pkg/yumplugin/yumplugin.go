package yumplugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/gorilla/mux"
	"go.ciq.dev/beskar/internal/pkg/config"
	"go.ciq.dev/beskar/internal/pkg/s3"
	"go.ciq.dev/beskar/internal/pkg/yumplugin/pkg/orasrpm"
	"go.ciq.dev/beskar/internal/pkg/yumplugin/pkg/yumdb"
	"go.ciq.dev/beskar/pkg/oras"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

type Plugin struct {
	registry        string
	bucketName      string
	dataDir         string
	authMethod      *s3.AuthMethod
	manifests       []*v1.Manifest
	manifestMutex   sync.Mutex
	queued          chan struct{}
	server          http.Server
	remoteOptions   []remote.Option
	bucket          *blob.Bucket
	beskarYumConfig *config.BeskarYumConfig
}

func New(ctx context.Context, beskarYumConfig *config.BeskarYumConfig, server bool) (*Plugin, error) {
	registryURL, err := url.Parse(beskarYumConfig.Registry.URL)
	if err != nil {
		return nil, err
	}

	authMethod, err := s3.NewAuthMethod(
		beskarYumConfig.S3.Endpoint,
		s3.WithCredentials(
			beskarYumConfig.S3.AccessKeyID,
			beskarYumConfig.S3.SecretAccessKey,
			beskarYumConfig.S3.SessionToken,
		),
		s3.WithRegion(beskarYumConfig.S3.Region),
		s3.WithDisableSSL(beskarYumConfig.S3.DisableSSL),
	)
	if err != nil {
		return nil, err
	}

	bucket, err := s3blob.OpenBucket(ctx, authMethod.Session(), beskarYumConfig.S3.Bucket, nil)
	if err != nil {
		return nil, err
	}

	if beskarYumConfig.DataDir == "" {
		beskarYumConfig.DataDir = config.DefaultBeskarYumDataDir
	}

	plugin := &Plugin{
		registry:        registryURL.Host,
		manifests:       make([]*v1.Manifest, 0, 32),
		queued:          make(chan struct{}, 1),
		bucketName:      beskarYumConfig.S3.Bucket,
		authMethod:      authMethod,
		dataDir:         beskarYumConfig.DataDir,
		bucket:          bucket,
		beskarYumConfig: beskarYumConfig,
		remoteOptions: []remote.Option{
			oras.AuthConfig(beskarYumConfig.Registry.Username, beskarYumConfig.Registry.Password),
		},
	}

	if err := os.MkdirAll(plugin.dataDir, 0o700); err != nil {
		return nil, err
	}

	if server {
		router := mux.NewRouter()
		router.HandleFunc("/event", plugin.eventHandler())
		router.HandleFunc("/yum/repo/{repository}/repodata/repomd.xml", repomdHandler(registryURL.Host))
		router.HandleFunc("/yum/repo/{repository}/repodata/{digest}-{file}", blobsHandler("repodata"))
		router.HandleFunc("/yum/repo/{repository}/packages/{digest}/{file}", blobsHandler("packages"))

		if beskarYumConfig.Profiling {
			plugin.setProfiling(router)
		}

		plugin.server = http.Server{
			Handler:           router,
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       20 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
		}

		go plugin.dequeue(ctx)
	}

	return plugin, nil
}

func (p *Plugin) setProfiling(router *mux.Router) {
	router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	router.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	router.Handle("/debug/pprof/{cmd}", http.HandlerFunc(pprof.Index)) // special handling for Gorilla mux
}

func (p *Plugin) Serve(ln net.Listener) error {
	return p.server.Serve(ln)
}

func (p *Plugin) dequeue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.queued:
			p.manifestMutex.Lock()
			length := len(p.manifests)
			if length > 20 {
				length = 20
			}
			manifests := make([]*v1.Manifest, length)
			copy(manifests, p.manifests)
			p.manifests = p.manifests[length:]
			if len(p.manifests) > 0 {
				p.enqueueNotify()
			}
			p.manifestMutex.Unlock()
			p.processPackages(ctx, manifests)
		}
	}
}

func (p *Plugin) enqueueNotify() {
	select {
	case p.queued <- struct{}{}:
	default:
	}
}

func (p *Plugin) enqueue(manifest *v1.Manifest) {
	p.manifestMutex.Lock()
	p.manifests = append(p.manifests, manifest)
	p.manifestMutex.Unlock()
	p.enqueueNotify()
}

func (p *Plugin) processPackages(ctx context.Context, manifests []*v1.Manifest) {
	repos := make(map[string]string)

	for idx, manifest := range manifests {
		if repository, dbDir, err := p.processPackage(ctx, manifest, idx == len(manifests)-1); err != nil {
			fmt.Printf("------ ERROR: %s", err)
		} else {
			repos[repository] = dbDir
		}
	}

	for repo, dbDir := range repos {
		fmt.Println("------ PROCESS METADATA", repo, time.Now())
		err := p.GenerateAndSaveMetadata(ctx, filepath.Dir(repo), dbDir, true)
		fmt.Println("------ PROCESS METADATA END", repo, time.Now())
		if err != nil {
			fmt.Printf("ERROR: %s", err)
		}
	}
}

func (p *Plugin) processPackage(ctx context.Context, manifest *v1.Manifest, keepDatabaseDir bool) (string, string, error) {
	layerIndex := -1

	for i, layer := range manifest.Layers {
		if layer.MediaType != orasrpm.RPMPackageLayerType {
			continue
		}
		layerIndex = i
		break
	}

	if layerIndex < 0 {
		return "", "", fmt.Errorf("no RPM package layer found in manifest")
	}

	packageLayer := manifest.Layers[layerIndex]

	packageFilename := packageLayer.Annotations["org.opencontainers.image.title"]

	fmt.Println("------ PROCESS PACKAGE", packageFilename, manifest.Layers[0].Digest.Hex, time.Now())
	defer func() {
		fmt.Println("------ PROCESS PACKAGE END", packageFilename, manifest.Layers[0].Digest.Hex, time.Now())
	}()

	repository := manifest.Annotations["repository"]
	ref := filepath.Join(p.registry, repository+"@sha256:"+packageLayer.Digest.Hex)
	repoDir := filepath.Join(p.dataDir, repository)

	tmpDir, err := os.MkdirTemp(p.dataDir, "package-")
	if err != nil {
		return "", "", fmt.Errorf("while creating temporary package directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	packageFile := filepath.Join(tmpDir, packageFilename)

	if err := downloadPackage(ref, packageFile); err != nil {
		return "", "", err
	}

	href := fmt.Sprintf("packages/%s/%s", "sha256:"+packageLayer.Digest.Hex, packageFilename)
	packageDir, err := extractPackageMetadata(tmpDir, repoDir, packageFilename, href)
	if err != nil {
		return "", "", err
	}

	dbDir, err := p.AddPackageToDatabase(ctx, manifest.Layers[0].Digest.Hex, repository, packageDir, true, keepDatabaseDir)
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(packageDir)

	return repository, dbDir, err
}

func (p *Plugin) GenerateAndSaveMetadata(ctx context.Context, repository, dbDir string, execute bool) error {
	if execute {
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		args := []string{
			"gen-metadata",
			fmt.Sprintf("-config-dir=%s", p.beskarYumConfig.ConfigDirectory),
			fmt.Sprintf("-db-dir=%s", dbDir),
			fmt.Sprintf("-repository=%s", repository),
		}

		//nolint:gosec // internal use only
		cmd := exec.CommandContext(ctx, os.Args[0], args...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		return cmd.Run()
	}

	defer func() {
		_ = os.RemoveAll(dbDir)
	}()

	repodataDir, err := os.MkdirTemp(p.dataDir, "repodata-")
	if err != nil {
		return fmt.Errorf("while creating temporary package directory: %w", err)
	}
	defer os.RemoveAll(repodataDir)

	outputDir := filepath.Join(repodataDir, "repodata")
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		return err
	}

	db, err := yumdb.Open(dbDir)
	if err != nil {
		return err
	}

	packageCount, err := db.CountPackages(ctx)
	if err != nil {
		return err
	}

	repomd, err := newRepoMetadata(outputDir, p.registry, filepath.Join(repository, "repodata"), packageCount)
	if err != nil {
		return err
	}

	err = db.WalkPackages(ctx, func(pkg *yumdb.Package) error {
		if err := repomd.Add(bytes.NewReader(pkg.Primary), primaryXMLFile); err != nil {
			return fmt.Errorf("while adding %s: %w", primaryXMLFile, err)
		}
		if err := repomd.Add(bytes.NewReader(pkg.Filelists), filelistsXMLFile); err != nil {
			return fmt.Errorf("while adding %s: %w", filelistsXMLFile, err)
		}
		if err := repomd.Add(bytes.NewReader(pkg.Other), otherXMLFile); err != nil {
			return fmt.Errorf("while adding %s: %w", otherXMLFile, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return repomd.Save(p.remoteOptions...)
}

func downloadPackage(ref string, destinationPath string) (errFn error) {
	dst, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer func() {
		err = dst.Close()
		if errFn == nil {
			errFn = err
		}
	}()

	digest, err := name.NewDigest(ref)
	if err != nil {
		return err
	}
	layer, err := remote.Layer(digest)
	if err != nil {
		return err
	}
	rc, err := layer.Compressed()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(dst, rc)
	return err
}
