package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyAuthWithoutSubcommand(t *testing.T) {
	home := t.TempDir()
	result := runCLIHelper(t, home, "auth", "new-app\nnew-secret\n")
	if result.err != nil {
		t.Fatalf("auth failed: %v\nstdout:\n%s\nstderr:\n%s", result.err, result.stdout, result.stderr)
	}
	data, err := os.ReadFile(filepath.Join(home, ".xparse-cli", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "app_id: new-app") || !strings.Contains(text, "secret_code: new-secret") {
		t.Fatalf("legacy auth did not save AppKey fields:\n%s", text)
	}
}

func TestLegacyAuthShowMasksSecret(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".xparse-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("app_id: app-12345678\nsecret_code: secret-12345678\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := runCLIHelper(t, home, "auth --show", "")
	if result.err != nil {
		t.Fatalf("auth --show failed: %v\nstderr:\n%s", result.err, result.stderr)
	}
	if strings.Contains(result.stdout, "secret-12345678") || !strings.Contains(result.stdout, "secr...5678") {
		t.Fatalf("auth --show masking changed:\n%s", result.stdout)
	}
}

func TestConfigResetDoesNotDeleteOAuthCredential(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".xparse-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tokenPath := filepath.Join(configDir, "oauth-token.json")
	if err := os.WriteFile(tokenPath, []byte(`{"access_token":"access","refresh_token":"refresh"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	result := runCLIHelper(t, home, "config reset", "")
	if result.err != nil {
		t.Fatalf("config reset failed: %v\nstderr:\n%s", result.err, result.stderr)
	}
	if _, err := os.Stat(tokenPath); err != nil {
		t.Fatalf("config reset removed OAuth credential: %v", err)
	}
}

func TestCLIHelper(t *testing.T) {
	if os.Getenv("XPARSE_CLI_HELPER") != "1" {
		return
	}
	os.Args = append([]string{"xparse-cli"}, strings.Fields(os.Getenv("XPARSE_CLI_ARGS"))...)
	if err := Execute(); err != nil {
		if coded, ok := err.(interface{ ExitCode() int }); ok {
			os.Exit(coded.ExitCode())
		}
		os.Exit(1)
	}
	os.Exit(0)
}

type cliHelperResult struct {
	stdout string
	stderr string
	err    error
}

func runCLIHelper(t *testing.T, home, args, stdin string) cliHelperResult {
	return runCLIHelperEnv(t, home, args, stdin, nil)
}

func runCLIHelperEnv(t *testing.T, home, args, stdin string, environment map[string]string) cliHelperResult {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=TestCLIHelper")
	command.Env = append(os.Environ(),
		"XPARSE_CLI_HELPER=1",
		"XPARSE_CLI_ARGS="+args,
		"HOME="+home,
		"XPARSE_CONFIG_DIR=",
		"XPARSE_APP_ID=",
		"XPARSE_SECRET_CODE=",
	)
	for key, value := range environment {
		command.Env = append(command.Env, key+"="+value)
	}
	command.Stdin = strings.NewReader(stdin)
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return cliHelperResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}
