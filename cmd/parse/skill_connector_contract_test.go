package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const expectedXParseLogoSHA256 = "2f5159dd77b1d4625d44ab5ac30a4b40417be1280083ae4c86ea674d485c4234"

type connectorCLIContract struct {
	Init            map[string]string `json:"init"`
	Auth            map[string]string `json:"auth"`
	UnAuth          map[string]string `json:"unAuth"`
	Status          map[string]string `json:"status"`
	StatusMatchJSON map[string]any    `json:"statusMatchJson"`
	AuthURLDomain   string            `json:"authUrlDomain"`
	Env             map[string]string `json:"env"`
	VersionCheck    struct {
		Command        map[string]string `json:"command"`
		MinVersion     string            `json:"minVersion"`
		VersionPattern string            `json:"versionPattern"`
	} `json:"versionCheck"`
	AuthDeviceFlow struct {
		URIPattern              string `json:"uriPattern"`
		CodePattern             string `json:"codePattern"`
		DefaultExpiresInSeconds int    `json:"defaultExpiresInSeconds"`
		CodeEmbeddedInURI       bool   `json:"codeEmbeddedInUri"`
	} `json:"authDeviceFlow"`
}

func TestWorkBuddyConnectorCommandAndExtractionContract(t *testing.T) {
	data := readRepositoryFile(t, "connector", "cli.json")
	if bytes.Contains(data, []byte("textin-sandbox.intsig.com")) {
		t.Fatal("formal Connector contains the sandbox domain")
	}
	if bytes.Contains(data, []byte("authQrModal")) ||
		bytes.Contains(data, []byte("authSuppressBrowser")) {
		t.Fatal("formal Connector contains a Device Flow-incompatible auth mode")
	}

	var contract connectorCLIContract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"darwin", "linux", "win32"} {
		if contract.Init[platform] == "" ||
			contract.Auth[platform] == "" ||
			contract.UnAuth[platform] == "" ||
			contract.Status[platform] == "" ||
			contract.VersionCheck.Command[platform] == "" {
			t.Fatalf("platform %q has an incomplete command contract", platform)
		}
		if !strings.Contains(contract.Auth[platform],
			"auth device --open-browser=always --output=jsonl") {
			t.Fatalf("platform %q auth command is not native Device Flow: %q",
				platform, contract.Auth[platform])
		}
		for _, command := range []string{
			contract.Auth[platform],
			contract.UnAuth[platform],
			contract.Status[platform],
		} {
			if !strings.Contains(command, "--profile workbuddy") {
				t.Fatalf("platform %q lifecycle command does not use the WorkBuddy profile: %q",
					platform, command)
			}
		}
	}
	for _, command := range []string{
		contract.Auth["win32"],
		contract.UnAuth["win32"],
		contract.Status["win32"],
		contract.VersionCheck.Command["win32"],
	} {
		if !strings.Contains(command, `%USERPROFILE%\.xparse-cli\bin\xparse-cli.exe`) {
			t.Fatalf("Windows command does not use the installer destination: %q", command)
		}
	}
	if contract.AuthURLDomain != "api.textin.com" {
		t.Fatalf("authUrlDomain = %q", contract.AuthURLDomain)
	}
	if contract.Env["XPARSE_OAUTH_CLIENT_ID"] != "cli_textin_xparse" {
		t.Fatalf("Connector public client ID = %q", contract.Env["XPARSE_OAUTH_CLIENT_ID"])
	}
	if contract.Env["XPARSE_CONFIG_DIR"] != "" ||
		contract.Env["XPARSE_CLIENT_FROM"] != "" {
		t.Fatalf("Connector lifecycle must not rely on task env propagation: %#v", contract.Env)
	}
	if contract.StatusMatchJSON["logged_in"] != "true" ||
		contract.StatusMatchJSON["method"] != "oauth" {
		t.Fatalf("statusMatchJson = %#v", contract.StatusMatchJSON)
	}
	workBuddyStatusMatches := func(output string) bool {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			return false
		}
		for key, expected := range contract.StatusMatchJSON {
			if value := parsed[key]; value == nil ||
				toWorkBuddyString(value) != toWorkBuddyString(expected) {
				return false
			}
		}
		return true
	}
	for _, testCase := range []struct {
		name   string
		output string
		want   bool
	}{
		{name: "oauth", output: `{"logged_in":true,"method":"oauth"}`, want: true},
		{name: "app-key", output: `{"logged_in":true,"method":"app-key"}`, want: false},
		{name: "logged-out", output: `{"logged_in":false}`, want: false},
		{name: "corrupt", output: `{`, want: false},
	} {
		if got := workBuddyStatusMatches(testCase.output); got != testCase.want {
			t.Fatalf("%s status match = %v, want %v", testCase.name, got, testCase.want)
		}
	}

	versionPattern := regexp.MustCompile(contract.VersionCheck.VersionPattern)
	if !versionPattern.MatchString("xparse-cli version 2.1.0") ||
		contract.VersionCheck.MinVersion != "2.1.0" {
		t.Fatalf("invalid version contract: %#v", contract.VersionCheck)
	}
	for _, legacy := range []string{"1.0.4", "2.0.3"} {
		legacyMatch := versionPattern.FindStringSubmatch("xparse-cli version v" + legacy)
		if len(legacyMatch) != 2 || legacyMatch[1] != legacy {
			t.Fatalf("legacy version %s extraction = %#v", legacy, legacyMatch)
		}
	}

	event := `{"type":"device_authorization","verification_uri":"https://api.textin.com/oauth21/device","verification_uri_complete":"https://api.textin.com/oauth21/device?user_code=ABCD-EFGH","user_code":"ABCD-EFGH","expires_in":240,"interval":5}`
	uriMatch := regexp.MustCompile(contract.AuthDeviceFlow.URIPattern).
		FindStringSubmatch(event)
	if len(uriMatch) != 2 ||
		uriMatch[1] != "https://api.textin.com/oauth21/device?user_code=ABCD-EFGH" {
		t.Fatalf("uri match = %#v", uriMatch)
	}
	codeMatch := regexp.MustCompile(contract.AuthDeviceFlow.CodePattern).
		FindStringSubmatch(event)
	if len(codeMatch) != 2 || codeMatch[1] != "ABCD-EFGH" {
		t.Fatalf("code match = %#v", codeMatch)
	}
	if contract.AuthDeviceFlow.DefaultExpiresInSeconds != 240 ||
		!contract.AuthDeviceFlow.CodeEmbeddedInURI {
		t.Fatalf("authDeviceFlow = %#v", contract.AuthDeviceFlow)
	}
}

func TestWorkBuddyTestConnectorIsPinnedAndSandboxOnly(t *testing.T) {
	data := readRepositoryFile(t, "connector", "cli.test.json")
	var contract connectorCLIContract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"darwin", "linux", "win32"} {
		initCommand := contract.Init[platform]
		if !strings.Contains(initCommand, "/v2.1.0-workbuddy-test.1/") ||
			!strings.Contains(initCommand, "textin-sandbox.intsig.com") ||
			!strings.Contains(initCommand, "--profile workbuddy config set base_url") {
			t.Fatalf("platform %q test init is not pinned to the sandbox profile: %q",
				platform, initCommand)
		}
		if strings.Contains(initCommand, "/latest/") {
			t.Fatalf("platform %q test init uses the rolling release directory: %q",
				platform, initCommand)
		}
	}
	if contract.AuthURLDomain != "textin-sandbox.intsig.com" {
		t.Fatalf("test authUrlDomain = %q", contract.AuthURLDomain)
	}
	if contract.Env["XPARSE_OAUTH_CLIENT_ID"] != "cli_textin_xparse_workbuddy" {
		t.Fatalf("test Connector public client ID = %q",
			contract.Env["XPARSE_OAUTH_CLIENT_ID"])
	}

	publishScript := string(readRepositoryFile(t, "cli", "publish-version.sh"))
	if !strings.Contains(publishScript, `BASE_PATH="xparse-cli/${VERSION}"`) {
		t.Fatal("isolated publisher does not target the requested version directory")
	}
	if strings.Contains(publishScript, `"${BASE_PATH}/latest/`) ||
		strings.Contains(publishScript, `"xparse-cli/latest/`) {
		t.Fatal("isolated publisher contains a rolling release upload destination")
	}
	for _, artifact := range []string{
		`"${BASE_PATH}/workbuddy-cli.json"`,
		`"${BASE_PATH}/enable-workbuddy-test.sh"`,
		`"${BASE_PATH}/restore-workbuddy-production.sh"`,
		`"${BASE_PATH}/enable-workbuddy-test.ps1"`,
		`"${BASE_PATH}/restore-workbuddy-production.ps1"`,
	} {
		if !strings.Contains(publishScript, artifact) {
			t.Fatalf("isolated publisher does not upload %s", artifact)
		}
	}
}

func TestWorkBuddyMacOSTestSwitchPreservesProductionState(t *testing.T) {
	tempDir := t.TempDir()
	connectorDir := filepath.Join(tempDir, "connector")
	profileDir := filepath.Join(tempDir, "profiles", "workbuddy")
	if err := os.MkdirAll(connectorDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}

	productionConnector := readRepositoryFile(t, "connector", "cli.json")
	testConnectorPath := repositoryPath(t, "connector", "cli.test.json")
	connectorPath := filepath.Join(connectorDir, "cli.json")
	if err := os.WriteFile(connectorPath, productionConnector, 0o600); err != nil {
		t.Fatal(err)
	}
	productionProfile := []byte("base_url: https://api.textin.com\n")
	if err := os.WriteFile(
		filepath.Join(profileDir, "config.yaml"),
		productionProfile,
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	productionToken := []byte(`{"client_id":"cli_textin_xparse","access_token":"secret"}`)
	if err := os.WriteFile(
		filepath.Join(profileDir, "oauth-token.json"),
		productionToken,
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	runWorkBuddyScript(t, "enable-workbuddy-test.sh", connectorDir, profileDir,
		"XPARSE_TEST_CONNECTOR_FILE="+testConnectorPath)
	assertFileContains(t, connectorPath, "cli_textin_xparse_workbuddy")
	assertFileContains(t, connectorPath, "textin-sandbox.intsig.com")
	assertFileContains(t, connectorPath+".production.bak", `"cli_textin_xparse"`)
	assertFileContent(t, filepath.Join(profileDir+".production.bak", "config.yaml"),
		productionProfile)
	assertFileContent(t, filepath.Join(profileDir+".production.bak", "oauth-token.json"),
		productionToken)
	if _, err := os.Stat(profileDir); !os.IsNotExist(err) {
		t.Fatalf("test switch kept the production profile active: %v", err)
	}

	runWorkBuddyScript(t, "enable-workbuddy-test.sh", connectorDir, profileDir,
		"XPARSE_TEST_CONNECTOR_FILE="+testConnectorPath)
	assertFileContent(t, filepath.Join(profileDir+".production.bak", "oauth-token.json"),
		productionToken)

	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	testToken := []byte(`{"client_id":"cli_textin_xparse_workbuddy","access_token":"test"}`)
	if err := os.WriteFile(
		filepath.Join(profileDir, "oauth-token.json"),
		testToken,
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	runWorkBuddyScript(t, "restore-workbuddy-production.sh", connectorDir, profileDir)
	assertFileContains(t, connectorPath, `"cli_textin_xparse"`)
	assertFileContent(t, filepath.Join(profileDir, "config.yaml"), productionProfile)
	assertFileContent(t, filepath.Join(profileDir, "oauth-token.json"), productionToken)
	testProfileBackups, err := filepath.Glob(profileDir + ".test.*.bak")
	if err != nil || len(testProfileBackups) != 1 {
		t.Fatalf("test profile backups = %v, err = %v", testProfileBackups, err)
	}
	assertFileContent(t, filepath.Join(testProfileBackups[0], "oauth-token.json"), testToken)
}

func TestWorkBuddyWindowsSwitchScriptsMatchMacOSSafetyContract(t *testing.T) {
	for _, name := range []string{
		"enable-workbuddy-test.ps1",
		"restore-workbuddy-production.ps1",
	} {
		script := string(readRepositoryFile(t, "connector", "test", name))
		for _, expected := range []string{
			"WORKBUDDY_CONNECTOR_DIR",
			"XPARSE_WORKBUDDY_PROFILE_DIR",
			"production.bak",
			"ConvertFrom-Json",
			"Move-Item",
		} {
			if !strings.Contains(script, expected) {
				t.Fatalf("%s does not contain %q", name, expected)
			}
		}
	}
	enableScript := string(readRepositoryFile(
		t,
		"connector",
		"test",
		"enable-workbuddy-test.ps1",
	))
	for _, expected := range []string{
		"v2.1.0-workbuddy-test.1",
		"cli_textin_xparse_workbuddy",
		"textin-sandbox.intsig.com",
		"workbuddy-cli.json",
	} {
		if !strings.Contains(enableScript, expected) {
			t.Fatalf("Windows enable script does not contain %q", expected)
		}
	}
	if strings.Contains(enableScript, "${DownloadBase}/latest/workbuddy-cli.json") ||
		strings.Contains(enableScript, `$env:XPARSER_VERSION } else { "latest" }`) {
		t.Fatal("Windows enable script references the rolling release directory")
	}
}

func runWorkBuddyScript(
	t *testing.T,
	name, connectorDir, profileDir string,
	extraEnv ...string,
) {
	t.Helper()
	command := exec.Command(
		"/bin/sh",
		repositoryPath(t, "connector", "test", name),
	)
	command.Env = append(
		os.Environ(),
		"WORKBUDDY_CONNECTOR_DIR="+connectorDir,
		"XPARSE_WORKBUDDY_PROFILE_DIR="+profileDir,
	)
	command.Env = append(command.Env, extraEnv...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s failed: %v\n%s", name, err, output)
	}
}

func assertFileContains(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(expected)) {
		t.Fatalf("%s does not contain %q", path, expected)
	}
}

func assertFileContent(t *testing.T, path string, expected []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, expected) {
		t.Fatalf("%s = %q, want %q", path, data, expected)
	}
}

func repositoryPath(t *testing.T, pathParts ...string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allParts := append([]string{dir, "..", "..", ".."}, pathParts...)
	return filepath.Clean(filepath.Join(allParts...))
}

func toWorkBuddyString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func TestSkillAndConnectorUseExactTransparentLogo(t *testing.T) {
	for _, pathParts := range [][]string{
		{"skills", "xparse-parse", "assets", "logo.png"},
		{"connector", "icon.png"},
	} {
		data := readRepositoryFile(t, pathParts...)
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != expectedXParseLogoSHA256 {
			t.Fatalf("%s sha256 = %s", filepath.Join(pathParts...), got)
		}
		image, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode %s: %v", filepath.Join(pathParts...), err)
		}
		bounds := image.Bounds()
		if bounds.Dx() != 228 || bounds.Dy() != 228 {
			t.Fatalf("%s bounds = %v", filepath.Join(pathParts...), bounds)
		}
		_, _, _, alpha := image.At(bounds.Min.X, bounds.Min.Y).RGBA()
		if alpha != 0 {
			t.Fatalf("%s does not preserve transparent corner", filepath.Join(pathParts...))
		}
	}
}

func TestSkillUsesFormalCLIWithoutCredentialCollection(t *testing.T) {
	skill := string(readRepositoryFile(t, "skills", "xparse-parse", "SKILL.md"))
	if !strings.Contains(skill, "xparse-cli parse") ||
		!strings.Contains(skill, "references/authentication.md") {
		t.Fatal("Skill does not route parsing and authentication to the formal CLI")
	}
	if !strings.Contains(skill, "xparse-cli --profile workbuddy parse") {
		t.Fatal("WorkBuddy Skill does not select the isolated WorkBuddy profile")
	}
	if !strings.Contains(skill, "xparse-cli --profile workbuddy parse <INPUT> --api free") ||
		!strings.Contains(skill, "Use `--api paid` only when the user explicitly asks") {
		t.Fatal("WorkBuddy Skill does not default parsing to the free API")
	}
	docTools := string(readRepositoryFile(t, "skills", "xparse-doc-tools", "SKILL.md"))
	if !strings.Contains(docTools, "xparse-cli --profile workbuddy") {
		t.Fatal("WorkBuddy document tools do not select the isolated WorkBuddy profile")
	}
	for _, forbidden := range []string{
		"curl -H \"Authorization:",
		"cat ~/.xparse-cli/oauth-token.json",
		"print the access token",
	} {
		if strings.Contains(strings.ToLower(skill), strings.ToLower(forbidden)) {
			t.Fatalf("Skill contains forbidden credential handling: %q", forbidden)
		}
	}
}

func readRepositoryFile(t *testing.T, pathParts ...string) []byte {
	t.Helper()
	parts := append([]string{"..", "..", ".."}, pathParts...)
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
