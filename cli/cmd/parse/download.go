package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/exitcode"
)

var (
	downloadOutput string
	downloadFrom   string
)

var downloadCmd = &cobra.Command{
	Use:   "download [url ...] [--from result.json]",
	Short: "Download images from parse results",
	Long: `Download element images from xParser parse results.

Two modes:

  1. By URL:      xparse-cli download <url1> <url2> -o ./images/
  2. From JSON:   xparse-cli download --from result.json -o ./images/

With --from, the command reads a parse result JSON file and downloads all
images found in data.elements[].image_data.image_url.

Image URLs are publicly accessible and downloaded directly via HTTP GET.`,
	Example: `  xparse-cli download --from result.json -o ./images/
  xparse-cli download https://web-api.textin.com/ocr_image/external/abc123.jpg -o ./images/`,
	RunE: runDownload,
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringVar(&downloadOutput, "output", ".", "Output path (file or directory)")
	downloadCmd.Flags().StringVar(&downloadFrom, "from", "", "Parse result JSON file to extract image URLs from")
}

// extractImageURLs reads a parse result JSON and extracts all image URLs.
func extractImageURLs(jsonPath string) ([]string, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("read JSON file: %w", err)
	}

	var result struct {
		Data struct {
			Elements []struct {
				ImageData struct {
					ImageURL string `json:"image_url"`
				} `json:"image_data"`
			} `json:"elements"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	seen := make(map[string]bool)
	var urls []string

	for _, el := range result.Data.Elements {
		u := strings.TrimSpace(el.ImageData.ImageURL)
		if u != "" && !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}

	return urls, nil
}

func runDownload(cmd *cobra.Command, args []string) error {
	var imageURLs []string
	imageURLs = append(imageURLs, args...)

	if downloadFrom != "" {
		extracted, err := extractImageURLs(downloadFrom)
		if err != nil {
			return generalErr("failed to read --from file: "+err.Error(),
				"[fix] check that "+downloadFrom+" is a valid parse result JSON")
		}
		if len(extracted) == 0 {
			return generalErr("no image URLs found in "+downloadFrom,
				"[fix] ensure the JSON contains data.elements[].image_data.image_url fields")
		}

		imageURLs = append(imageURLs, extracted...)
	}

	if len(imageURLs) == 0 {
		return usageErr(exitcode.ErrNoInput,
			"[fix] provide URLs as arguments or use --from <result.json>")
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	if verboseFlag {
		httpClient = newVerboseHTTPClient()
		httpClient.Timeout = 60 * time.Second
	}

	// Determine if --output is a directory:
	//   1. Path ends with / or OS separator
	//   2. Path already exists as a directory
	//   3. Multiple URLs (must be a directory)
	//   4. Path has no file extension (e.g. "./images" not "./photo.jpg")
	outputIsDir := strings.HasSuffix(downloadOutput, "/") ||
		strings.HasSuffix(downloadOutput, string(filepath.Separator)) ||
		isExistingDir(downloadOutput) ||
		len(imageURLs) > 1 ||
		filepath.Ext(downloadOutput) == ""

	if outputIsDir {
		if !isExistingDir(downloadOutput) {
			return generalErr(exitcode.ErrOutputDirNotExist+": "+downloadOutput,
				"[ask human] create the directory with mkdir -p "+downloadOutput)
		}
	} else {
		parentDir := filepath.Dir(downloadOutput)
		if !isExistingDir(parentDir) {
			return generalErr(exitcode.ErrOutputDirNotExist+": "+parentDir,
				"[ask human] create the directory with mkdir -p "+parentDir)
		}
	}

	succeeded := 0

	for i, imageURL := range imageURLs {
		var outPath string
		if outputIsDir {
			// Derive filename from URL
			filename := filepath.Base(imageURL)
			if idx := strings.IndexByte(filename, '?'); idx != -1 {
				filename = filename[:idx]
			}
			if filename == "" || filename == "." {
				filename = fmt.Sprintf("image_%d.jpg", i+1)
			}
			ext := strings.ToLower(filepath.Ext(filename))
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" && ext != ".bmp" && ext != ".tiff" {
				filename += ".jpg"
			}
			outPath = filepath.Join(downloadOutput, filename)
		} else {
			// --output is an explicit file path
			outPath = downloadOutput
		}

		resp, err := httpClient.Get(imageURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%d/%d] %s - request failed: %v\n", i+1, len(imageURLs), filepath.Base(imageURL), err)
			fmt.Fprintf(os.Stderr, "> [retry] %s; check if URL is accessible\n", imageURL)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "[%d/%d] %s - HTTP %d\n", i+1, len(imageURLs), filepath.Base(imageURL), resp.StatusCode)
			fmt.Fprintf(os.Stderr, "> [retry] %s; if persists, [ask human] check the URL\n", imageURL)
			continue
		}

		f, err := os.Create(outPath)
		if err != nil {
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "[%d/%d] %s - create file failed: %v\n", i+1, len(imageURLs), filepath.Base(imageURL), err)
			fmt.Fprintf(os.Stderr, "> [ask human] check write permissions for %s\n", outPath)
			continue
		}

		if _, err := io.Copy(f, resp.Body); err != nil {
			resp.Body.Close()
			f.Close()
			fmt.Fprintf(os.Stderr, "[%d/%d] %s - write failed: %v\n", i+1, len(imageURLs), filepath.Base(imageURL), err)
			fmt.Fprintf(os.Stderr, "> [ask human] check disk space and permissions for %s\n", outPath)
			continue
		}
		resp.Body.Close()
		f.Close()
		succeeded++
	}

	if succeeded < len(imageURLs) {
		return generalErr(fmt.Sprintf("%d/%d downloads failed", len(imageURLs)-succeeded, len(imageURLs)),
			"[retry] re-run the command; check stderr above for per-file details")
	}
	return nil
}

func isExistingDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
