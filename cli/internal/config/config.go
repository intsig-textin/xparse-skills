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
const DefaultOAuthClientID = "cli_textin_xparse"
const ProfileWorkBuddy = "workbuddy"

var activeProfile string

// Config represents the configuration file structure.
type Config struct {
	AppID             string      `yaml:"app_id,omitempty"`
	SecretCode        string      `yaml:"secret_code,omitempty"`
	BaseURL           string      `yaml:"base_url,omitempty"`
	DefaultAuthMethod string      `yaml:"default_auth_method,omitempty"`
	OAuth             OAuthConfig `yaml:"oauth,omitempty"`
}

// OAuthConfig contains non-secret OAuth client preferences. Tokens are stored
// separately so legacy config reset and AppKey operations do not log users out.
type OAuthConfig struct {
	ClientID    string `yaml:"client_id,omitempty"`
	Scope       string `yaml:"scope,omitempty"`
	RedirectURI string `yaml:"redirect_uri,omitempty"`
}

// SetProfile selects an isolated CLI profile for the current process.
func SetProfile(profile string) error {
	profile = strings.ToLower(strings.TrimSpace(profile))
	switch profile {
	case "", ProfileWorkBuddy:
		activeProfile = profile
		return nil
	default:
		return fmt.Errorf("unsupported profile %q", profile)
	}
}

// Profile returns the active CLI profile.
func Profile() string {
	return activeProfile
}

// Dir returns the credential directory. XPARSE_CONFIG_DIR remains the most
// specific override; named profiles otherwise live below ~/.xparse-cli.
func Dir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("XPARSE_CONFIG_DIR")); dir != "" {
		return filepath.Clean(dir), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot get home directory: %w", err)
	}
	dir := filepath.Join(home, configDirName)
	if activeProfile != "" {
		dir = filepath.Join(dir, "profiles", activeProfile)
	}
	return dir, nil
}

// configPath returns the path to the config file.
func configPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// Path returns the config file path for display purposes.
func Path() string {
	p, err := configPath()
	if err != nil {
		return "~/" + configDirName + "/" + configFileName
	}
	return p
}

// OAuthTokenPath returns the separate OAuth credential path.
func OAuthTokenPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "oauth-token.json"), nil
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
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := atomicWriteFile(path, data, 0o600); err != nil {
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
	if baseURL := strings.TrimSpace(os.Getenv("XPARSE_BASE_URL")); baseURL != "" {
		return strings.TrimRight(baseURL, "/")
	}
	if cfg != nil && strings.TrimSpace(cfg.BaseURL) != "" {
		return strings.TrimRight(cfg.BaseURL, "/")
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
	cfg.DefaultAuthMethod = "app-key"
	return Save(cfg)
}

// SetDefaultAuthMethod records an explicit user choice. An empty value restores
// compatibility auto-selection, which remains AppKey-first.
func SetDefaultAuthMethod(method string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.DefaultAuthMethod = strings.TrimSpace(method)
	return Save(cfg)
}

// ResolveOAuthClientID uses flag > environment > YAML precedence.
func ResolveOAuthClientID(flagValue string, cfg *Config) string {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("XPARSE_OAUTH_CLIENT_ID")); value != "" {
		return value
	}
	if cfg != nil {
		if value := strings.TrimSpace(cfg.OAuth.ClientID); value != "" {
			return value
		}
	}
	return DefaultOAuthClientID
}

// ResolveOAuthScope uses flag > environment > YAML > ocr:* precedence.
func ResolveOAuthScope(flagValue string, cfg *Config) string {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("XPARSE_OAUTH_SCOPE")); value != "" {
		return value
	}
	if cfg != nil {
		if value := strings.TrimSpace(cfg.OAuth.Scope); value != "" {
			return value
		}
	}
	return "ocr:*"
}

// ResolveOAuthRedirectURI uses flag > environment > YAML > loopback default.
func ResolveOAuthRedirectURI(flagValue string, cfg *Config) string {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("XPARSE_OAUTH_REDIRECT_URI")); value != "" {
		return value
	}
	if cfg != nil {
		if value := strings.TrimSpace(cfg.OAuth.RedirectURI); value != "" {
			return value
		}
	}
	return "http://127.0.0.1:0/callback"
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) (retErr error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if retErr != nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	dirFile, err := os.Open(dir)
	if err == nil {
		defer dirFile.Close()
		_ = dirFile.Sync()
	}
	return nil
}
