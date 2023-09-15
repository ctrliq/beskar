package mirror

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncer(t *testing.T) {
	dir, err := os.MkdirTemp("", "testing-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	m1, err := url.Parse("https://download.rockylinux2.org/pub/rocky/8/BaseOS/x86_64/os")
	require.NoError(t, err)

	m2, err := url.Parse("https://download.rockylinux.org/pub/rocky/8/BaseOS/x86_64/os")
	require.NoError(t, err)

	s := NewSyncer(dir, []*url.URL{m1, m2})
	packages, totalPackages := s.DownloadPackages(context.Background(), func(id string) bool {
		return true
	})

	packageCount := 0

	for range packages {
		packageCount++
	}

	require.NoError(t, s.Err())
	require.Equal(t, totalPackages, packageCount)

	_ = s.DownloadExtraMetadata(context.Background(), func(dataType string, checksum string) bool {
		return true
	})

	require.NoError(t, s.Err())
}
