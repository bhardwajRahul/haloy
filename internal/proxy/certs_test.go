package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCertManagerExactMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestCert(t, dir, "example.com")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cm, err := NewCertManager(dir, logger)
	if err != nil {
		t.Fatalf("NewCertManager() error = %v", err)
	}

	if _, err := cm.GetCertificate(&tls.ClientHelloInfo{ServerName: "example.com"}); err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}
}

func TestCertManagerAliasMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestCert(t, dir, "example.com")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cm, err := NewCertManager(dir, logger)
	if err != nil {
		t.Fatalf("NewCertManager() error = %v", err)
	}

	rb := NewRouteBuilder()
	rb.AddRoute("example.com", []string{"alias.example.com"}, nil)
	config, err := rb.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	cm.SetRouteTable(config)

	cert, err := cm.GetCertificate(&tls.ClientHelloInfo{ServerName: "alias.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}
	if cert == cm.defaultCert {
		t.Fatal("GetCertificate() for alias returned the default cert, want the canonical domain's cert")
	}
}

func TestCertManagerRejectsMaliciousSNI(t *testing.T) {
	baseDir := t.TempDir()
	certDir := filepath.Join(baseDir, "certs")
	if err := os.Mkdir(certDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// A valid combined cert outside the certificate directory: without SNI
	// validation, a traversal payload would load and serve it.
	writeTestCert(t, baseDir, "evil")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cm, err := NewCertManager(certDir, logger)
	if err != nil {
		t.Fatalf("NewCertManager() error = %v", err)
	}

	payloads := []string{
		"../evil",
		"..%2fevil",
		"foo/../../evil",
		"foo\\evil",
		"..",
		".",
		"",
		"foo..bar.example.com",
	}

	for _, payload := range payloads {
		cert, err := cm.GetCertificate(&tls.ClientHelloInfo{ServerName: payload})
		if err != nil {
			t.Fatalf("GetCertificate(%q) error = %v", payload, err)
		}
		if cert != cm.defaultCert {
			t.Errorf("GetCertificate(%q) escaped the cert directory, want default cert", payload)
		}
	}
}

func TestCertManagerSkipsDiskForUnknownDomains(t *testing.T) {
	dir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cm, err := NewCertManager(dir, logger)
	if err != nil {
		t.Fatalf("NewCertManager() error = %v", err)
	}

	rb := NewRouteBuilder()
	rb.AddRoute("routed.example.com", nil, nil)
	config, err := rb.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	cm.SetRouteTable(config)

	// Certs written after startup (not yet cached): only the routed domain
	// may be loaded from disk on demand.
	writeTestCert(t, dir, "routed.example.com")
	writeTestCert(t, dir, "unrouted.example.com")

	cert, err := cm.GetCertificate(&tls.ClientHelloInfo{ServerName: "unrouted.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}
	if cert != cm.defaultCert {
		t.Error("GetCertificate() for unrouted domain hit the disk, want default cert")
	}

	cert, err = cm.GetCertificate(&tls.ClientHelloInfo{ServerName: "routed.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}
	if cert == cm.defaultCert {
		t.Error("GetCertificate() for routed domain returned default cert, want cert loaded from disk")
	}
}

func TestCertManagerDoesNotServePreloadedUnknownCertificate(t *testing.T) {
	dir := t.TempDir()
	writeTestCert(t, dir, "routed.example.com")
	writeTestCert(t, dir, "stale.example.com")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cm, err := NewCertManager(dir, logger)
	if err != nil {
		t.Fatalf("NewCertManager() error = %v", err)
	}

	rb := NewRouteBuilder()
	rb.AddRoute("routed.example.com", nil, nil)
	config, err := rb.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	cm.SetRouteTable(config)

	cert, err := cm.GetCertificate(&tls.ClientHelloInfo{ServerName: "stale.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}
	if cert != cm.defaultCert {
		t.Error("GetCertificate() for preloaded unknown domain returned a cached cert, want default cert")
	}

	cert, err = cm.GetCertificate(&tls.ClientHelloInfo{ServerName: "routed.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}
	if cert == cm.defaultCert {
		t.Error("GetCertificate() for routed domain returned default cert, want cached cert")
	}
}

func TestCertManagerWildcardMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestCert(t, dir, "*.example.com")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cm, err := NewCertManager(dir, logger)
	if err != nil {
		t.Fatalf("NewCertManager() error = %v", err)
	}

	// Single-level subdomain should match wildcard cert
	wildcardCert, err := cm.GetCertificate(&tls.ClientHelloInfo{ServerName: "app.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}

	// Multi-level subdomain should NOT match wildcard cert (returns default cert instead)
	multiLevelCert, err := cm.GetCertificate(&tls.ClientHelloInfo{ServerName: "app.dev.example.com"})
	if err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}

	// Verify they're different certs (multi-level gets default, not wildcard)
	if wildcardCert == multiLevelCert {
		t.Fatal("GetCertificate() multi-level subdomain should not match wildcard cert")
	}
}

func writeTestCert(t *testing.T, dir, domain string) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	serialNumber, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("rand.Int() error = %v", err)
	}

	certTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames: []string{domain},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey() error = %v", err)
	}

	certPath := filepath.Join(dir, domain+".pem")
	pemData := append(
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})...,
	)

	if err := writeFile(certPath, pemData); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}
}

func writeFile(path string, contents []byte) error {
	return os.WriteFile(path, contents, 0o600)
}
