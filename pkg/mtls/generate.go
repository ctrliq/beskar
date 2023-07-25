// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mtls

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

type KeyAlg uint8

const (
	RSAKey KeyAlg = iota
	ECDSAKey
)

// CertRequestConfig holds certificate creation configuration.
type CertRequestConfig struct {
	CN       string
	Validity time.Time
	IP       []net.IP
	DNS      []string
	CA       *tls.Certificate
	KeyAlg   KeyAlg
}

// GenerateCA generates a CA certificate pair for a validity period
// with the corresponding key algorithm (RSA or ECDSA).
func GenerateCA(cn string, validity time.Time, keyAlg KeyAlg) ([]byte, []byte, error) {
	cfg := CertRequestConfig{
		CN:       cn,
		Validity: validity,
		KeyAlg:   keyAlg,
	}
	return generateKeyPair(&cfg)
}

// GenerateKeyPair generates a certificate pair for a valid period
// and returns them in PEM byte format. the requested configuration
// must at least provide a CN and a validity period. By default a
// RSA key is generated if KeyAlg is not specified by the request,
// if the CA keys are provided it uses the key algorithm of the CA.
func GenerateKeyPair(cfg *CertRequestConfig) ([]byte, []byte, error) {
	return generateKeyPair(cfg)
}

//nolint:gocyclo
func generateKeyPair(cfg *CertRequestConfig) ([]byte, []byte, error) {
	var certPubKey interface{}
	var certPrivKey interface{}

	isCA := cfg.CA == nil

	if !isCA {
		switch cfg.CA.PrivateKey.(type) {
		case *rsa.PrivateKey:
			cfg.KeyAlg = RSAKey
		case *ecdsa.PrivateKey:
			cfg.KeyAlg = ECDSAKey
		default:
			return nil, nil, fmt.Errorf("CA private key is not a RSA or ECDSA key")
		}
	}

	switch cfg.KeyAlg {
	case RSAKey:
		bits := 2048
		if isCA {
			bits = 4096
		}
		key, err := rsa.GenerateKey(rand.Reader, bits)
		if err != nil {
			return nil, nil, err
		}
		certPrivKey = key
		certPubKey = &key.PublicKey
	case ECDSAKey:
		curve := elliptic.P256()
		if isCA {
			curve = elliptic.P384()
		}
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		certPrivKey = key
		certPubKey = &key.PublicKey
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	keyUsage := x509.KeyUsageDigitalSignature
	if isCA {
		keyUsage |= x509.KeyUsageCertSign
	}

	if cfg.CN == "" {
		return nil, nil, fmt.Errorf("a CN must be provided")
	}
	if cfg.Validity.IsZero() {
		return nil, nil, fmt.Errorf("a validity period must be provided")
	}

	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:    cfg.CN,
			Organization:  []string{"CtrlIQ Inc"},
			Country:       []string{"United States"},
			Province:      []string{"CA"},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		IPAddresses:           cfg.IP,
		DNSNames:              cfg.DNS,
		IsCA:                  isCA,
		NotBefore:             time.Now(),
		NotAfter:              cfg.Validity,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              keyUsage,
		BasicConstraintsValid: isCA,
	}

	caCertPrivKey := certPrivKey
	caCert := cert

	if !isCA {
		caCertPrivKey = cfg.CA.PrivateKey
		caCert, err = x509.ParseCertificate(cfg.CA.Certificate[0])
		if err != nil {
			return nil, nil, err
		}
	}

	certBuf := new(bytes.Buffer)
	keyBuf := new(bytes.Buffer)

	certBlock := &pem.Block{Type: "CERTIFICATE"}
	privBlock := &pem.Block{}

	certBlock.Bytes, err = x509.CreateCertificate(rand.Reader, cert, caCert, certPubKey, caCertPrivKey)
	if err != nil {
		return nil, nil, err
	}

	err = pem.Encode(certBuf, certBlock)
	if err != nil {
		return nil, nil, err
	}

	switch cfg.KeyAlg {
	case RSAKey:
		privBlock.Type = "RSA PRIVATE KEY"
		privBlock.Bytes = x509.MarshalPKCS1PrivateKey(certPrivKey.(*rsa.PrivateKey))
	case ECDSAKey:
		privBlock.Type = "EC PRIVATE KEY"
		privBlock.Bytes, err = x509.MarshalECPrivateKey(certPrivKey.(*ecdsa.PrivateKey))
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("unknown key algorithm %d", cfg.KeyAlg)
	}

	err = pem.Encode(keyBuf, privBlock)
	if err != nil {
		return nil, nil, err
	}

	return certBuf.Bytes(), keyBuf.Bytes(), nil
}
