// Package config handles credential resolution and configuration file management.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const configDirName = ".xparse-cli"
const configFileName = "config.yaml"

// Config represents the configuration file structure.
type Config struct {
	AppID      string `yaml:"app_id,omitempty"`
	SecretCode string `yaml:"secret_code,omitempty"`
	BaseURL    string `yaml:"base_url,omitempty"`
}

// configPath returns the path to the config file.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot get home directory: %w", err)
	}
	return filepath.Join(home, configDirName, configFileName), nil
}

// Path returns the config file path for display purposes.
func Path() string {
	p, err := configPath()
	if err != nil {
		return "~/" + configDirName + "/" + configFileName
	}
	return p
}

// Load reads the configuration file.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// Save writes the configuration file.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// CredentialSource describes where credentials came from.
type CredentialSource struct {
	AppID      string
	SecretCode string
	Source     string // "flag", "env", "config", ""
}

// ResolveCredentials resolves the API credentials from multiple sources in priority order:
// 1. --app-id / --secret-code flags
// 2. XPARSE_APP_ID / XPARSE_SECRET_CODE env vars
// 3. ~/.xparse-cli/config.yaml
func ResolveCredentials(cmd *cobra.Command) (*CredentialSource, error) {
	// 1. Check flags
	if cmd != nil {
		appID, _ := cmd.Flags().GetString("app-id")
		secretCode, _ := cmd.Flags().GetString("secret-code")
		if strings.TrimSpace(appID) != "" && strings.TrimSpace(secretCode) != "" {
			return &CredentialSource{AppID: appID, SecretCode: secretCode, Source: "flag"}, nil
		}
	}

	// 2. Check env vars (V1: XPARSE_APP_ID / XPARSE_SECRET_CODE)
	appIDEnv := os.Getenv("XPARSE_APP_ID")
	secretCodeEnv := os.Getenv("XPARSE_SECRET_CODE")
	if strings.TrimSpace(appIDEnv) != "" && strings.TrimSpace(secretCodeEnv) != "" {
		return &CredentialSource{AppID: appIDEnv, SecretCode: secretCodeEnv, Source: "env"}, nil
	}

	// 3. Check config file
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.AppID) != "" && strings.TrimSpace(cfg.SecretCode) != "" {
		return &CredentialSource{AppID: cfg.AppID, SecretCode: cfg.SecretCode, Source: "config"}, nil
	}

	return &CredentialSource{}, nil
}

// GetBaseURL returns the base URL from flag, config, or default.
func GetBaseURL(cmd *cobra.Command, cfg *Config) string {
	if cmd != nil {
		baseURL, err := cmd.Flags().GetString("base-url")
		if err == nil && strings.TrimSpace(baseURL) != "" {
			return baseURL
		}
	}
	if cfg != nil && strings.TrimSpace(cfg.BaseURL) != "" {
		return cfg.BaseURL
	}
	return "https://api.textin.com"
}

// SetCredentials saves the credentials to the config file.
func SetCredentials(appID, secretCode string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.AppID = appID
	cfg.SecretCode = secretCode
	return Save(cfg)
}
