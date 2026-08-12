package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"gitlab.intsig.net/xparse/xparse-client/internal/config"
	"gitlab.intsig.net/xparse/xparse-client/internal/oauthclient"
)

var authShow bool

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Configure Textin API credentials",
	Long: `Configure and inspect Textin xParser authentication.

Running 'xparse-cli auth' in a terminal opens an authentication menu. Piped
or scripted input preserves the legacy AppKey setup behavior.

Get your credentials:
  https://www.textin.com/user/login?redirect=%252Fconsole%252Fdashboard%252Fsetting&from=xparse-parse-skill

Credentials are resolved in this order:
  1. --app-id / --secret-code flags
  2. XPARSE_APP_ID / XPARSE_SECRET_CODE environment variables
  3. ~/.xparse-cli/config.yaml (set via 'xparse-cli auth')`,
	Example: `  xparse-cli auth                    # Interactive authentication menu
  xparse-cli auth --show            # Show current AppKey source and masked values
  xparse-cli auth app-key            # Direct AppKey setup
  xparse-cli auth device             # OAuth Device Flow
  xparse-cli auth browser            # OAuth Authorization Code + PKCE
  xparse-cli auth browser --prompt consent
  xparse-cli auth status --output=json

  # Environment variables (useful for CI/CD):
  export XPARSE_APP_ID=your_app_id
  export XPARSE_SECRET_CODE=your_secret_code`,
	Args: cobra.NoArgs,
	RunE: runAuth,
}

func init() {
	rootCmd.AddCommand(authCmd)

	authCmd.Flags().BoolVar(&authShow, "show", false, "Show current credential source")
}

func runAuth(cmd *cobra.Command, args []string) error {
	if authShow {
		return runAuthShow()
	}
	if terminalInput(cmd.InOrStdin()) {
		return runAuthMenu(cmd)
	}
	return runAuthSetupIO(cmd.InOrStdin(), cmd.OutOrStdout())
}

func terminalInput(input io.Reader) bool {
	file, ok := input.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func runAuthMenu(cmd *cobra.Command) error {
	output := cmd.OutOrStdout()
	choice, err := authTUISelectAction(cmd)
	if err != nil {
		if authTUIWasCanceled(err) {
			fmt.Fprintln(output, "Canceled")
			return nil
		}
		return fmt.Errorf("authentication menu: %w", err)
	}

	switch choice {
	case authMenuOAuth:
		if available, reason := oauthBrowserAvailable(); available {
			fmt.Fprintln(output, "Using browser OAuth.")
			previousPolicy := authBrowserOpenBrowser
			authBrowserOpenBrowser = string(oauthclient.OpenAlways)
			defer func() { authBrowserOpenBrowser = previousPolicy }()
			if err := runAuthBrowser(cmd, nil); err != nil {
				if !errors.Is(err, oauthclient.ErrBrowserOpen) {
					return err
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "Browser could not be opened; falling back to Device OAuth.")
				authDeviceOpenBrowser = string(oauthclient.OpenAuto)
				authDeviceOutput = "text"
				return runAuthDevice(cmd, nil)
			}
			return nil
		} else {
			fmt.Fprintf(output, "Using Device OAuth (%s).\n", reason)
			authDeviceOpenBrowser = string(oauthclient.OpenAuto)
			authDeviceOutput = "text"
			return runAuthDevice(cmd, nil)
		}
	case authMenuAppKey:
		return runAuthSetupTUI(cmd)
	case authMenuStatus:
		return runAuthStatus(cmd, nil)
	case authMenuLogout:
		return runAuthLogoutMenu(cmd)
	case authMenuCancel:
		fmt.Fprintln(output, "Canceled")
		return nil
	default:
		return fmt.Errorf("invalid authentication menu action %q", choice)
	}
}

func runAuthLogoutMenu(cmd *cobra.Command) error {
	output := cmd.OutOrStdout()
	method, err := authTUISelectLogout(cmd)
	if err != nil {
		if authTUIWasCanceled(err) {
			fmt.Fprintln(output, "Canceled")
			return nil
		}
		return fmt.Errorf("logout menu: %w", err)
	}
	if method == "" {
		fmt.Fprintln(output, "Canceled")
		return nil
	}
	authLogoutMethod = method
	return runAuthLogout(cmd, nil)
}

func runAuthShow() error {
	credSrc, err := config.ResolveCredentials(nil)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if credSrc.AppID == "" {
		fmt.Println("No credentials configured.")
		fmt.Println("Run 'xparse-cli auth' to set up your API credentials.")
		return nil
	}

	fmt.Printf("Credential source: %s\n", credSrc.Source)
	fmt.Printf("App ID:      %s\n", maskToken(credSrc.AppID))
	fmt.Printf("Secret Code: %s\n", maskToken(credSrc.SecretCode))

	cfg, err := config.Load()
	if err == nil && cfg.BaseURL != "" {
		fmt.Printf("Base URL:    %s\n", cfg.BaseURL)
	}
	return nil
}

func runAuthSetup() error {
	return runAuthSetupIO(os.Stdin, os.Stdout)
}

func runAuthSetupIO(input io.Reader, output io.Writer) error {
	fmt.Fprintln(output, "Textin xParser API Credential Setup")
	fmt.Fprintln(output, "Get your credentials from: https://www.textin.com/user/login?redirect=%252Fconsole%252Fdashboard%252Fsetting&from=xparse-parse-skill")
	fmt.Fprintln(output)

	reader := bufio.NewReader(input)

	existing, _ := config.ResolveCredentials(nil)
	if existing.AppID != "" {
		fmt.Fprintf(output, "Current credential source: %s\n", existing.Source)
		fmt.Fprintf(output, "Current App ID: %s\n", maskToken(existing.AppID))
		fmt.Fprintln(output)
	}

	// Read App ID
	if existing.AppID != "" {
		fmt.Fprint(output, "Enter new App ID (or press Enter to keep current): ")
	} else {
		fmt.Fprint(output, "Enter your App ID (x-ti-app-id): ")
	}
	appID, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	appID = strings.TrimSpace(appID)

	if appID == "" && existing.AppID != "" {
		appID = existing.AppID
	}
	if appID == "" {
		return fmt.Errorf("app-id is required")
	}

	// Read Secret Code
	if existing.SecretCode != "" {
		fmt.Fprint(output, "Enter new Secret Code (or press Enter to keep current): ")
	} else {
		fmt.Fprint(output, "Enter your Secret Code (x-ti-secret-code): ")
	}
	var secretCode string
	if inputFile, ok := input.(*os.File); ok && term.IsTerminal(int(inputFile.Fd())) {
		value, readErr := term.ReadPassword(int(inputFile.Fd()))
		fmt.Fprintln(output)
		secretCode, err = string(value), readErr
	} else {
		secretCode, err = reader.ReadString('\n')
	}
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	secretCode = strings.TrimSpace(secretCode)

	if secretCode == "" && existing.SecretCode != "" {
		secretCode = existing.SecretCode
	}
	if secretCode == "" {
		return fmt.Errorf("secret-code is required")
	}

	if err := config.SetCredentials(appID, secretCode); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Fprintf(output, "Credentials saved to %s\n", config.Path())
	return nil
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
