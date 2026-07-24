package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"os"
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
			"auth device --open-browser=never --output=jsonl") {
			t.Fatalf("platform %q auth command is not native Device Flow: %q",
				platform, contract.Auth[platform])
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
	if contract.Env["XPARSE_CONFIG_DIR"] == "" {
		t.Fatal("Connector does not isolate XPARSE_CONFIG_DIR")
	}
	if contract.Env["XPARSE_OAUTH_CLIENT_ID"] != "cli_textin_xparse" {
		t.Fatalf("Connector public client ID = %q", contract.Env["XPARSE_OAUTH_CLIENT_ID"])
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

	event := `{"type":"device_authorization","verification_uri":"https://api.textin.com/oauth21/device","verification_uri_complete":"https://api.textin.com/oauth21/device?user_code=ABCDE-FGHJK","user_code":"ABCDE-FGHJK","expires_in":240,"interval":5}`
	uriMatch := regexp.MustCompile(contract.AuthDeviceFlow.URIPattern).
		FindStringSubmatch(event)
	codeMatch := regexp.MustCompile(contract.AuthDeviceFlow.CodePattern).
		FindStringSubmatch(event)
	if len(uriMatch) != 2 ||
		uriMatch[1] != "https://api.textin.com/oauth21/device?user_code=ABCDE-FGHJK" {
		t.Fatalf("uri match = %#v", uriMatch)
	}
	if len(codeMatch) != 2 || codeMatch[1] != "ABCDE-FGHJK" {
		t.Fatalf("code match = %#v", codeMatch)
	}
	if contract.AuthDeviceFlow.DefaultExpiresInSeconds != 240 ||
		!contract.AuthDeviceFlow.CodeEmbeddedInURI {
		t.Fatalf("authDeviceFlow = %#v", contract.AuthDeviceFlow)
	}
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
	parts := append([]string{"..", ".."}, pathParts...)
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
