package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update xParser CLI to the latest version",
	Long:  `Download and install the latest version of the xParser CLI binary.`,
	RunE:  runUpdate,
}

const (
	cdnBaseURL = "https://dllf.intsig.net/download/2026/Solution/xparse-cli"
)

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	binaryName := fmt.Sprintf("xparse-cli-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	downloadURL := fmt.Sprintf("%s/latest/%s", cdnBaseURL, binaryName)

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	tmpPath := execPath + ".new"
	if err := downloadFile(downloadURL, tmpPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download failed: %w", err)
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod failed: %w", err)
	}

	oldPath := execPath + ".old"
	os.Remove(oldPath)

	if err := os.Rename(execPath, oldPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Rename(oldPath, execPath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	os.Remove(oldPath)
	return nil
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
