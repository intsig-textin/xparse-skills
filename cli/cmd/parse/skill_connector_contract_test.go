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

const (
	expectedXParseLogoSHA256 = "2f5159dd77b1d4625d44ab5ac30a4b40417be1280083ae4c86ea674d485c4234"
	expectedOCRDescription   = "Parse PDFs, images, Word, Excel, PowerPoint, scanned documents, and more into Markdown or structured JSON from the command line. Preserve document hierarchies and complex table structures for high-accuracy, high-performance OCR and document processing tasks. Parse up to 1,000 pages free each day."
	expectedOCRDescriptionZH = "通过命令行将文件（PDF、图片、Word、Excel、PPT、扫描件等）解析为 Markdown 或结构化 JSON，可还原文档目录结构和复杂表格结构，适用于高精度高性能要求的文档处理OCR任务，每日免费1000页。"
)

type connectorCLIContract struct {
	Runtime struct {
		Type    string `json:"type"`
		Version string `json:"version"`
	} `json:"runtime"`
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
	if contract.Runtime.Type != "node" || contract.Runtime.Version != ">=18" {
		t.Fatalf("production runtime = %#v", contract.Runtime)
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
		if !strings.Contains(contract.Init[platform], "npm") ||
			!strings.Contains(contract.Init[platform], "install --global") ||
			!strings.Contains(contract.Init[platform], "xparse-cli@2.2.1-beta.2") ||
			!strings.Contains(contract.Init[platform], "--registry=https://registry.npmmirror.com") ||
			!strings.Contains(contract.Init[platform],
				"--profile workbuddy config set base_url https://api.textin.com") {
			t.Fatalf("platform %q production init is not pinned to npm beta.1: %q",
				platform, contract.Init[platform])
		}
		if strings.Contains(contract.Init[platform], "--prefix") ||
			strings.Contains(contract.Init[platform], "dllf.intsig.net") {
			t.Fatalf("platform %q production init does not use standard global npm: %q", platform, contract.Init[platform])
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
		if !strings.Contains(command, "xparse-cli.cmd") {
			t.Fatalf("Windows command does not use the npm command shim: %q", command)
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
	if !versionPattern.MatchString("xparse-cli version 2.2.1") ||
		contract.VersionCheck.MinVersion != "2.2.1" {
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
	if contract.Runtime.Type != "node" || contract.Runtime.Version != ">=18" {
		t.Fatalf("test runtime = %#v", contract.Runtime)
	}
	for _, platform := range []string{"darwin", "linux", "win32"} {
		initCommand := contract.Init[platform]
		if !strings.Contains(initCommand, "install --global") ||
			!strings.Contains(initCommand, "xparse-cli@2.2.1-beta.2") ||
			!strings.Contains(initCommand,
				"--registry=https://registry.npmmirror.com") ||
			!strings.Contains(initCommand, "textin-sandbox.intsig.com") ||
			!strings.Contains(initCommand, "--profile workbuddy config set base_url") {
			t.Fatalf("platform %q test init is not pinned to sandbox: %q",
				platform, initCommand)
		}
		if strings.Contains(initCommand, "@beta") || strings.Contains(initCommand, "install.sh") || strings.Contains(initCommand, "install.ps1") {
			t.Fatalf("platform %q test init does not use an exact npm beta: %q",
				platform, initCommand)
		}
		if strings.Contains(initCommand, "--prefix") {
			t.Fatalf("platform %q test init uses a custom npm prefix: %q", platform, initCommand)
		}
		if platform == "win32" {
			if !strings.HasPrefix(initCommand, `set "NODE_OPTIONS=" &&`) {
				t.Fatalf("Windows test init does not clear NODE_OPTIONS: %q", initCommand)
			}
			if !strings.Contains(initCommand, "npm.cmd install --global") ||
				strings.Contains(initCommand, "powershell") {
				t.Fatalf("Windows test init can resolve to an execution-policy-blocked npm.ps1: %q",
					initCommand)
			}
		} else if strings.Count(initCommand, "env -u NODE_OPTIONS") != 2 {
			t.Fatalf("platform %q test init does not isolate every Node process: %q",
				platform, initCommand)
		}
		for _, command := range []string{
			contract.Auth[platform],
			contract.UnAuth[platform],
			contract.Status[platform],
			contract.VersionCheck.Command[platform],
		} {
			if platform == "win32" {
				if !strings.Contains(command, `set "NODE_OPTIONS=" &&`) {
					t.Fatalf("Windows lifecycle command does not clear NODE_OPTIONS: %q", command)
				}
			} else if !strings.HasPrefix(command, "env -u NODE_OPTIONS ") {
				t.Fatalf("platform %q lifecycle command does not isolate NODE_OPTIONS: %q",
					platform, command)
			}
		}
	}
	if contract.VersionCheck.MinVersion != "2.2.1" {
		t.Fatalf("test Connector minVersion = %q", contract.VersionCheck.MinVersion)
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

func TestNodeOptionsIsolationAllowsNodeStartup(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is not installed")
	}
	command := exec.Command("/usr/bin/env", "-u", "NODE_OPTIONS", nodePath,
		"-e", `process.stdout.write("ok")`)
	command.Env = append(os.Environ(), "NODE_OPTIONS=--use-system-ca")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("sanitized Node startup failed: %v\n%s", err, output)
	}
	if string(output) != "ok" {
		t.Fatalf("sanitized Node output = %q", output)
	}
}

func TestBuildWorkBuddyLocalPackageUsesLocalAssetsAndNpmMirror(t *testing.T) {
	outputDir := t.TempDir()
	version := "v2.2.1-beta.2-local-test"
	command := exec.Command(
		"/bin/sh",
		repositoryPath(t, "connector", "test", "build-workbuddy-local-package.sh"),
		"--output-dir", outputDir,
		"--version", version,
	)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("build local package: %v\n%s", err, stderr.String())
	}
	packagePath := strings.TrimSpace(string(output))
	archive, err := zip.OpenReader(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()

	prefix := "workbuddy-xparse-connector-" + version + "/"
	contents := make(map[string][]byte)
	for _, file := range archive.File {
		if strings.Contains(file.Name, "/bin/") {
			t.Fatalf("Connector package unexpectedly bundles a CLI binary: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		contents[file.Name] = data
	}
	for name, data := range contents {
		if strings.HasSuffix(strings.ToLower(name), ".ps1") &&
			!bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
			t.Fatalf("PowerShell script is not UTF-8 BOM encoded for Windows PowerShell 5.1: %s", name)
		}
	}

	for _, relative := range []string{
		"install.sh",
		"install.ps1",
		"restore.sh",
		"restore.ps1",
		"manifest.json",
		"SHA256SUMS",
		"scripts/enable-workbuddy-test.sh",
		"scripts/enable-workbuddy-test.ps1",
		"assets/workbuddy-cli.json",
		"assets/workbuddy-connector-meta.json",
		"assets/workbuddy-icon.png",
		"assets/workbuddy-marketplace-entry.json",
		"assets/workbuddy-xparse-parse.zip",
	} {
		if _, ok := contents[prefix+relative]; !ok {
			t.Fatalf("Connector package is missing %s", relative)
		}
	}

	installScript := string(contents[prefix+"install.sh"])
	for _, expected := range []string{
		"env -u NODE_OPTIONS node",
		"env -u NODE_OPTIONS npm view",
		"env -u NODE_OPTIONS",
		"xparse-cli-darwin-arm64",
		"xparse-cli-linux-amd64",
		"WorkBuddy 状态未修改",
		"XPARSE_TEST_ASSET_DIR=",
		"XPARSE_NPM_VERSION=\"2.2.1-beta.2\"",
		"XPARSE_NPM_REGISTRY=\"https://registry.npmmirror.com\"",
	} {
		if !strings.Contains(installScript, expected) {
			t.Fatalf("package installer does not contain %q", expected)
		}
	}
	if strings.Contains(installScript, "dllf.intsig.net") {
		t.Fatal("package installer references DLLF")
	}
	if strings.Contains(installScript, "XPARSE_INSTALL_CLI_WITH_LOCAL_ASSETS=1") {
		t.Fatal("local package installer still performs a custom-prefix CLI installation")
	}
	windowsInstallScript := string(contents[prefix+"install.ps1"])
	for _, expected := range []string{
		"Get-FileHash",
		"npm.cmd view",
		`$OriginalNodeOptions = $env:NODE_OPTIONS`,
		"Remove-Item Env:NODE_OPTIONS",
		`$env:NODE_OPTIONS = $OriginalNodeOptions`,
		"finally",
		"xparse-cli-windows-amd64",
		"xparse-cli-windows-arm64",
		"WorkBuddy 状态未修改",
		`$env:XPARSE_TEST_ASSET_DIR`,
		`$env:XPARSE_NPM_VERSION = "2.2.1-beta.2"`,
		`$env:XPARSE_NPM_REGISTRY = "https://registry.npmmirror.com"`,
		"scripts\\enable-workbuddy-test.ps1",
	} {
		if !strings.Contains(windowsInstallScript, expected) {
			t.Fatalf("Windows package installer does not contain %q", expected)
		}
	}
	if strings.Contains(windowsInstallScript, "dllf.intsig.net") {
		t.Fatal("Windows package installer references DLLF")
	}
	if strings.Contains(windowsInstallScript, `$env:XPARSE_INSTALL_CLI_WITH_LOCAL_ASSETS = "1"`) {
		t.Fatal("Windows local package installer still performs a custom-prefix CLI installation")
	}
	if strings.Contains(windowsInstallScript, "& npm view") ||
		strings.Contains(windowsInstallScript, "Get-Command npm ") {
		t.Fatal("Windows package installer can resolve npm.ps1")
	}
	packagedEnableScript := string(contents[prefix+"scripts/enable-workbuddy-test.sh"])
	for _, expected := range []string{
		"XPARSE_INSTALL_CLI_WITH_LOCAL_ASSETS",
		"https://registry.npmmirror.com",
		`"--registry=${NPM_REGISTRY}"`,
		"install_command_launcher",
		"workbuddy-command-path.added",
		".local/bin",
		`"${COMMAND_PATH}" version`,
	} {
		if !strings.Contains(packagedEnableScript, expected) {
			t.Fatalf("packaged enable script does not contain %q", expected)
		}
	}
	packagedWindowsEnableScript := string(contents[prefix+"scripts/enable-workbuddy-test.ps1"])
	for _, expected := range []string{
		"Get-Command npm.cmd",
		"& npm.cmd install --global",
		"Install-CommandLauncher",
		`call "%USERPROFILE%\.xparse-cli\npm\xparse-cli.cmd" %*`,
		"LegacyCLIBackup",
		`SetEnvironmentVariable("Path"`,
		"& $CommandPath version",
	} {
		if !strings.Contains(packagedWindowsEnableScript, expected) {
			t.Fatalf("packaged Windows enable script does not contain %q", expected)
		}
	}
	packagedRestoreScript := string(contents[prefix+"restore.sh"])
	for _, expected := range []string{
		"restore_command_launcher",
		"COMMAND_BACKUP",
		"PATH_MARKER",
	} {
		if !strings.Contains(packagedRestoreScript, expected) {
			t.Fatalf("packaged restore script does not contain %q", expected)
		}
	}
	packagedWindowsRestoreScript := string(contents[prefix+"restore.ps1"])
	for _, expected := range []string{
		"Restore-CommandLauncher",
		"LegacyCLIBackup",
		`SetEnvironmentVariable(`,
	} {
		if !strings.Contains(packagedWindowsRestoreScript, expected) {
			t.Fatalf("packaged Windows restore script does not contain %q", expected)
		}
	}

	connectorConfig := string(contents[prefix+"assets/workbuddy-cli.json"])
	if !strings.Contains(connectorConfig, "xparse-cli@2.2.1-beta.2") ||
		!strings.Contains(connectorConfig,
			"--registry=https://registry.npmmirror.com") ||
		!strings.Contains(connectorConfig, `"type": "node"`) ||
		!strings.Contains(connectorConfig, `"version": ">=18"`) {
		t.Fatal("packaged Connector does not pin beta.1 to npmmirror")
	}
	if strings.Contains(connectorConfig, "dllf.intsig.net") || strings.Contains(connectorConfig, "--prefix") {
		t.Fatal("packaged Connector config does not use standard global npm")
	}

	assertMarketplaceDescriptions(t,
		readRepositoryFile(t, "connector", "marketplace-entry.json"))
	assertMarketplaceDescriptions(t,
		contents[prefix+"assets/workbuddy-marketplace-entry.json"])

	manifest := string(contents[prefix+"manifest.json"])
	for _, expected := range []string{
		`"npm_package": "xparse-cli@2.2.1-beta.2"`,
		`"npm_registry": "https://registry.npmmirror.com"`,
		`"connector_assets_are_local": true`,
		`"dllf_required": false`,
		`"command_line_launcher": false`,
		`"supported_installers": ["macos", "linux", "windows"]`,
	} {
		if !strings.Contains(manifest, expected) {
			t.Fatalf("package manifest does not contain %q", expected)
		}
	}
}

func assertMarketplaceDescriptions(t *testing.T, data []byte) {
	t.Helper()
	var entry struct {
		Description   string `json:"description"`
		DescriptionZH string `json:"description_zh"`
		DescriptionEN string `json:"description_en"`
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Description != expectedOCRDescription ||
		entry.DescriptionEN != expectedOCRDescription ||
		entry.DescriptionZH != expectedOCRDescriptionZH {
		t.Fatalf("marketplace OCR descriptions do not match: %#v", entry)
	}
}

func TestWorkBuddyPreConnectorIsPinnedToPreOnly(t *testing.T) {
	data := readRepositoryFile(t, "connector", "cli.pre.json")
	var contract connectorCLIContract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	if contract.Runtime.Type != "node" || contract.Runtime.Version != ">=18" {
		t.Fatalf("pre runtime = %#v", contract.Runtime)
	}
	for _, platform := range []string{"darwin", "linux", "win32"} {
		initCommand := contract.Init[platform]
		if !strings.Contains(initCommand, "xparse-cli@2.2.1-beta.2") ||
			!strings.Contains(initCommand, "--registry=https://registry.npmmirror.com") ||
			!strings.Contains(initCommand, "textin-api-pre.intsig.com") ||
			!strings.Contains(initCommand, "--profile workbuddy config set base_url") {
			t.Fatalf("platform %q pre init is not pinned to pre: %q",
				platform, initCommand)
		}
		if strings.Contains(initCommand, "textin-sandbox.intsig.com") ||
			strings.Contains(initCommand, "/latest/") ||
			strings.Contains(initCommand, "--prefix") ||
			strings.Contains(initCommand, "dllf.intsig.net") {
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
      "id": "textin-xparse",
      "description": "old production description",
      "description_zh": "旧的线上描述",
      "description_en": "old production description"
    },
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
	assertCatalogMarketplaceDescriptions(t, catalogPath)
	assertFileContent(t, catalogPath+".textin-xparse.production.bak", productionCatalog)
	assertFileContent(t, filepath.Join(profileDir+".production.bak", "config.yaml"),
		productionProfile)
	assertFileContent(t, filepath.Join(profileDir+".production.bak", "oauth-token.json"),
		productionToken)
	assertFileContent(t, cliPath, productionCLI)
	if _, err := os.Stat(filepath.Dir(cliPath) + ".production.bak"); !os.IsNotExist(err) {
		t.Fatalf("standard npm switch created a custom-prefix backup: %v", err)
	}
	assertFileContent(
		t,
		filepath.Join(activeSkillsDir+".production.bak", "legacy-skill", "SKILL.md"),
		productionActivatedSkill,
	)
	if _, err := os.Stat(profileDir); !os.IsNotExist(err) {
		t.Fatalf("test switch kept the production profile active: %v", err)
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
	assertFileContent(t, cliPath, productionCLI)
	assertCatalogConnectorCount(t, catalogPath, "textin-xparse", 1)
	assertCatalogMarketplaceDescriptions(t, catalogPath)
	assertFileContent(t, marketplaceIcon, testIcon)

	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cliPath), 0o700); err != nil {
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
	assertCatalogConnectorCount(t, catalogPath, "textin-xparse", 1)
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
	testCLIBackups, err := filepath.Glob(filepath.Dir(cliPath) + ".test.*.bak")
	if err != nil || len(testCLIBackups) != 0 {
		t.Fatalf("standard npm switch unexpectedly archived a custom-prefix CLI: %v, err = %v", testCLIBackups, err)
	}
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
	assertCatalogConnectorCount(t, catalogPath, "textin-xparse", 1)
	assertCatalogMarketplaceDescriptions(t, catalogPath)

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

func TestWorkBuddyNpmLauncherMigratesLegacyCLIAndRestoresIt(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	marketplaceRoot := filepath.Join(tempDir, "marketplace")
	catalogDir := filepath.Join(marketplaceRoot, ".codebuddy-connector")
	connectorsDir := filepath.Join(marketplaceRoot, "connectors")
	assetDir := filepath.Join(tempDir, "assets")
	profileDir := filepath.Join(tempDir, "profiles", "workbuddy")
	npmPrefix := filepath.Join(tempDir, "npm")
	cliPath := filepath.Join(npmPrefix, "bin", "xparse-cli")
	commandDir := filepath.Join(homeDir, ".local", "bin")
	commandPath := filepath.Join(commandDir, "xparse-cli")
	pathMarker := filepath.Join(homeDir, ".xparse-cli", "workbuddy-command-path.added")
	testBackupRoot := filepath.Join(tempDir, "test-backups")
	activeSkillsDir := filepath.Join(tempDir, "activated-skills", "connector-textin-xparse")
	fakeBin := filepath.Join(tempDir, "fake-bin")
	for _, dir := range []string{
		homeDir,
		catalogDir,
		connectorsDir,
		assetDir,
		commandDir,
		fakeBin,
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	catalogPath := filepath.Join(catalogDir, "connectors.json")
	productionCatalog := []byte("{\"connectors\":[]}\n")
	if err := os.WriteFile(catalogPath, productionCatalog, 0o600); err != nil {
		t.Fatal(err)
	}
	writeWorkBuddyTestAssets(t, assetDir)
	legacyCLI := []byte("legacy-dllf-cli\n")
	if err := os.WriteFile(commandPath, legacyCLI, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(fakeBin, "node"),
		[]byte("#!/bin/sh\nprintf '24\\n'\n"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	fakeNPM := `#!/bin/sh
prefix=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--prefix" ]; then
    shift
    prefix="$1"
  fi
  shift
done
mkdir -p "$(dirname "${XPARSE_CLI_PATH}")"
printf '#!/bin/sh\nif [ "$1" = version ]; then printf "xparse-cli version 2.2.1-beta.2\\n"; fi\n' > "${XPARSE_CLI_PATH}"
chmod 0755 "${XPARSE_CLI_PATH}"
`
	if err := os.WriteFile(filepath.Join(fakeBin, "npm"), []byte(fakeNPM), 0o700); err != nil {
		t.Fatal(err)
	}

	extraEnv := []string{
		"HOME=" + homeDir,
		"SHELL=/bin/sh",
		"PATH=" + fakeBin + ":/usr/bin:/bin",
		"XPARSE_INSTALL_CLI_WITH_LOCAL_ASSETS=1",
		"XPARSE_COMMAND_DIR=" + commandDir,
		"XPARSE_PATH_MARKER=" + pathMarker,
	}
	runWorkBuddyScriptWithExtraEnv(
		t,
		"enable-workbuddy-test.sh",
		marketplaceRoot,
		profileDir,
		cliPath,
		assetDir,
		testBackupRoot,
		activeSkillsDir,
		extraEnv,
	)
	assertFileContent(t, commandPath+".production.bak", legacyCLI)
	if target, err := os.Readlink(commandPath); err != nil || target != cliPath {
		t.Fatalf("command launcher target = %q, err = %v", target, err)
	}
	assertFileContains(t, filepath.Join(homeDir, ".profile"), commandDir)
	assertFileContains(t, pathMarker, filepath.Join(homeDir, ".profile"))
	output, err := exec.Command(commandPath, "version").CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte("2.2.1-beta.2")) {
		t.Fatalf("command launcher failed: %v\n%s", err, output)
	}

	runWorkBuddyScriptWithExtraEnv(
		t,
		"restore-workbuddy-production.sh",
		marketplaceRoot,
		profileDir,
		cliPath,
		assetDir,
		testBackupRoot,
		activeSkillsDir,
		extraEnv,
	)
	assertFileContent(t, commandPath, legacyCLI)
	if _, err := os.Stat(commandPath + ".production.bak"); !os.IsNotExist(err) {
		t.Fatalf("legacy command backup remains: %v", err)
	}
	if _, err := os.Stat(pathMarker); !os.IsNotExist(err) {
		t.Fatalf("PATH marker remains: %v", err)
	}
	profileData, err := os.ReadFile(filepath.Join(homeDir, ".profile"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(profileData, []byte("xparse-cli WorkBuddy npm launcher")) {
		t.Fatalf("restored shell profile still contains launcher PATH: %s", profileData)
	}
	assertFileContent(t, catalogPath, productionCatalog)
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
	npmBackup := filepath.Dir(cliPath) + ".production.bak"
	if err := os.MkdirAll(npmBackup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npmBackup, filepath.Base(cliPath)), productionCLI, 0o700); err != nil {
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
		npmBackup,
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
	assertFileContent(t, filepath.Join(recoveryRoots[0], "npm-cli.current", filepath.Base(cliPath)), testCLI)
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
	assertFileContent(t, cliPath, productionCLI)
	if _, err := os.Stat(filepath.Dir(cliPath) + ".production.bak"); !os.IsNotExist(err) {
		t.Fatalf("standard npm re-enable recreated a custom-prefix backup: %v", err)
	}
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
			"XPARSE_NPM_PREFIX",
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
		"npm.cmd install --global",
		"Install-CommandLauncher",
		"LegacyCLIBackup",
		`call "%USERPROFILE%\.xparse-cli\npm\xparse-cli.cmd" %*`,
		`SetEnvironmentVariable("Path"`,
		"2.2.1-beta.2",
		"https://registry.npmmirror.com",
		"XPARSE_INSTALL_CLI_WITH_LOCAL_ASSETS",
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
		"Restore-CommandLauncher",
		"LegacyCLIBackup",
		`SetEnvironmentVariable(`,
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
	runWorkBuddyScriptWithExtraEnv(
		t,
		name,
		marketplaceRoot,
		profileDir,
		cliPath,
		assetDir,
		testBackupRoot,
		activeSkillsDir,
		nil,
	)
}

func runWorkBuddyScriptWithExtraEnv(
	t *testing.T,
	name, marketplaceRoot, profileDir, cliPath, assetDir, testBackupRoot,
	activeSkillsDir string,
	extraEnv []string,
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
		"XPARSE_NPM_PREFIX="+filepath.Dir(cliPath),
		"XPARSE_CLI_PATH="+cliPath,
		"XPARSE_TEST_ASSET_DIR="+assetDir,
		"WORKBUDDY_TEST_BACKUP_ROOT="+testBackupRoot,
		"WORKBUDDY_CONNECTOR_SKILLS_DIR="+activeSkillsDir,
	)
	command.Env = overlayEnvironment(command.Env, extraEnv)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s failed: %v\n%s", name, err, output)
	}
}

func overlayEnvironment(base, overrides []string) []string {
	overrideKeys := make(map[string]struct{}, len(overrides))
	for _, entry := range overrides {
		key, _, _ := strings.Cut(entry, "=")
		overrideKeys[key] = struct{}{}
	}
	merged := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, overridden := overrideKeys[key]; !overridden {
			merged = append(merged, entry)
		}
	}
	return append(merged, overrides...)
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

func assertCatalogMarketplaceDescriptions(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Connectors []json.RawMessage `json:"connectors"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, connector := range catalog.Connectors {
		var identity struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(connector, &identity); err != nil {
			t.Fatal(err)
		}
		if identity.ID == "textin-xparse" {
			assertMarketplaceDescriptions(t, connector)
			return
		}
	}
	t.Fatalf("%s does not contain textin-xparse", path)
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
	allParts := append([]string{skillsRepositoryRoot(t)}, pathParts...)
	return filepath.Clean(filepath.Join(allParts...))
}

func skillsRepositoryRoot(t *testing.T) string {
	t.Helper()
	if configured := os.Getenv("XPARSE_SKILLS_REPOSITORY"); configured != "" {
		return configured
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{
		filepath.Join(dir, "..", "..", "..", "xparse-skills"),
		filepath.Join(dir, "..", "..", ".."),
	} {
		if _, err := os.Stat(filepath.Join(candidate, "connector", "cli.json")); err == nil {
			return candidate
		}
	}
	t.Fatal("xparse-skills repository not found; set XPARSE_SKILLS_REPOSITORY")
	return ""
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
	if !strings.Contains(skill, "xparse-cli --profile workbuddy <command>") ||
		!strings.Contains(skill, "standard global npm prefix") {
		t.Fatal("WorkBuddy Skill does not use the isolated profile and standard npm runtime")
	}
	if !strings.Contains(skill, "Use `--api auto` by default") ||
		!strings.Contains(skill, "Authentication is identity, not permission to spend") ||
		!strings.Contains(skill, "quota --output json") {
		t.Fatal("WorkBuddy Skill does not default to quota-aware automatic routing")
	}
	for _, taskContextContract := range []string{
		"xparse_task_context.v1",
		"--task-context <FILE>",
		"only on the first xParse invocation",
		"Do not pass inline JSON",
	} {
		if !strings.Contains(skill, taskContextContract) {
			t.Fatalf("WorkBuddy Skill is missing task context contract %q", taskContextContract)
		}
	}
	if _, err := os.Stat(repositoryPath(t, "skills", "xparse-doc-tools", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("standalone xparse-doc-tools Skill still exists: %v", err)
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
	if !strings.Contains(cliGuidance, "AppKey-authenticated request") ||
		!strings.Contains(cliGuidance, "free_remain_count") ||
		!strings.Contains(authentication, "uses the daily free allowance first") ||
		!strings.Contains(authentication, "not approval to run `--api paid`") {
		t.Fatal("Skill references do not explain automatic free-package routing and paid approval")
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
	parts := append([]string{skillsRepositoryRoot(t)}, pathParts...)
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
