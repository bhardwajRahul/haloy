package haloydcli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/haloydev/haloy/internal/config"
	"github.com/haloydev/haloy/internal/constants"
	"github.com/haloydev/haloy/internal/helpers"
	"github.com/haloydev/haloy/internal/ui"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

const certPEMExt = ".pem"

// restartCommand returns the appropriate command to restart haloyd based on the init system
func restartCommand() string {
	return helpers.RestartCommand()
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage haloyd configuration",
		Long:  "Commands to get and set haloyd configuration values.",
	}

	cmd.AddCommand(
		configGetCmd(),
		configSetCmd(),
		configReloadCertsCmd(),
		newConfigGenerateTokenCmd(),
	)

	return cmd
}

func configGetCmd() *cobra.Command {
	var raw bool

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: `Get a configuration value.

Available keys:
  api-token   - The API authentication token
  api-domain  - The configured API domain`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			configDir, err := config.HaloydConfigDir()
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			switch key {
			case "api-token":
				envPath := filepath.Join(configDir, constants.ConfigEnvFileName)
				env, err := godotenv.Read(envPath)
				if err != nil {
					return fmt.Errorf("failed to read .env file: %w", err)
				}
				token := env[constants.EnvVarAPIToken]
				if raw {
					fmt.Print(token)
				} else {
					ui.Info("API Token: %s", token)
				}

			case "api-url", "api-domain":
				haloydConfig, err := loadHaloydConfig(configDir)
				if err != nil {
					return err
				}
				domain := haloydConfig.API.Domain
				if raw {
					fmt.Print(domain)
				} else {
					if domain == "" {
						ui.Info("API domain is not configured")
					} else {
						ui.Info("API domain: %s", domain)
					}
				}

			default:
				return fmt.Errorf("unknown config key: %s", key)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&raw, "raw", false, "Output raw value without formatting")

	return cmd
}

func configSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value.

Available keys:
  api-domain  - The domain for the haloyd API

Note: After changing configuration, restart haloyd for changes to take effect.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			configDir, err := config.HaloydConfigDir()
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			haloydConfig, err := loadHaloydConfig(configDir)
			if err != nil {
				return err
			}

			var postSaveErr error
			switch key {
			case "api-domain":
				oldDomain := haloydConfig.API.Domain
				value = strings.ToLower(value)
				haloydConfig.API.Domain = value

				if err := saveHaloydConfig(configDir, haloydConfig); err != nil {
					return err
				}

				ui.Info("API domain set to: %s", value)
				postSaveErr = cleanupOldAPIDomainCertificate(oldDomain, value)

			default:
				return fmt.Errorf("unknown config key: %s", key)
			}

			ui.Info("Restart haloyd for changes to take effect: %s", restartCommand())

			return postSaveErr
		},
	}

	return cmd
}

func configReloadCertsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reload-certs",
		Short: "Reload TLS certificates",
		Long: `Trigger an immediate reload of TLS certificates.

This is useful after manually adding certificate files to the cert-storage directory.
Certificate reloads happen only when explicitly triggered.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// This command would need to communicate with the running haloyd
			// For now, we'll just print a message suggesting a restart
			ui.Info("To reload certificates, restart haloyd: %s", restartCommand())
			ui.Info("Certificate reloads occur only when explicitly triggered.")
			return nil
		},
	}
}

func newConfigGenerateTokenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate-token",
		Short: "Generate a new API token",
		Long: `Generate a new API authentication token.

Warning: This will invalidate the existing token. You will need to update
the token in your haloy CLI configuration after running this command.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir, err := config.HaloydConfigDir()
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			bytes := make([]byte, apiTokenLength)
			if _, err := rand.Read(bytes); err != nil {
				return fmt.Errorf("failed to generate token: %w", err)
			}
			newToken := hex.EncodeToString(bytes)

			envPath := filepath.Join(configDir, constants.ConfigEnvFileName)
			env, err := godotenv.Read(envPath)
			if err != nil {
				env = make(map[string]string)
			}
			env[constants.EnvVarAPIToken] = newToken

			if err := godotenv.Write(env, envPath); err != nil {
				return fmt.Errorf("failed to write .env file: %w", err)
			}

			if err := os.Chmod(envPath, constants.ModeFileSecret); err != nil {
				ui.Warn("Failed to set file permissions: %v", err)
			}

			ui.Success("New API token generated: %s", newToken)
			ui.Info("Restart haloyd for the new token to take effect: %s", restartCommand())
			ui.Info("Update your haloy CLI with: haloy server add <server-name> %s --force", newToken)

			return nil
		},
	}
}

func loadHaloydConfig(configDir string) (*config.HaloydConfig, error) {
	configPath := filepath.Join(configDir, constants.HaloydConfigFileName)
	haloydConfig, err := config.LoadHaloydConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if haloydConfig == nil {
		return &config.HaloydConfig{}, nil
	}
	return haloydConfig, nil
}

func saveHaloydConfig(configDir string, haloydConfig *config.HaloydConfig) error {
	if err := haloydConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	configPath := filepath.Join(configDir, constants.HaloydConfigFileName)
	if err := config.SaveHaloydConfig(haloydConfig, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func cleanupOldAPIDomainCertificate(oldDomain, newDomain string) error {
	oldDomainNormalized := strings.ToLower(oldDomain)
	newDomainNormalized := strings.ToLower(newDomain)
	if oldDomainNormalized == "" {
		return nil
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return fmt.Errorf("API domain updated, but failed to determine data directory for old certificate cleanup: %w", err)
	}

	certDir := filepath.Join(dataDir, constants.CertStorageDir)
	certNames := make(map[string]struct{})
	if oldDomainNormalized != newDomainNormalized {
		certNames[oldDomainNormalized] = struct{}{}
	}
	if oldDomain != oldDomainNormalized {
		certNames[oldDomain] = struct{}{}
	}

	for certName := range certNames {
		certPath := filepath.Join(certDir, certName+certPEMExt)
		if err := os.Remove(certPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("API domain updated, but failed to remove old API domain certificate %s: %w", certPath, err)
		}
		ui.Info("Removed old API domain certificate: %s", certPath)
	}

	return nil
}
