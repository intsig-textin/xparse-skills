package cmd

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"io"
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
		if !strings.Contains(contract.Init[platform], "/v2.2.0/") ||
			!strings.Contains(contract.Init[platform],
				"--profile workbuddy config set base_url https://api.textin.com") {
			t.Fatalf("platform %q production init is not pinned to 2.2.0: %q",
				platform, contract.Init[platform])
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
	if contract.Env["XPARSE_OAUTH_CLIENT_ID"] != "cli_textin_xparse_workbuddy" {
		t.Fatalf("Connector public client ID = %q", contract.Env["XPARSE_OAUTH_CLIENT_ID"])
	}
	if contract.Env["XPARSE_BASE_URL"] != "https://api.textin.com" {
		t.Fatalf("Connector base URL = %q", contract.Env["XPARSE_BASE_URL"])
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
	if !versionPattern.MatchString("xparse-cli version 2.2.0") ||
		contract.VersionCheck.MinVersion != "2.2.0" {
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
		if !strings.Contains(initCommand, "/v2.2.0-workbuddy-test.3/") ||
			!strings.Contains(initCommand, "textin-sandbox.intsig.com") ||
			!strings.Contains(initCommand, "--profile workbuddy config set base_url") {
			t.Fatalf("platform %q test init is not pinned to sandbox: %q",
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
	for _, releaseContract := range []string{
		`RELEASE_KIND="${2:-test}"`,
		`CONNECTOR_CONFIG="${CONNECTOR_DIR}/cli.json"`,
		`CONNECTOR_CONFIG="${CONNECTOR_DIR}/cli.pre.json"`,
		`CONNECTOR_CONFIG="${CONNECTOR_DIR}/cli.test.json"`,
		`upload "${PINNED_INSTALL_SH}"`,
		`upload "${PINNED_INSTALL_PS1}"`,
		`upload "${PINNED_ENABLE_SH}"`,
		`upload "${PINNED_ENABLE_PS1}"`,
	} {
		if !strings.Contains(publishScript, releaseContract) {
			t.Fatalf("isolated publisher is missing release contract %q", releaseContract)
		}
	}
	if strings.Contains(publishScript, `"${BASE_PATH}/latest/`) ||
		strings.Contains(publishScript, `"xparse-cli/latest/`) {
		t.Fatal("isolated publisher contains a rolling release upload destination")
	}
	for _, artifact := range []string{
		`"${BASE_PATH}/workbuddy-cli.json"`,
		`"${BASE_PATH}/workbuddy-connector-meta.json"`,
		`"${BASE_PATH}/workbuddy-icon.png"`,
		`"${BASE_PATH}/workbuddy-marketplace-entry.json"`,
		`"${BASE_PATH}/workbuddy-xparse-parse.zip"`,
		`"${BASE_PATH}/enable-workbuddy-test.sh"`,
		`"${BASE_PATH}/restore-workbuddy-production.sh"`,
		`"${BASE_PATH}/enable-workbuddy-test.ps1"`,
		`"${BASE_PATH}/enable-workbuddy.ps1"`,
		`"${BASE_PATH}/restore-workbuddy-production.ps1"`,
	} {
		if !strings.Contains(publishScript, artifact) {
			t.Fatalf("isolated publisher does not upload %s", artifact)
		}
	}
	for _, contentType := range []string{
		`"text/plain; charset=utf-8"`,
		`"application/json; charset=utf-8"`,
	} {
		if !strings.Contains(publishScript, contentType) {
			t.Fatalf("isolated publisher does not set %s", contentType)
		}
	}
	uploadScript := string(readRepositoryFile(t, "cli", "upload.sh"))
	if !strings.Contains(uploadScript, "--content-type=${content_type}") {
		t.Fatal("uploader does not forward the explicit UTF-8 content type to azcopy")
	}
}

func TestWorkBuddyPreConnectorIsPinnedToPreOnly(t *testing.T) {
	data := readRepositoryFile(t, "connector", "cli.pre.json")
	var contract connectorCLIContract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"darwin", "linux", "win32"} {
		initCommand := contract.Init[platform]
		if !strings.Contains(initCommand, "/v2.2.0-workbuddy-pre.2/") ||
			!strings.Contains(initCommand, "textin-api-pre.intsig.com") ||
			!strings.Contains(initCommand, "--profile workbuddy config set base_url") {
			t.Fatalf("platform %q pre init is not pinned to pre: %q",
				platform, initCommand)
		}
		if strings.Contains(initCommand, "textin-sandbox.intsig.com") ||
			strings.Contains(initCommand, "/latest/") {
			t.Fatalf("platform %q pre init references the wrong environment: %q",
				platform, initCommand)
		}
	}
	if contract.AuthURLDomain != "textin-api-pre.intsig.com" {
		t.Fatalf("pre authUrlDomain = %q", contract.AuthURLDomain)
	}
	if contract.Env["XPARSE_OAUTH_CLIENT_ID"] != "cli_textin_xparse_workbuddy" {
		t.Fatalf("pre Connector public client ID = %q",
			contract.Env["XPARSE_OAUTH_CLIENT_ID"])
	}
}

func TestWorkBuddyMacOSTestInjectionRestoresUninstalledBaseline(t *testing.T) {
	tempDir := t.TempDir()
	marketplaceRoot := filepath.Join(tempDir, "marketplace")
	catalogDir := filepath.Join(marketplaceRoot, ".codebuddy-connector")
	connectorsDir := filepath.Join(marketplaceRoot, "connectors")
	connectorDir := filepath.Join(connectorsDir, "textin-xparse")
	marketplaceIcon := filepath.Join(marketplaceRoot, "icons", "textin-xparse.png")
	profileDir := filepath.Join(tempDir, "profiles", "workbuddy")
	cliPath := filepath.Join(tempDir, "bin", "xparse-cli")
	assetDir := filepath.Join(tempDir, "assets")
	testBackupRoot := filepath.Join(tempDir, "test-backups")
	activeSkillsDir := filepath.Join(tempDir, "activated-skills", "connector-textin-xparse")
	for _, dir := range []string{
		catalogDir,
		connectorsDir,
		filepath.Dir(marketplaceIcon),
		profileDir,
		filepath.Dir(cliPath),
		assetDir,
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	productionCatalog := []byte(`{
  "tokenProviders": [],
  "connectors": [
    {
      "id": "existing",
      "name": "Existing Connector"
    }
  ]
}
`)
	catalogPath := filepath.Join(catalogDir, "connectors.json")
	if err := os.WriteFile(catalogPath, productionCatalog, 0o600); err != nil {
		t.Fatal(err)
	}
	writeWorkBuddyTestAssets(t, assetDir)
	productionMarketplaceIcon := []byte("production-marketplace-icon")
	if err := os.WriteFile(marketplaceIcon, productionMarketplaceIcon, 0o600); err != nil {
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
	productionCLI := []byte("production-cli")
	if err := os.WriteFile(
		cliPath,
		productionCLI,
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	productionActivatedSkill := []byte("production-activated-skill")
	if err := os.MkdirAll(filepath.Join(activeSkillsDir, "legacy-skill"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(activeSkillsDir, "legacy-skill", "SKILL.md"),
		productionActivatedSkill,
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	runWorkBuddyScript(
		t,
		"enable-workbuddy-test.sh",
		marketplaceRoot,
		profileDir,
		cliPath,
		assetDir,
		testBackupRoot,
		activeSkillsDir,
	)
	assertFileContains(t, filepath.Join(connectorDir, "cli.json"),
		"cli_textin_xparse_workbuddy")
	assertFileContains(t, filepath.Join(connectorDir, "cli.json"),
		"textin-sandbox.intsig.com")
	assertFileContains(t, filepath.Join(connectorDir, "connector-meta.json"),
		`"source": "textin-xparse"`)
	assertFileContains(t, filepath.Join(connectorDir, ".workbuddy-test"),
		"v2.2.0-workbuddy-test.3")
	testIcon := readRepositoryFile(t, "connector", "icon.png")
	assertFileContent(t, filepath.Join(connectorDir, "icon.png"), testIcon)
	assertFileContent(t, marketplaceIcon, testIcon)
	assertFileContent(t, marketplaceIcon+".production.bak", productionMarketplaceIcon)
	assertFileContains(t, filepath.Join(connectorDir, "skills", "xparse-parse", "SKILL.md"),
		"name: xparse-parse")
	assertFileContains(t, filepath.Join(activeSkillsDir, "xparse-parse", "SKILL.md"),
		"name: xparse-parse")
	if _, err := os.Stat(filepath.Join(connectorDir, "skills", "xparse-doc-tools")); !os.IsNotExist(err) {
		t.Fatalf("test Connector unexpectedly includes xparse-doc-tools: %v", err)
	}
	assertCatalogConnectorCount(t, catalogPath, "textin-xparse", 1)
	assertFileContent(t, catalogPath+".textin-xparse.production.bak", productionCatalog)
	assertFileContent(t, filepath.Join(profileDir+".production.bak", "config.yaml"),
		productionProfile)
	assertFileContent(t, filepath.Join(profileDir+".production.bak", "oauth-token.json"),
		productionToken)
	assertFileContent(t, cliPath+".production.bak", productionCLI)
	assertFileContent(
		t,
		filepath.Join(activeSkillsDir+".production.bak", "legacy-skill", "SKILL.md"),
		productionActivatedSkill,
	)
	if _, err := os.Stat(profileDir); !os.IsNotExist(err) {
		t.Fatalf("test switch kept the production profile active: %v", err)
	}
	if _, err := os.Stat(cliPath); !os.IsNotExist(err) {
		t.Fatalf("test switch kept the production CLI active: %v", err)
	}

	runWorkBuddyScript(
		t,
		"enable-workbuddy-test.sh",
		marketplaceRoot,
		profileDir,
		cliPath,
		assetDir,
		testBackupRoot,
		activeSkillsDir,
	)
	assertFileContent(t, filepath.Join(profileDir+".production.bak", "oauth-token.json"),
		productionToken)
	assertFileContent(t, cliPath+".production.bak", productionCLI)
	assertCatalogConnectorCount(t, catalogPath, "textin-xparse", 1)
	assertFileContent(t, marketplaceIcon, testIcon)

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
	testCLI := []byte("test-cli")
	if err := os.WriteFile(cliPath, testCLI, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(activeSkillsDir, "xparse-parse"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(activeSkillsDir, "xparse-parse", "SKILL.md"),
		[]byte("test-activated-skill"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	runWorkBuddyScript(
		t,
		"restore-workbuddy-production.sh",
		marketplaceRoot,
		profileDir,
		cliPath,
		assetDir,
		testBackupRoot,
		activeSkillsDir,
	)
	assertFileContent(t, catalogPath, productionCatalog)
	assertCatalogConnectorCount(t, catalogPath, "textin-xparse", 0)
	if _, err := os.Stat(connectorDir); !os.IsNotExist(err) {
		t.Fatalf("restore kept an injected Connector active: %v", err)
	}
	assertFileContent(t, filepath.Join(profileDir, "config.yaml"), productionProfile)
	assertFileContent(t, filepath.Join(profileDir, "oauth-token.json"), productionToken)
	assertFileContent(t, cliPath, productionCLI)
	assertFileContent(t, marketplaceIcon, productionMarketplaceIcon)
	testProfileBackups, err := filepath.Glob(profileDir + ".test.*.bak")
	if err != nil || len(testProfileBackups) != 1 {
		t.Fatalf("test profile backups = %v, err = %v", testProfileBackups, err)
	}
	assertFileContent(t, filepath.Join(testProfileBackups[0], "oauth-token.json"), testToken)
	testCLIBackups, err := filepath.Glob(cliPath + ".test.*.bak")
	if err != nil || len(testCLIBackups) != 1 {
		t.Fatalf("test CLI backups = %v, err = %v", testCLIBackups, err)
	}
	assertFileContent(t, testCLIBackups[0], testCLI)
	testConnectorBackups, err := filepath.Glob(filepath.Join(testBackupRoot, "connector.*"))
	if err != nil || len(testConnectorBackups) != 1 {
		t.Fatalf("test Connector backups = %v, err = %v", testConnectorBackups, err)
	}
	assertFileContains(t, filepath.Join(testConnectorBackups[0], "cli.json"),
		"cli_textin_xparse_workbuddy")
	testMarketplaceIconBackups, err := filepath.Glob(
		filepath.Join(testBackupRoot, "marketplace-icon.*.png"),
	)
	if err != nil || len(testMarketplaceIconBackups) != 1 {
		t.Fatalf("marketplace icon backups = %v, err = %v",
			testMarketplaceIconBackups, err)
	}
	assertFileContent(t, testMarketplaceIconBackups[0], testIcon)
	assertFileContent(
		t,
		filepath.Join(activeSkillsDir, "legacy-skill", "SKILL.md"),
		productionActivatedSkill,
	)
	if _, err := os.Stat(filepath.Join(activeSkillsDir, "xparse-parse")); !os.IsNotExist(err) {
		t.Fatalf("restore kept xparse-parse active over the original Skill state: %v", err)
	}
	testActiveSkillBackups, err := filepath.Glob(
		filepath.Join(testBackupRoot, "activated-skills.*"),
	)
	if err != nil || len(testActiveSkillBackups) != 1 {
		t.Fatalf("activated Skill backups = %v, err = %v", testActiveSkillBackups, err)
	}
	assertFileContent(
		t,
		filepath.Join(testActiveSkillBackups[0], "xparse-parse", "SKILL.md"),
		[]byte("test-activated-skill"),
	)
}

func TestWorkBuddyLegacyRefreshPreservesActivatedSkillForRestore(t *testing.T) {
	tempDir := t.TempDir()
	marketplaceRoot := filepath.Join(tempDir, "marketplace")
	catalogDir := filepath.Join(marketplaceRoot, ".codebuddy-connector")
	connectorDir := filepath.Join(marketplaceRoot, "connectors", "textin-xparse")
	marketplaceIcon := filepath.Join(marketplaceRoot, "icons", "textin-xparse.png")
	assetDir := filepath.Join(tempDir, "assets")
	testBackupRoot := filepath.Join(tempDir, "test-backups")
	activeSkillsDir := filepath.Join(tempDir, "activated-skills", "connector-textin-xparse")
	profileDir := filepath.Join(tempDir, "profiles", "workbuddy")
	cliPath := filepath.Join(tempDir, "bin", "xparse-cli")
	for _, dir := range []string{
		catalogDir,
		connectorDir,
		assetDir,
		filepath.Join(activeSkillsDir, "legacy-skill"),
		filepath.Dir(marketplaceIcon),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	productionCatalog := []byte("{\"connectors\":[]}\n")
	testCatalog := []byte("{\"connectors\":[{\"id\":\"textin-xparse\"}]}\n")
	catalogPath := filepath.Join(catalogDir, "connectors.json")
	if err := os.WriteFile(catalogPath, testCatalog, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		catalogPath+".textin-xparse.production.bak",
		productionCatalog,
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(connectorDir, ".workbuddy-test"),
		[]byte("v2.1.0-workbuddy-test.3\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	legacySkill := []byte("legacy-activated-skill")
	if err := os.WriteFile(
		filepath.Join(activeSkillsDir, "legacy-skill", "SKILL.md"),
		legacySkill,
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	legacyMarketplaceIcon := []byte("legacy-marketplace-icon")
	if err := os.WriteFile(marketplaceIcon, legacyMarketplaceIcon, 0o600); err != nil {
		t.Fatal(err)
	}
	writeWorkBuddyTestAssets(t, assetDir)

	runWorkBuddyScript(
		t,
		"enable-workbuddy-test.sh",
		marketplaceRoot,
		profileDir,
		cliPath,
		assetDir,
		testBackupRoot,
		activeSkillsDir,
	)
	assertFileContent(
		t,
		filepath.Join(activeSkillsDir+".production.bak", "legacy-skill", "SKILL.md"),
		legacySkill,
	)
	assertFileContains(t, filepath.Join(activeSkillsDir, "xparse-parse", "SKILL.md"),
		"name: xparse-parse")
	assertFileContains(t, filepath.Join(connectorDir, ".workbuddy-test"),
		"v2.2.0-workbuddy-test.3")
	assertFileContent(t, marketplaceIcon+".production.bak", legacyMarketplaceIcon)
	assertFileContent(t, marketplaceIcon, readRepositoryFile(t, "connector", "icon.png"))

	runWorkBuddyScript(
		t,
		"restore-workbuddy-production.sh",
		marketplaceRoot,
		profileDir,
		cliPath,
		assetDir,
		testBackupRoot,
		activeSkillsDir,
	)
	assertFileContent(t, catalogPath, productionCatalog)
	assertFileContent(
		t,
		filepath.Join(activeSkillsDir, "legacy-skill", "SKILL.md"),
		legacySkill,
	)
	assertFileContent(t, marketplaceIcon, legacyMarketplaceIcon)
	if _, err := os.Stat(filepath.Join(activeSkillsDir, "xparse-parse")); !os.IsNotExist(err) {
		t.Fatalf("legacy refresh restore kept xparse-parse active: %v", err)
	}
}

func TestWorkBuddyRestoreRecoversOrphanedFixedBackups(t *testing.T) {
	tempDir := t.TempDir()
	marketplaceRoot := filepath.Join(tempDir, "marketplace")
	profileDir := filepath.Join(tempDir, "profiles", "workbuddy")
	profileBackup := profileDir + ".production.bak"
	cliPath := filepath.Join(tempDir, "bin", "xparse-cli")
	activeSkillsDir := filepath.Join(tempDir, "activated-skills", "connector-textin-xparse")
	testBackupRoot := filepath.Join(tempDir, "test-backups")
	assetDir := filepath.Join(tempDir, "assets")
	catalogPath := filepath.Join(marketplaceRoot, ".codebuddy-connector", "connectors.json")
	for _, dir := range []string{
		profileDir,
		profileBackup,
		filepath.Dir(cliPath),
		filepath.Join(activeSkillsDir, "xparse-parse"),
		filepath.Join(activeSkillsDir+".production.bak", "legacy-skill"),
		filepath.Dir(catalogPath),
		assetDir,
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	testConfig := []byte("base_url: https://textin-sandbox.intsig.com\n")
	productionConfig := []byte("base_url: https://api.textin.com\n")
	if err := os.WriteFile(filepath.Join(profileDir, "config.yaml"), testConfig, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileBackup, "config.yaml"), productionConfig, 0o600); err != nil {
		t.Fatal(err)
	}
	testCLI := []byte("test-cli")
	productionCLI := []byte("production-cli")
	if err := os.WriteFile(cliPath, testCLI, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliPath+".production.bak", productionCLI, 0o700); err != nil {
		t.Fatal(err)
	}
	testSkill := []byte("test-skill")
	productionSkill := []byte("production-skill")
	if err := os.WriteFile(
		filepath.Join(activeSkillsDir, "xparse-parse", "SKILL.md"), testSkill, 0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(activeSkillsDir+".production.bak", "legacy-skill", "SKILL.md"),
		productionSkill,
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	productionCatalog := []byte("{\"connectors\":[]}\n")
	if err := os.WriteFile(catalogPath, productionCatalog, 0o600); err != nil {
		t.Fatal(err)
	}
	writeWorkBuddyTestAssets(t, assetDir)

	runWorkBuddyScript(
		t,
		"restore-workbuddy-production.sh",
		marketplaceRoot,
		profileDir,
		cliPath,
		assetDir,
		testBackupRoot,
		activeSkillsDir,
	)
	assertFileContent(t, filepath.Join(profileDir, "config.yaml"), productionConfig)
	assertFileContent(t, cliPath, productionCLI)
	assertFileContent(
		t,
		filepath.Join(activeSkillsDir, "legacy-skill", "SKILL.md"),
		productionSkill,
	)
	for _, backup := range []string{
		profileBackup,
		cliPath + ".production.bak",
		activeSkillsDir + ".production.bak",
	} {
		if _, err := os.Stat(backup); !os.IsNotExist(err) {
			t.Fatalf("orphaned backup remains at %s: %v", backup, err)
		}
	}
	recoveryRoots, err := filepath.Glob(filepath.Join(testBackupRoot, "orphan-recovery.*"))
	if err != nil || len(recoveryRoots) != 1 {
		t.Fatalf("orphan recovery roots = %v, err = %v", recoveryRoots, err)
	}
	assertFileContent(t, filepath.Join(recoveryRoots[0], "profile.current", "config.yaml"), testConfig)
	assertFileContent(t, filepath.Join(recoveryRoots[0], "xparse-cli.current"), testCLI)
	assertFileContent(
		t,
		filepath.Join(recoveryRoots[0], "activated-skills.current", "xparse-parse", "SKILL.md"),
		testSkill,
	)

	runWorkBuddyScript(
		t,
		"enable-workbuddy-test.sh",
		marketplaceRoot,
		profileDir,
		cliPath,
		assetDir,
		testBackupRoot,
		activeSkillsDir,
	)
	assertFileContains(
		t,
		filepath.Join(marketplaceRoot, "connectors", "textin-xparse", ".workbuddy-test"),
		"v2.2.0-workbuddy-test.3",
	)
	assertFileContent(t, filepath.Join(profileDir+".production.bak", "config.yaml"), productionConfig)
	assertFileContent(t, cliPath+".production.bak", productionCLI)
}

func TestWorkBuddyWindowsSwitchScriptsMatchMacOSSafetyContract(t *testing.T) {
	for _, name := range []string{
		"enable-workbuddy-test.ps1",
		"restore-workbuddy-production.ps1",
	} {
		script := string(readRepositoryFile(t, "connector", "test", name))
		if strings.Contains(script, "Get-Content") {
			t.Fatalf("%s still relies on Windows PowerShell's default file encoding", name)
		}
		for _, expected := range []string{
			"WORKBUDDY_MARKETPLACE_ROOT",
			"WORKBUDDY_CONNECTOR_CATALOG",
			"WORKBUDDY_CONNECTOR_DIR",
			"XPARSE_WORKBUDDY_PROFILE_DIR",
			"XPARSE_CLI_PATH",
			"WORKBUDDY_CONNECTOR_SKILLS_DIR",
			"WORKBUDDY_MARKETPLACE_ICONS_DIR",
			"MarketplaceIconBackup",
			"production.bak",
			"ConvertFrom-Json",
			"UTF8Strict",
			"Read-Utf8Json",
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
		"v2.2.0-workbuddy-test.3",
		"cli_textin_xparse_workbuddy",
		"textin-sandbox.intsig.com",
		"workbuddy-cli.json",
		"workbuddy-connector-meta.json",
		"workbuddy-icon.png",
		"workbuddy-marketplace-entry.json",
		"workbuddy-xparse-parse.zip",
		"Expand-Archive",
		"Get-FileHash",
		"ExpectedIconSHA256",
		"install.ps1",
		"XPARSE_EXPECTED_AUTH_DOMAIN",
		"XPARSE_PROFILE_BASE_URL",
		"config set base_url $ProfileBaseURL",
	} {
		if !strings.Contains(enableScript, expected) {
			t.Fatalf("Windows enable script does not contain %q", expected)
		}
	}
	if strings.Contains(enableScript, "${DownloadBase}/latest/workbuddy-cli.json") ||
		strings.Contains(enableScript, `$env:XPARSER_VERSION } else { "latest" }`) {
		t.Fatal("Windows enable script references the rolling release directory")
	}
	restoreScript := string(readRepositoryFile(
		t,
		"connector",
		"test",
		"restore-workbuddy-production.ps1",
	))
	for _, expected := range []string{
		"OrphanRecoveryRoot",
		"Restore-OrphanedBackup",
		"orphan-recovery.",
		"开始自动恢复孤立正式备份",
		"现在可以重新运行 enable 一键安装命令",
	} {
		if !strings.Contains(restoreScript, expected) {
			t.Fatalf("Windows restore script does not contain %q", expected)
		}
	}
	if strings.Contains(restoreScript, "无法自动判断是否应恢复") {
		t.Fatal("Windows restore script still rejects orphaned backups")
	}
}

func runWorkBuddyScript(
	t *testing.T,
	name, marketplaceRoot, profileDir, cliPath, assetDir, testBackupRoot,
	activeSkillsDir string,
) {
	t.Helper()
	command := exec.Command(
		"/bin/sh",
		repositoryPath(t, "connector", "test", name),
	)
	command.Env = append(
		os.Environ(),
		"WORKBUDDY_MARKETPLACE_ROOT="+marketplaceRoot,
		"XPARSE_WORKBUDDY_PROFILE_DIR="+profileDir,
		"XPARSE_CLI_PATH="+cliPath,
		"XPARSE_TEST_ASSET_DIR="+assetDir,
		"WORKBUDDY_TEST_BACKUP_ROOT="+testBackupRoot,
		"WORKBUDDY_CONNECTOR_SKILLS_DIR="+activeSkillsDir,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s failed: %v\n%s", name, err, output)
	}
}

func writeXParseSkillArchive(t *testing.T, destination string) {
	t.Helper()
	output, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(output)
	sourceRoot := repositoryPath(t, "skills", "xparse-parse")
	walkErr := filepath.Walk(sourceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(filepath.Dir(sourceRoot), path)
		if err != nil {
			return err
		}
		writer, err := archive.Create(filepath.ToSlash(relative))
		if err != nil {
			return err
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	closeArchiveErr := archive.Close()
	closeOutputErr := output.Close()
	for _, err := range []error{walkErr, closeArchiveErr, closeOutputErr} {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func writeWorkBuddyTestAssets(t *testing.T, assetDir string) {
	t.Helper()
	for remoteName, repositoryParts := range map[string][]string{
		"workbuddy-cli.json":               {"connector", "cli.test.json"},
		"workbuddy-connector-meta.json":    {"connector", "connector-meta.json"},
		"workbuddy-icon.png":               {"connector", "icon.png"},
		"workbuddy-marketplace-entry.json": {"connector", "marketplace-entry.json"},
	} {
		if err := os.WriteFile(
			filepath.Join(assetDir, remoteName),
			readRepositoryFile(t, repositoryParts...),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	writeXParseSkillArchive(t, filepath.Join(assetDir, "workbuddy-xparse-parse.zip"))
}

func assertCatalogConnectorCount(t *testing.T, path, connectorID string, want int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Connectors []struct {
			ID string `json:"id"`
		} `json:"connectors"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	got := 0
	for _, connector := range catalog.Connectors {
		if connector.ID == connectorID {
			got++
		}
	}
	if got != want {
		t.Fatalf("%s connector %q count = %d, want %d", path, connectorID, got, want)
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
	const purchaseURL = "https://www.textin.com/market/chager/pdf_to_markdown"
	if !strings.Contains(skill, "xparse-cli parse") ||
		!strings.Contains(skill, "references/authentication.md") {
		t.Fatal("Skill does not route parsing and authentication to the formal CLI")
	}
	frontmatter := strings.SplitN(skill, "---", 3)
	if len(frontmatter) != 3 || !strings.Contains(frontmatter[1], purchaseURL) {
		t.Fatalf("Skill description does not include the paid purchase URL %s", purchaseURL)
	}
	if !strings.Contains(skill, "xparse-cli --profile workbuddy parse") {
		t.Fatal("WorkBuddy Skill does not select the isolated WorkBuddy profile")
	}
	if !strings.Contains(skill, "xparse-cli --profile workbuddy parse <INPUT> --api free") ||
		!strings.Contains(skill, "Use `--api paid` only when the user explicitly asks") {
		t.Fatal("WorkBuddy Skill does not default parsing to the free API")
	}
	for _, taskContextContract := range []string{
		"xparse_task_context.v1",
		"--task-context <FILE>",
		"only to the first xParse command",
		"do not put the JSON content in shell arguments",
	} {
		if !strings.Contains(skill, taskContextContract) {
			t.Fatalf("WorkBuddy Skill is missing task context contract %q", taskContextContract)
		}
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

	cliGuidance := string(readRepositoryFile(
		t, "skills", "xparse-parse", "references", "cli-guidance.md",
	))
	authentication := string(readRepositoryFile(
		t, "skills", "xparse-parse", "references", "authentication.md",
	))
	errorHandling := string(readRepositoryFile(
		t, "skills", "xparse-parse", "references", "error-handling.md",
	))
	apiReference := string(readRepositoryFile(
		t, "skills", "xparse-parse", "references", "api-reference.md",
	))
	keySetup := string(readRepositoryFile(
		t, "skills", "xparse-parse", "references", "textin-key-setup.md",
	))
	if !strings.Contains(cliGuidance, "Always use the anonymous free API") ||
		!strings.Contains(cliGuidance, "Compatibility alias for `free`") ||
		!strings.Contains(authentication, "authentication alone never changes") &&
			!strings.Contains(authentication, "credentials exist or not") {
		t.Fatal("Skill references do not preserve the default-free API boundary")
	}
	for _, document := range []string{errorHandling, apiReference} {
		if !strings.Contains(document, "explicit") ||
			!strings.Contains(document, "--api paid") {
			t.Fatal("Skill error guidance does not require explicit paid approval")
		}
	}
	for _, document := range []struct {
		name    string
		content string
	}{
		{name: "error handling", content: errorHandling},
		{name: "API reference", content: apiReference},
		{name: "AppKey setup", content: keySetup},
	} {
		if !strings.Contains(document.content, purchaseURL) {
			t.Fatalf("Skill %s does not direct insufficient-balance users to %s", document.name, purchaseURL)
		}
	}
	for _, forbidden := range []string{
		"Auto-select from the requested/available auth method",
		"Rerun the same parse command",
		"IF user HAS configured credentials",
	} {
		if strings.Contains(cliGuidance+authentication+errorHandling+apiReference, forbidden) {
			t.Fatalf("Skill references retain obsolete automatic-paid guidance: %q", forbidden)
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
