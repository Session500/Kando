package phonesync

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// The files Kando keeps its phone sync certificate and its key in, in its config folder, readable by the user alone.
const (
	CertFile = "phone-sync-cert.pem"
	KeyFile  = "phone-sync-key.pem"
)

// certLifetime is how long the certificate is good for. Phones pin it by its fingerprint, so a new one would unpair every phone; it is made once and kept.
const certLifetime = 20 * 365 * 24 * time.Hour

// loadCert reads the certificate and key in dir, or makes them when neither is there: an ECDSA P-256 key and a certificate it signs itself. A pair of which one file is missing or cannot be read is an error rather than replaced, as a new certificate would unpair every phone without saying why.
func loadCert(dir string) (tls.Certificate, error) {
	certPath, keyPath := filepath.Join(dir, CertFile), filepath.Join(dir, KeyFile)
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err == nil {
		return cert, nil
	}
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)
	if !errors.Is(certErr, os.ErrNotExist) || !errors.Is(keyErr, os.ErrNotExist) {
		return tls.Certificate{}, fmt.Errorf("read the phone sync certificate: %w", err)
	}
	certPEM, keyPEM, err := makeCert(time.Now())
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, err
	}
	if err := writePrivate(keyPath, keyPEM); err != nil {
		return tls.Certificate{}, err
	}
	if err := writePrivate(certPath, certPEM); err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

// makeCert makes a key and a certificate it signs itself, valid from an hour before now, as PEM.
func makeCert(now time.Time) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Kando phone sync"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(certLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), nil
}

// writePrivate writes data to path through a file beside it, readable and writable by the user alone: os.CreateTemp makes the file 0600 on Unix, and on Windows the user's config folder is the user's own.
func writePrivate(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
