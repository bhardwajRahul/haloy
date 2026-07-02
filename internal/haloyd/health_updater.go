package haloyd

import (
	"log/slog"

	"github.com/haloydev/haloy/internal/healthcheck"
	"github.com/haloydev/haloy/internal/proxy"
)

// HealthConfigUpdater bridges the health monitor to the proxy configuration.
// When health state changes, it rebuilds the proxy config with only healthy backends.
type HealthConfigUpdater struct {
	deploymentManager *DeploymentManager
	proxy             *proxy.Proxy
	apiDomain         string
	logger            *slog.Logger
}

// NewHealthConfigUpdater creates a new health config updater.
func NewHealthConfigUpdater(
	deploymentManager *DeploymentManager,
	proxy *proxy.Proxy,
	apiDomain string,
	logger *slog.Logger,
) *HealthConfigUpdater {
	return &HealthConfigUpdater{
		deploymentManager: deploymentManager,
		proxy:             proxy,
		apiDomain:         apiDomain,
		logger:            logger,
	}
}

// OnHealthChange is called when the health state of any target changes.
// It rebuilds the proxy configuration, filtering unhealthy backends while keeping routes.
func (u *HealthConfigUpdater) OnHealthChange(healthyTargets []healthcheck.Target) {
	// Build a set of healthy container IDs for quick lookup
	healthyIDs := make(map[string]struct{}, len(healthyTargets))
	for _, t := range healthyTargets {
		healthyIDs[t.ID] = struct{}{}
	}

	deployments := u.deploymentManager.Deployments()

	for appName, d := range deployments {
		healthyCount := 0
		for _, inst := range d.Instances {
			if _, isHealthy := healthyIDs[inst.ContainerID]; isHealthy {
				healthyCount++
			}
		}
		if healthyCount == 0 {
			u.logger.Warn("App has no healthy backends",
				"app", appName,
				"total_instances", len(d.Instances))
		}
	}

	proxyConfig, err := buildProxyConfig(deployments, u.deploymentManager.FailedDeployments(), u.apiDomain,
		func(inst DeploymentInstance) bool {
			_, isHealthy := healthyIDs[inst.ContainerID]
			return isHealthy
		})
	if err != nil {
		u.logger.Error("Failed to build proxy config from health check", "error", err)
		return
	}

	u.proxy.UpdateConfig(proxyConfig)
	u.logger.Info("Proxy configuration updated from health monitor",
		"apps", len(deployments),
		"healthy_targets", len(healthyTargets))
}
