package haloyd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestCertificatesManager creates a manager backed by a temp cert dir. The
// challenge server binds an ephemeral port so tests can run in parallel.
func newTestCertificatesManager(t *testing.T) *CertificatesManager {
	t.Helper()

	m, err := NewCertificatesManager(CertificatesManagerConfig{
		CertDir:          t.TempDir(),
		HTTPProviderPort: "0",
		TlsStaging:       false,
	}, nil)
	if err != nil {
		t.Fatalf("NewCertificatesManager() error = %v", err)
	}
	t.Cleanup(m.Stop)
	return m
}

// TestCheckRenewalsContinuesPastFailedDomain verifies that one domain failing
// to obtain a certificate does not abort renewal attempts for the remaining
// domains, and that all failures are reported. The .invalid TLD is reserved
// and never resolves, so domain validation fails before any ACME contact.
func TestCheckRenewalsContinuesPastFailedDomain(t *testing.T) {
	m := newTestCertificatesManager(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	domains := []CertificatesDomain{
		{Canonical: "haloy-test-a.invalid"},
		{Canonical: "haloy-test-b.invalid"},
	}

	renewed, err := m.checkRenewals(logger, domains)
	if err == nil {
		t.Fatal("checkRenewals() expected error for unresolvable domains, got nil")
	}
	if len(renewed) != 0 {
		t.Fatalf("checkRenewals() renewed = %v, want none", renewed)
	}
	for _, domain := range domains {
		if !strings.Contains(err.Error(), domain.Canonical) {
			t.Errorf("checkRenewals() error should mention %s, got: %v", domain.Canonical, err)
		}
	}
}

// TestCheckRenewalsKeepsCertOnFailedObtain verifies that when a domain's
// configuration changes (e.g. new alias) and obtaining the replacement
// certificate fails, the existing certificate is left on disk so the proxy
// can keep serving it.
func TestCheckRenewalsKeepsCertOnFailedObtain(t *testing.T) {
	m := newTestCertificatesManager(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	const canonical = "haloy-test-a.invalid"
	certPath := writeCombinedTestCert(t, m.config.CertDir, canonical)

	// Adding an alias changes the required SAN set, forcing a re-obtain.
	renewed, err := m.checkRenewals(logger, []CertificatesDomain{
		{Canonical: canonical, Aliases: []string{"www." + canonical}},
	})
	if err == nil {
		t.Fatal("checkRenewals() expected error for unresolvable domain, got nil")
	}
	if len(renewed) != 0 {
		t.Fatalf("checkRenewals() renewed = %v, want none", renewed)
	}
	if _, statErr := os.Stat(certPath); statErr != nil {
		t.Fatalf("existing certificate should survive a failed obtain: %v", statErr)
	}
}

// writeCombinedTestCert writes a combined key+certificate PEM file for domain
// in the layout the certificate manager uses, and returns its path.
func writeCombinedTestCert(t *testing.T, dir, domain string) string {
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
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{domain},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey() error = %v", err)
	}

	pemData := append(
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})...,
	)

	certPath := filepath.Join(dir, domain+combinedCertExt)
	if err := os.WriteFile(certPath, pemData, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return certPath
}
