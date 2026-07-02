package haloydcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haloydev/haloy/internal/config"
	"github.com/haloydev/haloy/internal/constants"
)

func TestConfigSetAPIDomainNormalizesPreservesConfigAndRemovesOldCert(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv(constants.EnvVarConfigDir, configDir)
	t.Setenv(constants.EnvVarDataDir, dataDir)

	enabled := false
	existing := &config.HaloydConfig{
		API: config.HaloydAPIConfig{Domain: "old.example.com"},
		HealthMonitor: config.HealthMonitorConfig{
			Enabled:  &enabled,
			Interval: "30s",
			Fall:     5,
			Rise:     4,
			Timeout:  "7s",
		},
	}
	if err := config.SaveHaloydConfig(existing, filepath.Join(configDir, constants.HaloydConfigFileName)); err != nil {
		t.Fatalf("SaveHaloydConfig() error = %v", err)
	}

	certDir := filepath.Join(dataDir, constants.CertStorageDir)
	if err := os.MkdirAll(certDir, constants.ModeDirPrivate); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	oldCertPath := filepath.Join(certDir, "old.example.com.pem")
	if err := os.WriteFile(oldCertPath, []byte("old cert"), constants.ModeFileSecret); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := configSetCmd()
	cmd.SetArgs([]string{"api-domain", "API.EXAMPLE.COM"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set error = %v", err)
	}

	loaded, err := config.LoadHaloydConfig(filepath.Join(configDir, constants.HaloydConfigFileName))
	if err != nil {
		t.Fatalf("LoadHaloydConfig() error = %v", err)
	}
	if loaded.API.Domain != "api.example.com" {
		t.Fatalf("API.Domain = %q, want %q", loaded.API.Domain, "api.example.com")
	}
	if loaded.HealthMonitor.Enabled == nil || *loaded.HealthMonitor.Enabled != enabled ||
		loaded.HealthMonitor.Interval != "30s" ||
		loaded.HealthMonitor.Fall != 5 ||
		loaded.HealthMonitor.Rise != 4 ||
		loaded.HealthMonitor.Timeout != "7s" {
		t.Fatalf("HealthMonitor was not preserved: %+v", loaded.HealthMonitor)
	}
	if _, err := os.Stat(oldCertPath); !os.IsNotExist(err) {
		t.Fatalf("old cert still exists or stat failed unexpectedly: %v", err)
	}
}

func TestConfigSetAPIDomainSameDomainKeepsCertificate(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv(constants.EnvVarConfigDir, configDir)
	t.Setenv(constants.EnvVarDataDir, dataDir)

	existing := &config.HaloydConfig{
		API: config.HaloydAPIConfig{Domain: "api.example.com"},
	}
	if err := config.SaveHaloydConfig(existing, filepath.Join(configDir, constants.HaloydConfigFileName)); err != nil {
		t.Fatalf("SaveHaloydConfig() error = %v", err)
	}

	certDir := filepath.Join(dataDir, constants.CertStorageDir)
	if err := os.MkdirAll(certDir, constants.ModeDirPrivate); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	certPath := filepath.Join(certDir, "api.example.com.pem")
	if err := os.WriteFile(certPath, []byte("cert"), constants.ModeFileSecret); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := configSetCmd()
	cmd.SetArgs([]string{"api-domain", "API.EXAMPLE.COM"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set error = %v", err)
	}

	if _, err := os.Stat(certPath); err != nil {
		t.Fatalf("same-domain cert should remain: %v", err)
	}
}

func TestConfigSetAPIDomainRemovesLegacyUppercaseCertificate(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv(constants.EnvVarConfigDir, configDir)
	t.Setenv(constants.EnvVarDataDir, dataDir)

	existing := &config.HaloydConfig{
		API: config.HaloydAPIConfig{Domain: "API.EXAMPLE.COM"},
	}
	if err := config.SaveHaloydConfig(existing, filepath.Join(configDir, constants.HaloydConfigFileName)); err != nil {
		t.Fatalf("SaveHaloydConfig() error = %v", err)
	}

	certDir := filepath.Join(dataDir, constants.CertStorageDir)
	if err := os.MkdirAll(certDir, constants.ModeDirPrivate); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	legacyCertPath := filepath.Join(certDir, "API.EXAMPLE.COM.pem")
	if err := os.WriteFile(legacyCertPath, []byte("cert"), constants.ModeFileSecret); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := configSetCmd()
	cmd.SetArgs([]string{"api-domain", "api.example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set error = %v", err)
	}

	if _, err := os.Stat(legacyCertPath); !os.IsNotExist(err) {
		t.Fatalf("legacy uppercase cert still exists or stat failed unexpectedly: %v", err)
	}
}

func TestConfigSetAPIDomainCreatesMissingConfig(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv(constants.EnvVarConfigDir, configDir)
	t.Setenv(constants.EnvVarDataDir, dataDir)

	cmd := configSetCmd()
	cmd.SetArgs([]string{"api-domain", "api.example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set error = %v", err)
	}

	loaded, err := config.LoadHaloydConfig(filepath.Join(configDir, constants.HaloydConfigFileName))
	if err != nil {
		t.Fatalf("LoadHaloydConfig() error = %v", err)
	}
	if loaded == nil || loaded.API.Domain != "api.example.com" {
		t.Fatalf("loaded config = %+v, want api.example.com", loaded)
	}
}

func TestConfigGetAPIDomainHandlesMissingConfig(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(constants.EnvVarConfigDir, configDir)

	cmd := configGetCmd()
	cmd.SetArgs([]string{"api-domain", "--raw"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get error = %v", err)
	}
}

func TestConfigSetAPIDomainRejectsMalformedConfig(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv(constants.EnvVarConfigDir, configDir)
	t.Setenv(constants.EnvVarDataDir, dataDir)

	configPath := filepath.Join(configDir, constants.HaloydConfigFileName)
	original := []byte("api:\n  domain: api.example.com\n    invalid_indent: true\n")
	if err := os.WriteFile(configPath, original, constants.ModeFileDefault); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := configSetCmd()
	cmd.SetArgs([]string{"api-domain", "new.example.com"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("config set succeeded with malformed config, want error")
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(after) != string(original) {
		t.Fatalf("malformed config was modified:\n%s", after)
	}
}

func TestConfigSetAPIDomainReportsCleanupFailureAfterSaving(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv(constants.EnvVarConfigDir, configDir)
	t.Setenv(constants.EnvVarDataDir, dataDir)

	existing := &config.HaloydConfig{
		API: config.HaloydAPIConfig{Domain: "old.example.com"},
	}
	if err := config.SaveHaloydConfig(existing, filepath.Join(configDir, constants.HaloydConfigFileName)); err != nil {
		t.Fatalf("SaveHaloydConfig() error = %v", err)
	}

	certPath := filepath.Join(dataDir, constants.CertStorageDir, "old.example.com.pem")
	if err := os.MkdirAll(certPath, constants.ModeDirPrivate); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(certPath, "child"), []byte("x"), constants.ModeFileDefault); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := configSetCmd()
	cmd.SetArgs([]string{"api-domain", "new.example.com"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("config set succeeded, want cleanup error")
	}
	if !strings.Contains(err.Error(), certPath) {
		t.Fatalf("error %q does not include cert path %q", err, certPath)
	}

	loaded, err := config.LoadHaloydConfig(filepath.Join(configDir, constants.HaloydConfigFileName))
	if err != nil {
		t.Fatalf("LoadHaloydConfig() error = %v", err)
	}
	if loaded.API.Domain != "new.example.com" {
		t.Fatalf("API.Domain = %q, want new.example.com", loaded.API.Domain)
	}
}
