/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package release fetches release metadata and downloads product and sample binaries.
package release

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
)

var platformMap = map[string]string{
	"darwin":  "macos",
	"linux":   "linux",
	"windows": "win",
}

var archMap = map[string]string{
	"amd64": "x64",
	"arm64": "arm64",
}

type releaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"downloadUrl"`
}

type releaseEntry struct {
	TagName  string         `json:"tagName"`
	IsLatest bool           `json:"isLatest"`
	Assets   []releaseAsset `json:"assets"`
}

type releasesData struct {
	LatestRelease releaseEntry   `json:"latestRelease"`
	Releases      []releaseEntry `json:"releases"`
}

// PlatformAssetName returns the platform-specific ZIP name for the product binary.
func PlatformAssetName(version string) (string, error) {
	platform, ok := platformMap[runtime.GOOS]
	if !ok {
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	arch, ok := archMap[runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
	return fmt.Sprintf("%s-%s-%s-%s.zip", product.Slug, version, platform, arch), nil
}

func fetchJSON(url string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", product.Slug+"-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func fetchReleasesData() (*releasesData, error) {
	var data releasesData
	if err := fetchJSON(product.ReleasesURL, &data); err == nil {
		return &data, nil
	}

	// Fallback to GitHub API.
	var gh struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := fetchJSON(product.GitHubAPI, &gh); err != nil {
		return nil, err
	}
	if gh.TagName == "" {
		return nil, fmt.Errorf("tag_name missing from GitHub release response")
	}
	assets := make([]releaseAsset, len(gh.Assets))
	for i, a := range gh.Assets {
		assets[i] = releaseAsset{Name: a.Name, DownloadURL: a.BrowserDownloadURL}
	}
	r := releaseEntry{TagName: gh.TagName, IsLatest: true, Assets: assets}
	return &releasesData{LatestRelease: r, Releases: []releaseEntry{r}}, nil
}

// FetchLatestVersion queries releases metadata and returns the latest version string (no "v" prefix).
func FetchLatestVersion() (string, error) {
	data, err := fetchReleasesData()
	if err != nil {
		return "", err
	}
	tag := data.LatestRelease.TagName
	if tag == "" {
		return "", fmt.Errorf("tagName missing from releases data")
	}
	return strings.TrimPrefix(tag, "v"), nil
}

// ProgressFunc is called during download/extract operations.
// pct is the percentage (0–100), or -1 for status-only messages (e.g. "Extracting...").
type ProgressFunc func(pct int, msg string)

// Download downloads and extracts the product release for the current platform.
func Download(version, destDir string, onProgress ProgressFunc) error {
	assetName, err := PlatformAssetName(version)
	if err != nil {
		return err
	}

	data, err := fetchReleasesData()
	if err != nil {
		return err
	}

	found := findAsset(data, version, assetName)
	if found == nil {
		return fmt.Errorf("no release asset found for %s", assetName)
	}

	if onProgress != nil {
		onProgress(-1, fmt.Sprintf("Downloading Thunder v%s for %s/%s", version, runtime.GOOS, runtime.GOARCH))
	}

	zipPath := filepath.Join(os.TempDir(), assetName)
	if err := downloadFile(found.DownloadURL, zipPath, func(received, total int64) {
		if total > 0 && onProgress != nil {
			pct := int(float64(received) / float64(total) * 100)
			onProgress(pct, fmt.Sprintf("Downloading Thunder v%s", version))
		}
	}); err != nil {
		return err
	}
	defer func() { _ = os.Remove(zipPath) }()

	if onProgress != nil {
		onProgress(-1, "Extracting...")
	}
	return extractZip(zipPath, destDir)
}

// SampleAssetName returns the ZIP name for a sample app.
// Pattern: sample-app-{name}-{version}.zip
func SampleAssetName(sampleName, version string) (string, error) {
	return fmt.Sprintf("sample-app-%s-%s.zip", sampleName, version), nil
}

// DownloadSample downloads and extracts the named sample to destDir.
func DownloadSample(sampleName, version, destDir string, onProgress ProgressFunc) error {
	assetName, err := SampleAssetName(sampleName, version)
	if err != nil {
		return err
	}

	data, err := fetchReleasesData()
	if err != nil {
		return err
	}

	found := findAsset(data, version, assetName)
	if found == nil {
		return fmt.Errorf("no release asset found for %s", assetName)
	}

	if onProgress != nil {
		onProgress(-1, fmt.Sprintf("Downloading %s sample v%s", sampleName, version))
	}

	zipPath := filepath.Join(os.TempDir(), assetName)
	if err := downloadFile(found.DownloadURL, zipPath, func(received, total int64) {
		if total > 0 && onProgress != nil {
			pct := int(float64(received) / float64(total) * 100)
			onProgress(pct, fmt.Sprintf("Downloading %s sample", sampleName))
		}
	}); err != nil {
		return err
	}
	defer func() { _ = os.Remove(zipPath) }()

	if onProgress != nil {
		onProgress(-1, "Extracting...")
	}
	return extractZip(zipPath, destDir)
}

func findAsset(data *releasesData, version, assetName string) *releaseAsset {
	for _, r := range data.Releases {
		if r.TagName == "v"+version {
			for i := range r.Assets {
				if r.Assets[i].Name == assetName {
					return &r.Assets[i]
				}
			}
		}
	}
	for i := range data.LatestRelease.Assets {
		if data.LatestRelease.Assets[i].Name == assetName {
			return &data.LatestRelease.Assets[i]
		}
	}
	return nil
}

func downloadFile(url, destPath string, onProgress func(received, total int64)) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", product.Slug+"-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d downloading %s", resp.StatusCode, url)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	total := resp.ContentLength
	var received int64
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			received += int64(n)
			if onProgress != nil {
				onProgress(received, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func extractZip(zipPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	// Determine the top-level directory prefix to strip.
	var prefix string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			prefix = f.Name
			break
		}
	}

	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)

	for _, f := range r.File {
		name := strings.TrimPrefix(f.Name, prefix)
		if name == "" {
			continue
		}

		target := filepath.Join(destDir, filepath.Clean(name))

		// Guard against zip-slip.
		if !strings.HasPrefix(target, cleanDest) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, f.Mode()) //nolint:errcheck
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			_ = out.Close()
			return err
		}

		_, copyErr := io.Copy(out, rc) //nolint:gosec
		if err := rc.Close(); err != nil && copyErr == nil {
			copyErr = err
		}
		if err := out.Close(); err != nil && copyErr == nil {
			copyErr = err
		}
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}
