package haloyd

import (
	"github.com/haloydev/haloy/internal/proxy"
)

// buildProxyConfig converts deployments into a validated proxy routing config.
// includeInstance filters which instances become backends; nil includes all.
// Apps in failedDeployments that are no longer deployed keep their routes with
// no backends, so the proxy serves 502 instead of 404 for them.
func buildProxyConfig(
	deployments map[string]Deployment,
	failedDeployments map[string]Deployment,
	apiDomain string,
	includeInstance func(DeploymentInstance) bool,
) (*proxy.Config, error) {
	rb := proxy.NewRouteBuilder()
	rb.SetAPIDomain(apiDomain)

	for _, d := range deployments {
		var backends []proxy.Backend
		for _, inst := range d.Instances {
			if includeInstance != nil && !includeInstance(inst) {
				continue
			}
			backends = append(backends, proxy.Backend{IP: inst.IP, Port: inst.Port})
		}

		for _, domain := range d.Labels.Domains {
			if domain.Canonical == "" {
				continue
			}
			rb.AddRoute(domain.Canonical, domain.Aliases, backends)
		}
	}

	for appName, d := range failedDeployments {
		if _, exists := deployments[appName]; exists {
			continue
		}

		for _, domain := range d.Labels.Domains {
			if domain.Canonical == "" {
				continue
			}
			rb.AddRoute(domain.Canonical, domain.Aliases, nil)
		}
	}

	return rb.Build()
}
