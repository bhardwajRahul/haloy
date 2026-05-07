package docker

import (
	"context"
	"fmt"
	"time"

	dockerregistry "github.com/docker/docker/api/types/registry"
	"github.com/haloydev/haloy/internal/config"
)

const dockerHubAuthServerAddress = "https://index.docker.io/v1/"

func registryAuthServerAddress(server string) string {
	normalized := config.NormalizeRegistryServer(server)
	if normalized == "docker.io" {
		return dockerHubAuthServerAddress
	}
	return normalized
}

func registryAuthConfig(auth config.RegistryAuth) (dockerregistry.AuthConfig, error) {
	resolved, err := config.ResolveRegistryAuth(auth)
	if err != nil {
		return dockerregistry.AuthConfig{}, err
	}

	server := registryAuthServerAddress(resolved.Server)
	return dockerregistry.AuthConfig{
		Username:      resolved.Username.Value,
		Password:      resolved.Password.Value,
		ServerAddress: server,
	}, nil
}

func VerifyRegistryLogin(ctx context.Context, auth config.RegistryAuth) error {
	authConfig, err := registryAuthConfig(auth)
	if err != nil {
		return fmt.Errorf("failed to resolve registry credentials: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cli, err := NewClient(ctx)
	if err != nil {
		return err
	}
	defer cli.Close()

	if _, err := cli.RegistryLogin(ctx, authConfig); err != nil {
		return fmt.Errorf("docker login to %s failed: %w", config.NormalizeRegistryServer(auth.Server), err)
	}
	return nil
}
