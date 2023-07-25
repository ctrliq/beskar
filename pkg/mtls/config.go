package mtls

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func loadCerts(caCert, cert, key io.Reader) (*x509.CertPool, []tls.Certificate, error) {
	caCertBytes, err := io.ReadAll(caCert)
	if err != nil {
		return nil, nil, err
	}
	certBytes, err := io.ReadAll(cert)
	if err != nil {
		return nil, nil, err
	}
	keyBytes, err := io.ReadAll(key)
	if err != nil {
		return nil, nil, err
	}

	certs, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertBytes)

	return caCertPool, []tls.Certificate{certs}, nil
}

// ServerConfig returns a mTLS configuration for a server with
// the provided certificate and key.
func ServerConfig(caCert, cert, key io.Reader) (*tls.Config, error) {
	caCertPool, certs, err := loadCerts(caCert, cert, key)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: certs,
		MinVersion:   tls.VersionTLS13,
	}

	return tlsConfig, nil
}

// ClientConfig returns a mTLS configuration for a client with
// the provided certificate and key.
func ClientConfig(caCert, cert, key io.Reader) (*tls.Config, error) {
	caCertPool, certs, err := loadCerts(caCert, cert, key)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: certs,
		MinVersion:   tls.VersionTLS13,
	}

	return tlsConfig, nil
}

func generateConfig(caCertReader, caKeyReader io.ReadSeeker, validity time.Time, serverConfig bool, certOpts ...CertRequestOption) (*tls.Config, error) {
	ca, err := LoadCACertificate(caCertReader, caKeyReader)
	if err != nil {
		return nil, fmt.Errorf("while loading CA certificate and key: %w", err)
	}

	var keyAlg KeyAlg

	switch ca.PrivateKey.(type) {
	case *rsa.PrivateKey:
		keyAlg = RSAKey
	case *ecdsa.PrivateKey:
		keyAlg = ECDSAKey
	default:
		return nil, fmt.Errorf("CA key algorithm not supported: must be RSA or ECDSA")
	}

	cfg := &CertRequestConfig{
		CN:       "foobar", // TODO: should we derive a common name from something else? Like service name?
		Validity: validity,
		IP: []net.IP{
			net.ParseIP("127.0.0.1"),
		},
		DNS: []string{
			"localhost",
		},
		CA:     &ca,
		KeyAlg: keyAlg,
	}

	for _, certOpt := range certOpts {
		certOpt(cfg)
	}

	cert, key, err := GenerateKeyPair(cfg)
	if err != nil {
		return nil, fmt.Errorf("while generate client certificate/key: %w", err)
	} else if _, err := caCertReader.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	if serverConfig {
		return ServerConfig(caCertReader, bytes.NewReader(cert), bytes.NewReader(key))
	}

	return ClientConfig(caCertReader, bytes.NewReader(cert), bytes.NewReader(key))
}

// CertRequestOption represents a certificate request option.
type CertRequestOption func(crc *CertRequestConfig)

// WithCertRequestIPs specifies additional IP address to add with
// the certificate request.
func WithCertRequestIPs(ips ...net.IP) CertRequestOption {
	return func(crc *CertRequestConfig) {
		crc.IP = append(crc.IP, ips...)
	}
}

// WithCertRequestIPs specifies additional hostnames to add with
// the certificate request.
func WithCertRequestHostnames(hostnames ...string) CertRequestOption {
	return func(crc *CertRequestConfig) {
		crc.DNS = append(crc.DNS, hostnames...)
	}
}

// GenerateClientConfigFromFile generates new client mTLS certificate based
// on the provided CA certificate/key for a validity period.
func GenerateClientConfigFromFile(caCertFile, caKeyFile string, validity time.Time, certOpts ...CertRequestOption) (*tls.Config, error) {
	caCert, err := os.Open(caCertFile)
	if err != nil {
		return nil, err
	}
	defer caCert.Close()

	caKey, err := os.Open(caKeyFile)
	if err != nil {
		return nil, err
	}
	defer caKey.Close()

	return generateConfig(caCert, caKey, validity, false, certOpts...)
}

// GenerateClientConfig generates new client mTLS certificate based
// on the provided CA certificate/key for a validity period.
func GenerateClientConfig(caCertReader, caKeyReader io.ReadSeeker, validity time.Time, certOpts ...CertRequestOption) (*tls.Config, error) {
	return generateConfig(caCertReader, caKeyReader, validity, false, certOpts...)
}

// GenerateServerConfigFromFile generates new server mTLS certificate based
// on the provided CA certificate/key for a validity period.
func GenerateServerConfigFromFile(caCertFile, caKeyFile string, validity time.Time, certOpts ...CertRequestOption) (*tls.Config, error) {
	caCert, err := os.Open(caCertFile)
	if err != nil {
		return nil, err
	}
	defer caCert.Close()

	caKey, err := os.Open(caKeyFile)
	if err != nil {
		return nil, err
	}
	defer caKey.Close()

	return generateConfig(caCert, caKey, validity, true, certOpts...)
}

// GenerateServerConfig generates new server mTLS certificate based
// on the provided CA certificate/key for a validity period.
func GenerateServerConfig(caCertReader, caKeyReader io.ReadSeeker, validity time.Time, certOpts ...CertRequestOption) (*tls.Config, error) {
	return generateConfig(caCertReader, caKeyReader, validity, true, certOpts...)
}

// LoadCACertificate loads CA certificate and key
func LoadCACertificate(caCert, caKey io.Reader) (tls.Certificate, error) {
	certBytes, err := io.ReadAll(caCert)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyBytes, err := io.ReadAll(caKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certBytes, keyBytes)
}

// CAPEM defines CA certificate and key in PEM format.
type CAPEM struct {
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}

// MarshalCAPEM encodes a CAPEM instance and returns bytes.
func MarshalCAPEM(cp *CAPEM) ([]byte, error) {
	return json.Marshal(cp)
}

// UnmarshalCAPEM decodes a byte encoded CAPEM and returns an
// instance of it.
func UnmarshalCAPEM(b []byte) (*CAPEM, error) {
	var cp CAPEM

	if err := json.Unmarshal(b, &cp); err != nil {
		return nil, err
	}

	return &cp, nil
}
