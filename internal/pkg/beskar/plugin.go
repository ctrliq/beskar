package beskar

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	"google.golang.org/protobuf/proto"
)

type proxyPlugin struct {
	url *url.URL
}

func (pp proxyPlugin) send(ctx context.Context, repository string, mediaType string, payload []byte, dgst string) error {
	event := &eventv1.ManifestEvent{
		Digest:     dgst,
		Mediatype:  mediaType,
		Payload:    payload,
		Repository: repository,
	}

	data, err := proto.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, pp.url.String(), bytes.NewReader(data))
	if err != nil {
		log.Fatalf("%v", err)
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/octet-stream")

	client := http.DefaultClient

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("plugin backend has returned an unknown status %d", resp.StatusCode)
	}

	return nil
}

func initPlugins(ctx context.Context, registry *Registry) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	execPath := filepath.Dir(self)

	for _, plugin := range registry.beskarConfig.Plugins {
		if len(plugin.Backends) != 1 {
			return fmt.Errorf("only backend supported for now")
		}

		pluginURL, err := url.Parse(plugin.Backends[0].URL)
		if err != nil {
			return fmt.Errorf("while parsing plugin URL %s: %w", plugin.Backends[0].URL, err)
		}

		executable := pluginURL.Query().Get("executable")
		if executable != "" {
			executable := filepath.Join(execPath, executable)
			cmd := exec.CommandContext(ctx, executable, os.Args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				return err
			}
			go func() {
				_ = cmd.Wait()
			}()
		}

		pluginURL.RawQuery = ""

		proxy := httputil.NewSingleHostReverseProxy(pluginURL)
		registry.router.PathPrefix(plugin.Prefix).Handler(proxy)

		purl := *pluginURL
		purl.Path = "/event"

		registry.proxyPlugins[plugin.Mediatype] = &proxyPlugin{
			url: &purl,
		}
	}

	return nil
}
