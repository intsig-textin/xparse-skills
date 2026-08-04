package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
	"github.com/intsig-textin/xparse-skills/cli/internal/credential"
)

type authMenuAction string

const (
	authMenuOAuth  authMenuAction = "oauth"
	authMenuAppKey authMenuAction = "app-key"
	authMenuStatus authMenuAction = "status"
	authMenuLogout authMenuAction = "logout"
	authMenuCancel authMenuAction = "cancel"
)

type authTUISnapshot struct {
	Environment string
	BaseURL     string
	OAuth       bool
	AppKey      bool
	Active      string
}

type authTUICredentials struct {
	AppID      string
	SecretCode string
}

var (
	authTUISelectAction = runAuthTUIAction
	authTUIReadAppKey   = runAuthTUIAppKey
	authTUISelectLogout = runAuthTUILogout
)

func runAuthTUIAction(cmd *cobra.Command) (authMenuAction, error) {
	snapshot, err := loadAuthTUISnapshot(cmd)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(cmd.OutOrStdout(), renderAuthTUIHeader(snapshot))

	var action authMenuAction
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[authMenuAction]().
				Title("Authentication").
				Description("Choose how you want to authenticate").
				Options(
					huh.NewOption("OAuth login           Browser or Device, auto-detected", authMenuOAuth),
					huh.NewOption("App credentials       Configure App ID / Secret Code", authMenuAppKey),
					huh.NewOption("Authentication status Show the active credential", authMenuStatus),
					huh.NewOption("Logout                Remove saved credentials", authMenuLogout),
					huh.NewOption("Cancel", authMenuCancel),
				).
				Value(&action),
		),
	)
	err = configureAuthTUIForm(cmd, form).RunWithContext(cmd.Context())
	return action, err
}

func runAuthSetupTUI(cmd *cobra.Command) error {
	existing, err := config.ResolveCredentials(nil)
	if err != nil {
		return fmt.Errorf("failed to load existing credentials: %w", err)
	}
	values, err := authTUIReadAppKey(cmd, existing)
	if err != nil {
		if authTUIWasCanceled(err) {
			fmt.Fprintln(cmd.OutOrStdout(), "Canceled")
			return nil
		}
		return fmt.Errorf("App credentials form: %w", err)
	}
	if strings.TrimSpace(values.AppID) == "" {
		values.AppID = existing.AppID
	}
	if strings.TrimSpace(values.SecretCode) == "" {
		values.SecretCode = existing.SecretCode
	}
	if err := config.SetCredentials(strings.TrimSpace(values.AppID), strings.TrimSpace(values.SecretCode)); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Credentials saved to %s\n", config.Path())
	return nil
}

func runAuthTUIAppKey(cmd *cobra.Command, existing *config.CredentialSource) (authTUICredentials, error) {
	var values authTUICredentials
	appDescription := "Header: x-ti-app-id"
	secretDescription := "Header: x-ti-secret-code"
	if existing.AppID != "" {
		appDescription = fmt.Sprintf("Current: %s · leave empty to keep it", maskToken(existing.AppID))
	}
	if existing.SecretCode != "" {
		secretDescription = "A Secret Code is already configured · leave empty to keep it"
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("App ID").
				Description(appDescription).
				Placeholder("Enter your App ID").
				Value(&values.AppID).
				Validate(requiredCredential("App ID", existing.AppID)),
			huh.NewInput().
				Title("Secret Code").
				Description(secretDescription).
				Placeholder("Enter your Secret Code").
				EchoMode(huh.EchoModePassword).
				Value(&values.SecretCode).
				Validate(requiredCredential("Secret Code", existing.SecretCode)),
		).Title("App credentials").
			Description("Configure credentials for paid TextIn APIs"),
	)
	err := configureAuthTUIForm(cmd, form).RunWithContext(cmd.Context())
	return values, err
}

func runAuthTUILogout(cmd *cobra.Command) (string, error) {
	method := "oauth"
	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Logout").
				Description("Choose which local credentials to remove").
				Options(
					huh.NewOption("OAuth session", "oauth"),
					huh.NewOption("App ID / Secret Code", "app-key"),
					huh.NewOption("All credentials", "all"),
					huh.NewOption("Cancel", ""),
				).
				Value(&method),
		),
	)
	if err := configureAuthTUIForm(cmd, selectForm).RunWithContext(cmd.Context()); err != nil {
		return "", err
	}
	if method == "" {
		return "", nil
	}

	confirmed := false
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Confirm logout").
				Description(logoutDescription(method)).
				Affirmative("Logout").
				Negative("Cancel").
				Value(&confirmed),
		),
	)
	if err := configureAuthTUIForm(cmd, confirmForm).RunWithContext(cmd.Context()); err != nil {
		return "", err
	}
	if !confirmed {
		return "", nil
	}
	return method, nil
}

func configureAuthTUIForm(cmd *cobra.Command, form *huh.Form) *huh.Form {
	return form.
		WithInput(cmd.InOrStdin()).
		WithOutput(cmd.OutOrStdout()).
		WithTheme(textInAuthTheme()).
		WithKeyMap(textInAuthKeyMap()).
		WithWidth(72).
		WithShowHelp(true).
		WithAccessible(strings.TrimSpace(os.Getenv("XPARSE_TUI_ACCESSIBLE")) != "")
}

func textInAuthKeyMap() *huh.KeyMap {
	keyMap := huh.NewDefaultKeyMap()
	keyMap.Quit.SetKeys("ctrl+c", "esc")
	keyMap.Quit.SetHelp("esc", "cancel")
	keyMap.Select.Filter.SetEnabled(false)
	return keyMap
}

func textInAuthTheme() *huh.Theme {
	theme := huh.ThemeBase()
	primary := lipgloss.AdaptiveColor{Light: "#075DCC", Dark: "#5EA7FF"}
	secondary := lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#9CA3AF"}
	foreground := lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F3F4F6"}
	success := lipgloss.AdaptiveColor{Light: "#087A55", Dark: "#34D399"}
	danger := lipgloss.AdaptiveColor{Light: "#C2413A", Dark: "#FB7185"}

	theme.Focused.Base = theme.Focused.Base.
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderForeground(primary).
		PaddingLeft(1)
	theme.Focused.Card = theme.Focused.Base
	theme.Focused.Title = theme.Focused.Title.Foreground(primary).Bold(true)
	theme.Focused.Description = theme.Focused.Description.Foreground(secondary)
	theme.Focused.SelectSelector = lipgloss.NewStyle().Foreground(primary).Bold(true).SetString("› ")
	theme.Focused.Option = theme.Focused.Option.Foreground(foreground)
	theme.Focused.TextInput.Cursor = theme.Focused.TextInput.Cursor.Foreground(primary)
	theme.Focused.TextInput.Prompt = theme.Focused.TextInput.Prompt.Foreground(primary)
	theme.Focused.FocusedButton = theme.Focused.FocusedButton.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(primary).
		Bold(true)
	theme.Focused.BlurredButton = theme.Focused.BlurredButton.Foreground(foreground)
	theme.Focused.ErrorIndicator = theme.Focused.ErrorIndicator.Foreground(danger)
	theme.Focused.ErrorMessage = theme.Focused.ErrorMessage.Foreground(danger)
	theme.Focused.NextIndicator = theme.Focused.NextIndicator.Foreground(primary)
	theme.Focused.PrevIndicator = theme.Focused.PrevIndicator.Foreground(primary)
	theme.Focused.SelectedOption = theme.Focused.SelectedOption.Foreground(success)

	theme.Blurred = theme.Focused
	theme.Blurred.Base = theme.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	theme.Blurred.Card = theme.Blurred.Base
	theme.Blurred.SelectSelector = lipgloss.NewStyle().SetString("  ")
	theme.Blurred.Description = theme.Blurred.Description.Foreground(secondary)
	theme.Group.Title = theme.Focused.Title
	theme.Group.Description = theme.Focused.Description
	return theme
}

func loadAuthTUISnapshot(cmd *cobra.Command) (authTUISnapshot, error) {
	cfg, err := config.Load()
	if err != nil {
		return authTUISnapshot{}, fmt.Errorf("load config: %w", err)
	}
	appKey, err := config.ResolveCredentials(cmd)
	if err != nil {
		return authTUISnapshot{}, fmt.Errorf("resolve App credentials: %w", err)
	}
	baseURL := config.GetBaseURL(cmd, cfg)
	oauthLoggedIn := false
	if store, storeErr := credential.DefaultStore(); storeErr == nil {
		if token, loadErr := store.Load(); loadErr == nil {
			oauthLoggedIn = token.LoggedIn(oauthNow())
		}
	}
	appKeyConfigured := appKey.AppID != "" && appKey.SecretCode != ""
	return authTUISnapshot{
		Environment: authEnvironmentLabel(baseURL),
		BaseURL:     baseURL,
		OAuth:       oauthLoggedIn,
		AppKey:      appKeyConfigured,
		Active:      activeAuthenticationLabel(cfg.DefaultAuthMethod, oauthLoggedIn, appKeyConfigured),
	}, nil
}

func renderAuthTUIHeader(snapshot authTUISnapshot) string {
	primary := lipgloss.AdaptiveColor{Light: "#075DCC", Dark: "#5EA7FF"}
	muted := lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#9CA3AF"}
	title := lipgloss.NewStyle().Foreground(primary).Bold(true).Render("◆  TextIn xParse")
	badgeColor := lipgloss.AdaptiveColor{Light: "#E8F1FF", Dark: "#17365D"}
	if snapshot.Environment == "TEST" {
		badgeColor = lipgloss.AdaptiveColor{Light: "#FFF3D6", Dark: "#594214"}
	}
	badge := lipgloss.NewStyle().
		Foreground(primary).
		Background(badgeColor).
		Bold(true).
		Padding(0, 1).
		Render(snapshot.Environment)
	status := fmt.Sprintf(
		"OAuth %s  ·  AppKey %s  ·  Active %s",
		oauthStatusLabel(snapshot.OAuth),
		appKeyStatusLabel(snapshot.AppKey),
		snapshot.Active,
	)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", badge),
		lipgloss.NewStyle().Foreground(muted).Render(status),
	)
}

func authEnvironmentLabel(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "CUSTOM"
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "api.textin.com":
		return "PRODUCTION"
	case "textin-sandbox.intsig.com":
		return "TEST"
	default:
		return "CUSTOM"
	}
}

func activeAuthenticationLabel(preferred string, oauthLoggedIn, appKeyConfigured bool) string {
	switch preferred {
	case "oauth":
		if oauthLoggedIn {
			return "OAuth"
		}
	case "app-key":
		if appKeyConfigured {
			return "AppKey"
		}
	}
	if appKeyConfigured {
		return "AppKey"
	}
	if oauthLoggedIn {
		return "OAuth"
	}
	return "Free API"
}

func oauthStatusLabel(loggedIn bool) string {
	if loggedIn {
		return "signed in"
	}
	return "signed out"
}

func appKeyStatusLabel(configured bool) string {
	if configured {
		return "configured"
	}
	return "not configured"
}

func requiredCredential(name, existing string) func(string) error {
	return func(value string) error {
		if strings.TrimSpace(value) == "" && strings.TrimSpace(existing) == "" {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
}

func logoutDescription(method string) string {
	switch method {
	case "oauth":
		return "Revoke the OAuth session and remove the local token?"
	case "app-key":
		return "Remove the saved App ID and Secret Code?"
	default:
		return "Remove both OAuth and App credentials?"
	}
}

func authTUIWasCanceled(err error) bool {
	return errors.Is(err, huh.ErrUserAborted)
}
