package haloy

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/haloydev/haloy/internal/apiclient"
	"github.com/haloydev/haloy/internal/apitypes"
	"github.com/haloydev/haloy/internal/config"
	"github.com/haloydev/haloy/internal/constants"
	"github.com/haloydev/haloy/internal/helpers"
	"github.com/oklog/ulid"
)

func createDeploymentID() string {
	entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
	return strings.ToLower(id)
}

func checkServerAuth(ctx context.Context, server string, targetConfig *config.TargetConfig) error {
	token, err := getToken(targetConfig, server)
	if err != nil {
		return fmt.Errorf("server %s: %w", server, err)
	}

	api, err := apiclient.New(server, token)
	if err != nil {
		return fmt.Errorf("server %s: unable to create API client: %w", server, err)
	}

	var version apitypes.VersionResponse
	if err := api.Get(ctx, "version", &version); err != nil {
		return fmt.Errorf("server %s: authentication check failed: %w", server, err)
	}

	return nil
}

func checkServersAuth(ctx context.Context, targets map[string]config.TargetConfig) error {
	checked := make(map[string]bool)
	for _, target := range targets {
		normalized, err := helpers.NormalizeServerURL(target.Server)
		if err != nil {
			return fmt.Errorf("invalid server URL %q: %w", target.Server, err)
		}
		if checked[normalized] {
			continue
		}
		checked[normalized] = true
		if err := checkServerAuth(ctx, target.Server, &target); err != nil {
			return err
		}
	}
	return nil
}

func getToken(targetConfig *config.TargetConfig, url string) (string, error) {
	if targetConfig != nil && targetConfig.APIToken != nil && targetConfig.APIToken.Value != "" {
		return targetConfig.APIToken.Value, nil
	}

	configDir, err := config.HaloyConfigDir()
	if err != nil {
		return "", err
	}
	clientConfigPath := filepath.Join(configDir, constants.ClientConfigFileName)
	clientConfig, err := config.LoadClientConfig(clientConfigPath)
	if err != nil {
		return "", err
	}

	if clientConfig == nil {
		return "", fmt.Errorf("no client configuration found. Run: haloy server add <url> <token>")
	}

	normalizedURL, err := helpers.NormalizeServerURL(url)
	if err != nil {
		return "", err
	}

	serverConfig, exists := clientConfig.Servers[normalizedURL]
	if !exists {
		return "", fmt.Errorf("server %s not configured. Run: haloy server add %s <token>", normalizedURL, normalizedURL)
	}

	token := os.Getenv(serverConfig.TokenEnv)
	if token == "" {
		return "", fmt.Errorf("token not found for server %s. Please set environment variable: %s", normalizedURL, serverConfig.TokenEnv)
	}

	return token, nil
}
