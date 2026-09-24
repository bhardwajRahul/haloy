package haloy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haloydev/haloy/internal/apitypes"
	"github.com/haloydev/haloy/internal/config"
	"github.com/haloydev/haloy/internal/constants"
)

func newVersionServer(statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/version" {
			w.WriteHeader(statusCode)
			if statusCode == http.StatusOK {
				json.NewEncoder(w).Encode(apitypes.VersionResponse{Version: "test"})
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func targetWithServer(serverURL string) config.TargetConfig {
	return config.TargetConfig{
		Server:   serverURL,
		APIToken: &config.ValueSource{Value: "test-token"},
	}
}

func TestCheckServerAuth_ValidAuth(t *testing.T) {
	srv := newVersionServer(http.StatusOK)
	defer srv.Close()

	target := targetWithServer(srv.URL)
	err := checkServerAuth(context.Background(), target.Server, &target)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckServerAuth_InvalidAuth(t *testing.T) {
	srv := newVersionServer(http.StatusUnauthorized)
	defer srv.Close()

	target := targetWithServer(srv.URL)
	err := checkServerAuth(context.Background(), target.Server, &target)
	if err == nil {
		t.Fatal("expected error for unauthorized, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed - check your "+constants.EnvVarAPIToken) {
		t.Fatalf("expected unauthorized error, got: %v", err)
	}
}

func TestCheckServerAuth_ServerUnreachable(t *testing.T) {
	target := targetWithServer("http://127.0.0.1:1")
	err := checkServerAuth(context.Background(), target.Server, &target)
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

func TestCheckServerAuth_MissingToken(t *testing.T) {
	t.Setenv(constants.EnvVarConfigDir, t.TempDir())
	t.Setenv(constants.EnvVarAPIToken, "")

	srv := newVersionServer(http.StatusOK)
	defer srv.Close()

	target := config.TargetConfig{
		Server: srv.URL,
	}
	err := checkServerAuth(context.Background(), target.Server, &target)
	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
}

func TestCheckServersAuth_DeduplicatesSameServer(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/version" {
			callCount++
			json.NewEncoder(w).Encode(apitypes.VersionResponse{Version: "test"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	targets := map[string]config.TargetConfig{
		"target1": targetWithServer(srv.URL),
		"target2": targetWithServer(srv.URL),
	}

	err := checkServersAuth(context.Background(), targets)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected version endpoint called once, got %d", callCount)
	}
}

func TestCheckServersAuth_MultipleServersOneFailsReturnsError(t *testing.T) {
	goodSrv := newVersionServer(http.StatusOK)
	defer goodSrv.Close()

	badSrv := newVersionServer(http.StatusUnauthorized)
	defer badSrv.Close()

	targets := map[string]config.TargetConfig{
		"good": targetWithServer(goodSrv.URL),
		"bad":  targetWithServer(badSrv.URL),
	}

	err := checkServersAuth(context.Background(), targets)
	if err == nil {
		t.Fatal("expected error when one server fails auth, got nil")
	}
}

func TestGetToken_InlineTokenTakesPriority(t *testing.T) {
	t.Setenv(constants.EnvVarAPIToken, "env-token")
	t.Setenv(constants.EnvVarConfigDir, t.TempDir())

	target := &config.TargetConfig{
		APIToken: &config.ValueSource{Value: "inline-token"},
	}
	token, err := getToken(target, "https://example.com")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if token != "inline-token" {
		t.Fatalf("expected inline-token, got: %s", token)
	}
}

func TestGetToken_HALOYAPITOKENFallback(t *testing.T) {
	t.Setenv(constants.EnvVarAPIToken, "fallback-token")
	t.Setenv(constants.EnvVarConfigDir, t.TempDir())

	token, err := getToken(nil, "https://example.com")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if token != "fallback-token" {
		t.Fatalf("expected fallback-token, got: %s", token)
	}
}

func TestGetToken_ErrorWhenNoTokenAvailable(t *testing.T) {
	t.Setenv(constants.EnvVarConfigDir, t.TempDir())
	t.Setenv(constants.EnvVarAPIToken, "")

	_, err := getToken(nil, "https://example.com")
	if err == nil {
		t.Fatal("expected error when no token available, got nil")
	}
	if !strings.Contains(err.Error(), constants.EnvVarAPIToken) {
		t.Fatalf("expected error to mention %s, got: %v", constants.EnvVarAPIToken, err)
	}
	if !strings.Contains(err.Error(), "haloy server add") {
		t.Fatalf("expected error to mention 'haloy server add', got: %v", err)
	}
}

func TestGetToken_ClientConfigTokenOverHALOYAPITOKEN(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(constants.EnvVarConfigDir, tmpDir)
	t.Setenv(constants.EnvVarAPIToken, "fallback-token")

	serverTokenEnv := "HALOY_TEST_SERVER_TOKEN"
	t.Setenv(serverTokenEnv, "server-specific-token")

	clientConfigContent := `servers:
  "example.com":
    token_env: ` + serverTokenEnv + `
`
	err := os.WriteFile(filepath.Join(tmpDir, constants.ClientConfigFileName), []byte(clientConfigContent), 0o644)
	if err != nil {
		t.Fatalf("failed to write client config: %v", err)
	}

	token, err := getToken(nil, "https://example.com")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if token != "server-specific-token" {
		t.Fatalf("expected server-specific-token, got: %s", token)
	}
}
