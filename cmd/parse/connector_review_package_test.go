package cmd

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDomesticConnectorReviewPackageContract(t *testing.T) {
	outputDir := t.TempDir()
	clientRoot := repositoryPath(t)
	skillsRoot := filepath.Dir(repositoryPath(t, "skills"))
	command := exec.Command(
		"bash",
		filepath.Join(clientRoot, "connector", "package-review.sh"),
		"v2.2.1",
		outputDir,
	)
	command.Env = append(os.Environ(), "XPARSE_SKILLS_REPOSITORY="+skillsRoot)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("package review connector: %v\n%s", err, output)
	}

	archivePath := filepath.Join(outputDir, "textin-xparse-v2.2.1-cn.zip")
	files := readReviewArchive(t, archivePath)
	prefix := "textin-xparse-v2.2.1/"
	for _, required := range []string{
		"SHA256SUMS",
		"cli.json",
		"connector-meta.json",
		"icon.png",
		"marketplace-entry.json",
		"skills/xparse-parse/SKILL.md",
		"skills/xparse-parse/agents/openai.yaml",
		"skills/xparse-parse/assets/logo.png",
	} {
		if _, ok := files[prefix+required]; !ok {
			t.Errorf("review archive missing %s", required)
		}
	}
	for name := range files {
		lowerName := strings.ToLower(name)
		if strings.Contains(name, "xparse-doc-tools") ||
			strings.Contains(lowerName, "review.md") ||
			strings.Contains(lowerName, ".ds_store") ||
			strings.HasSuffix(lowerName, ".bak") ||
			strings.Contains(lowerName, ".dev-flow") ||
			strings.Contains(lowerName, ".global") {
			t.Errorf("review archive contains forbidden path %s", name)
		}
	}

	var marketplace struct {
		VisibleIn           []string `json:"visible_in"`
		MinWorkBuddyVersion string   `json:"minWorkbuddyVersion"`
		Description         string   `json:"description"`
		DescriptionZH       string   `json:"description_zh"`
		DescriptionEN       string   `json:"description_en"`
	}
	if err := json.Unmarshal(files[prefix+"marketplace-entry.json"], &marketplace); err != nil {
		t.Fatal(err)
	}
	wantVisibleIn := []string{"internal", "iOA", "cloudhosted", "selfhosted"}
	if strings.Join(marketplace.VisibleIn, ",") != strings.Join(wantVisibleIn, ",") {
		t.Fatalf("visible_in = %v, want %v", marketplace.VisibleIn, wantVisibleIn)
	}
	if marketplace.MinWorkBuddyVersion != "5.0.0" {
		t.Fatalf("minWorkbuddyVersion = %q", marketplace.MinWorkBuddyVersion)
	}
	for field, description := range map[string]string{
		"description":    marketplace.Description,
		"description_zh": marketplace.DescriptionZH,
		"description_en": marketplace.DescriptionEN,
	} {
		if !strings.Contains(description, "500") || strings.Contains(description, "1,000") ||
			strings.Contains(description, "1000") {
			t.Errorf("%s does not describe the 500-page free quota: %q", field, description)
		}
	}

	var cli connectorCLIContract
	if err := json.Unmarshal(files[prefix+"cli.json"], &cli); err != nil {
		t.Fatal(err)
	}
	if cli.VersionCheck.MinVersion != "2.2.1" {
		t.Fatalf("minVersion = %q", cli.VersionCheck.MinVersion)
	}
	for platform, initCommand := range cli.Init {
		if !strings.Contains(initCommand, "/v2.2.1/install") ||
			strings.Contains(initCommand, "/latest/") {
			t.Errorf("%s init command is not pinned: %s", platform, initCommand)
		}
	}

	rootIcon := files[prefix+"icon.png"]
	skillLogo := files[prefix+"skills/xparse-parse/assets/logo.png"]
	if len(rootIcon) == 0 || len(skillLogo) == 0 {
		t.Fatal("both Connector icon and Skill logo must be packaged")
	}
	if sha256.Sum256(rootIcon) != sha256.Sum256(skillLogo) {
		t.Fatal("Connector icon and Skill logo no longer match the approved asset")
	}
	assertReviewChecksums(t, prefix, files)
}

func TestDomesticSkillDocumentsUse500PageDailyQuota(t *testing.T) {
	guidance := string(readRepositoryFile(
		t,
		"skills",
		"xparse-parse",
		"references",
		"cli-guidance.md",
	))
	if !strings.Contains(guidance, "单 IP ≤ 500 页/天") {
		t.Fatal("Skill guidance does not document the 500-page daily free quota")
	}
	if strings.Contains(guidance, "单 IP ≤ 1000 页/天") ||
		strings.Contains(guidance, "单 IP ≤ 1,000 页/天") {
		t.Fatal("Skill guidance retains the old 1000-page daily free quota")
	}
}

func TestDomesticConnectorReviewValidatorRejectsMissingVisibleIn(t *testing.T) {
	clientRoot := repositoryPath(t)
	marketplacePath := filepath.Join(t.TempDir(), "marketplace-entry.json")
	var marketplace map[string]any
	if err := json.Unmarshal(
		readRepositoryFile(t, "connector", "marketplace-entry.json"),
		&marketplace,
	); err != nil {
		t.Fatal(err)
	}
	delete(marketplace, "visible_in")
	data, err := json.Marshal(marketplace)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marketplacePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(
		"python3",
		filepath.Join(clientRoot, "connector", "validate-review-input.py"),
		filepath.Join(clientRoot, "connector", "cli.json"),
		filepath.Join(clientRoot, "connector", "connector-meta.json"),
		marketplacePath,
		"v2.2.1",
		repositoryPath(t, "skills", "xparse-parse"),
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("validator accepted marketplace-entry.json without visible_in")
	}
	if !strings.Contains(string(output), "visible_in") {
		t.Fatalf("validator error does not identify visible_in: %s", output)
	}
}

func readReviewArchive(t *testing.T, archivePath string) map[string][]byte {
	t.Helper()
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()

	files := make(map[string][]byte)
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			reader.Close()
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		files[entry.Name] = data
	}
	return files
}

func assertReviewChecksums(t *testing.T, prefix string, files map[string][]byte) {
	t.Helper()
	checksumData := string(files[prefix+"SHA256SUMS"])
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(checksumData), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("invalid checksum line %q", line)
		}
		name := strings.TrimPrefix(fields[1], "./")
		data, ok := files[prefix+name]
		if !ok {
			t.Fatalf("checksum references missing file %s", name)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != fields[0] {
			t.Fatalf("checksum for %s = %s, want %s", name, fields[0], got)
		}
		seen[name] = true
	}
	for name := range files {
		relative := strings.TrimPrefix(name, prefix)
		if relative != "SHA256SUMS" && !seen[relative] {
			t.Errorf("file missing checksum: %s", relative)
		}
	}
}
