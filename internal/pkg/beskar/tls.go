//nolint:goheader

// code copied from https://github.com/distribution/distribution/blob/main/registry/registry.go
// and modified under Apache-2.0 license
package beskar

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// a map of TLS cipher suite names to constants in https://golang.org/pkg/crypto/tls/#pkg-constants
var cipherSuites = map[string]uint16{
	// TLS 1.0 - 1.2 cipher suites
	"TLS_RSA_WITH_3DES_EDE_CBC_SHA":                 tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	"TLS_RSA_WITH_AES_128_CBC_SHA":                  tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	"TLS_RSA_WITH_AES_256_CBC_SHA":                  tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	"TLS_RSA_WITH_AES_128_GCM_SHA256":               tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_RSA_WITH_AES_256_GCM_SHA384":               tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":          tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":          tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":           tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":            tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":            tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":       tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":       tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	// TLS 1.3 cipher suites
	"TLS_AES_128_GCM_SHA256":       tls.TLS_AES_128_GCM_SHA256,
	"TLS_AES_256_GCM_SHA384":       tls.TLS_AES_256_GCM_SHA384,
	"TLS_CHACHA20_POLY1305_SHA256": tls.TLS_CHACHA20_POLY1305_SHA256,
}

// a list of default ciphersuites to utilize
var defaultCipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_AES_128_GCM_SHA256,
	tls.TLS_CHACHA20_POLY1305_SHA256,
	tls.TLS_AES_256_GCM_SHA384,
}

const defaultTLSVersionStr = "tls1.2"

// tlsVersions maps user-specified values to tls version constants.
var tlsVersions = map[string]uint16{
	"tls1.2": tls.VersionTLS12,
	"tls1.3": tls.VersionTLS13,
}

// takes a list of cipher suites and converts it to a list of respective tls constants
// if an empty list is provided, then the defaults will be used
func getCipherSuites(names []string) ([]uint16, error) {
	if len(names) == 0 {
		return defaultCipherSuites, nil
	}
	cipherSuiteConsts := make([]uint16, len(names))
	for i, name := range names {
		cipherSuiteConst, ok := cipherSuites[name]
		if !ok {
			return nil, fmt.Errorf("unknown TLS cipher suite '%s' specified for http.tls.cipherSuites", name)
		}
		cipherSuiteConsts[i] = cipherSuiteConst
	}
	return cipherSuiteConsts, nil
}

// takes a list of cipher suite ids and converts it to a list of respective names
func getCipherSuiteNames(ids []uint16) []string {
	if len(ids) == 0 {
		return nil
	}
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = tls.CipherSuiteName(id)
	}
	return names
}

// set ACME-server/DirectoryURL, if provided
func setDirectoryURL(directoryurl string) *acme.Client {
	if len(directoryurl) > 0 {
		return &acme.Client{DirectoryURL: directoryurl}
	}
	return nil
}

func nextProtos(config *configuration.Configuration) []string {
	switch config.HTTP.HTTP2.Disabled {
	case true:
		return []string{"http/1.1"}
	default:
		return []string{"h2", "http/1.1"}
	}
}

// serve runs the registry's HTTP server.
func (br *Registry) getTLSConfig(logger *logrus.Entry) (*tls.Config, error) {
	var err error

	config := br.beskarConfig.Registry

	if config.HTTP.TLS.Certificate == "" && config.HTTP.TLS.LetsEncrypt.CacheFile == "" {
		return nil, nil
	}

	if config.HTTP.TLS.MinimumTLS == "" {
		config.HTTP.TLS.MinimumTLS = defaultTLSVersionStr
	}
	tlsMinVersion, ok := tlsVersions[config.HTTP.TLS.MinimumTLS]
	if !ok {
		return nil, fmt.Errorf("unknown minimum TLS level '%s' specified for http.tls.minimumtls", config.HTTP.TLS.MinimumTLS)
	}
	logger.Infof("restricting TLS version to %s or higher", config.HTTP.TLS.MinimumTLS)

	var tlsCipherSuites []uint16
	// configuring cipher suites are no longer supported after the tls1.3.
	// (https://go.dev/blog/tls-cipher-suites)
	if tlsMinVersion > tls.VersionTLS12 {
		logger.Warnf("restricting TLS cipher suites to empty. Because configuring cipher suites is no longer supported in %s", config.HTTP.TLS.MinimumTLS)
	} else {
		tlsCipherSuites, err = getCipherSuites(config.HTTP.TLS.CipherSuites)
		if err != nil {
			return nil, err
		}
		logger.Infof("restricting TLS cipher suites to: %s", strings.Join(getCipherSuiteNames(tlsCipherSuites), ","))
	}

	//nolint:gosec
	tlsConf := &tls.Config{
		ClientAuth:   tls.NoClientCert,
		NextProtos:   nextProtos(config),
		MinVersion:   tlsMinVersion,
		CipherSuites: tlsCipherSuites,
	}

	if config.HTTP.TLS.LetsEncrypt.CacheFile != "" {
		if config.HTTP.TLS.Certificate != "" {
			return nil, fmt.Errorf("cannot specify both certificate and Let's Encrypt")
		}
		m := &autocert.Manager{
			HostPolicy: autocert.HostWhitelist(config.HTTP.TLS.LetsEncrypt.Hosts...),
			Cache:      autocert.DirCache(config.HTTP.TLS.LetsEncrypt.CacheFile),
			Email:      config.HTTP.TLS.LetsEncrypt.Email,
			Prompt:     autocert.AcceptTOS,
			Client:     setDirectoryURL(config.HTTP.TLS.LetsEncrypt.DirectoryURL),
		}
		tlsConf.GetCertificate = m.GetCertificate
		tlsConf.NextProtos = append(tlsConf.NextProtos, acme.ALPNProto)
	} else {
		tlsConf.Certificates = make([]tls.Certificate, 1)
		tlsConf.Certificates[0], err = tls.LoadX509KeyPair(config.HTTP.TLS.Certificate, config.HTTP.TLS.Key)
		if err != nil {
			return nil, err
		}
	}

	if len(config.HTTP.TLS.ClientCAs) != 0 {
		pool := x509.NewCertPool()

		for _, ca := range config.HTTP.TLS.ClientCAs {
			caPem, err := os.ReadFile(ca)
			if err != nil {
				return nil, err
			}

			if ok := pool.AppendCertsFromPEM(caPem); !ok {
				return nil, fmt.Errorf("could not add CA to pool")
			}
		}

		tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConf.ClientCAs = pool
	}

	return tlsConf, nil
}
