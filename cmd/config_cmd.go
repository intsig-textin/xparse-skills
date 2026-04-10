package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long:  `View, set, or reset xParser CLI configuration stored in ~/.xparse-cli/config.yaml.`,
}

// ── config show ──

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		credSrc, _ := config.ResolveCredentials(nil)

		fmt.Println("# xParser CLI Configuration")
		fmt.Println()

		if credSrc.AppID != "" {
			fmt.Printf("app_id:      %s  (source: %s)\n", maskToken(credSrc.AppID), credSrc.Source)
			fmt.Printf("secret_code: %s  (source: %s)\n", maskToken(credSrc.SecretCode), credSrc.Source)
		} else {
			fmt.Println("app_id:      (not set)")
			fmt.Println("secret_code: (not set)")
		}

		if cfg.BaseURL != "" {
			fmt.Printf("base_url:    %s\n", cfg.BaseURL)
		} else {
			fmt.Printf("base_url:    %s  (default)\n", "https://api.textin.com")
		}

		return nil
	},
}

// ── config set ──

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value. Supported keys:
  - app_id        Textin App ID
  - secret_code   Textin Secret Code
  - base_url      API base URL (for private deployments)`,
	Example: `  ./xparse-cli config set base_url https://your-server.com
  ./xparse-cli config set app_id your_app_id`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		switch key {
		case "app_id":
			cfg.AppID = value
		case "secret_code":
			cfg.SecretCode = value
		case "base_url":
			cfg.BaseURL = value
		default:
			return fmt.Errorf("unknown config key: %s. Supported: app_id, secret_code, base_url", key)
		}

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Set %s successfully\n", key)
		return nil
	},
}

// ── config reset ──

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	Long:  `Remove all saved configuration from ~/.xparse-cli/config.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Save(&config.Config{}); err != nil {
			return fmt.Errorf("failed to reset config: %w", err)
		}
		fmt.Println("Configuration reset to defaults")
		return nil
	},
}

// ── config path ──

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.Path())
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)
	configCmd.AddCommand(configPathCmd)
}
