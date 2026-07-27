package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
	"github.com/intsig-textin/xparse-skills/cli/internal/exitcode"
)

type authMethod string

const (
	authMethodAppKey authMethod = "app-key"
	authMethodOAuth  authMethod = "oauth"
)

type parseAuthSelection struct {
	IsFree bool
	Method authMethod
	Source string
}

func selectParseAuthentication(cmd *cobra.Command, apiMode APIMode, methodFlag string, appKey *config.CredentialSource, cfg *config.Config) (parseAuthSelection, error) {
	method, source, err := configuredAuthMethod(methodFlag, cfg)
	if err != nil {
		return parseAuthSelection{}, err
	}
	if apiMode == APIModeFree {
		if method != "" {
			return parseAuthSelection{}, usageErr("--api free cannot be combined with an authentication method",
				"[fix] remove --auth-method/XPARSE_AUTH_METHOD/default_auth_method or use --api paid")
		}
		return parseAuthSelection{IsFree: true}, nil
	}
	if method == authMethodOAuth && explicitAppKeyFlags(cmd) {
		return parseAuthSelection{}, usageErr("--auth-method oauth cannot be combined with --app-id or --secret-code",
			"[fix] remove the AppKey flags when using OAuth")
	}
	if method == authMethodAppKey {
		if appKey.AppID == "" || appKey.SecretCode == "" {
			return parseAuthSelection{}, usageErr(exitcode.ErrPaidNoCreds,
				"[ask human] run xparse-cli auth app-key or configure a complete AppKey pair")
		}
		return parseAuthSelection{Method: method, Source: source}, nil
	}
	if method == authMethodOAuth {
		if !hasOAuthSession(oauthNow()) {
			return parseAuthSelection{}, usageErr("OAuth login required",
				"[ask human] run xparse-cli auth device or xparse-cli auth browser")
		}
		return parseAuthSelection{Method: method, Source: source}, nil
	}

	// Legacy auto selection remains AppKey-first when no method is configured.
	if appKey.AppID != "" && appKey.SecretCode != "" {
		return parseAuthSelection{Method: authMethodAppKey, Source: "auto"}, nil
	}
	if hasOAuthSession(oauthNow()) {
		return parseAuthSelection{Method: authMethodOAuth, Source: "auto"}, nil
	}
	if apiMode == APIModePaid {
		return parseAuthSelection{}, usageErr(exitcode.ErrPaidNoCreds,
			"[ask human] run xparse-cli auth app-key, auth device, or auth browser")
	}
	return parseAuthSelection{IsFree: true}, nil
}

func configuredAuthMethod(flagValue string, cfg *config.Config) (authMethod, string, error) {
	if value := normalizeAuthMethod(flagValue); value != "" {
		method, err := validateAuthMethod(value)
		return method, "flag", err
	}
	if value := normalizeAuthMethod(os.Getenv("XPARSE_AUTH_METHOD")); value != "" {
		method, err := validateAuthMethod(value)
		return method, "env", err
	}
	if cfg != nil {
		if value := normalizeAuthMethod(cfg.DefaultAuthMethod); value != "" {
			method, err := validateAuthMethod(value)
			return method, "config", err
		}
	}
	return "", "", nil
}

func validateAuthMethod(value string) (authMethod, error) {
	switch authMethod(strings.ToLower(value)) {
	case authMethodAppKey:
		return authMethodAppKey, nil
	case authMethodOAuth:
		return authMethodOAuth, nil
	default:
		return "", usageErr("invalid --auth-method value, must be app-key or oauth",
			"[fix] use --auth-method app-key or --auth-method oauth")
	}
}
