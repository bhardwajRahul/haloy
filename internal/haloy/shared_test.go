package haloy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/haloydev/haloy/internal/apitypes"
	"github.com/haloydev/haloy/internal/config"
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
	if !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("expected authentication error, got: %v", err)
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
