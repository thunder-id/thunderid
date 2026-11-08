// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package release fetches release metadata and downloads product and sample binaries.
package release

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tools/cli/internal/plane"
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
	TagName      string         `json:"tagName"`
	IsLatest     bool           `json:"isLatest"`
	IsPrerelease bool           `json:"isPrerelease"`
	PublishedAt  string         `json:"publishedAt"`
	Assets       []releaseAsset `json:"assets"`
}

type releasesData struct {
	LatestRelease releaseEntry   `json:"latestRelease"`
	Releases      []releaseEntry `json:"releases"`
}

// entries returns every release the manifest lists. The latest release can appear only under
// latestRelease, so it is included alongside the others.
func (d *releasesData) entries() []releaseEntry {
	return append([]releaseEntry{d.LatestRelease}, d.Releases...)
}

// PlatformAssetName returns the platform-specific ZIP name of the pack named stem in a release.
// The stem is the product's own name, or gatewayStem for a 2.x Data Plane pack.
func PlatformAssetName(stem, version string) (string, error) {
	platform, ok := platformMap[runtime.GOOS]
	if !ok {
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	arch, ok := archMap[runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
	return fmt.Sprintf("%s-%s-%s-%s.zip", stem, version, platform, arch), nil
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

func (c *Client) fetchReleasesData() (*releasesData, error) {
	if c.GitHubRelease {
		return fetchGitHubRelease(c.ManifestURL)
	}
	return fetchReleasesDataFrom(c.ManifestURL, c.FallbackURL)
}

// fetchReleasesDataFrom reads releases metadata from primaryURL and falls back to the
// GitHub release API at fallbackURL. An empty fallbackURL disables the fallback, which is what a
// custom source implies. The URLs are parameters so tests can point both at a local server.
func fetchReleasesDataFrom(primaryURL, fallbackURL string) (*releasesData, error) {
	var data releasesData
	primaryErr := fetchJSON(primaryURL, &data)
	if primaryErr == nil {
		return &data, nil
	}
	if fallbackURL == "" {
		return nil, primaryErr
	}

	release, err := fetchGitHubRelease(fallbackURL)
	if err != nil {
		// Report both hosts: dropping the primary error blames the fallback for a
		// failure that started somewhere else.
		return nil, fmt.Errorf("%w; fallback %w", primaryErr, err)
	}
	return release, nil
}

// fetchGitHubRelease reads one release from the GitHub release API at url and presents it as a
// manifest that carries only that release.
func fetchGitHubRelease(url string) (*releasesData, error) {
	var gh struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := fetchJSON(url, &gh); err != nil {
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

// FetchLatestVersion returns the latest version from the configured source.
func FetchLatestVersion() (string, error) { return Default.LatestVersion() }

// LatestVersion returns the newest version the manifest advertises, without a leading "v".
// It ignores any pinned version: callers use it to tell the operator that an upgrade exists.
// The manifest's latest release is its newest stable one; a client that takes prereleases reads
// the newest release of any kind instead.
func (c *Client) LatestVersion() (string, error) {
	data, err := c.fetchReleasesData()
	if err != nil {
		return "", err
	}
	tag := data.LatestRelease.TagName
	if c.Prereleases && !c.GitHubRelease {
		newest := newestRelease(data)
		if newest == "" {
			// Only the GitHub fallback lacks publish dates, and it reports the latest stable release
			// alone. Taking it would quietly move a next-channel CLI onto the stable line.
			return "", fmt.Errorf("could not find the newest prerelease: the release manifest at %s "+
				"is unreachable, and the fallback lists stable releases only", c.ManifestURL)
		}
		tag = newest
	}
	if tag == "" {
		return "", fmt.Errorf("tagName missing from releases data")
	}
	return strings.TrimPrefix(tag, "v"), nil
}

// newestRelease returns the tag of the most recently published release in data, prerelease or
// not, or "" when no entry carries a publish date, which is the case for the GitHub fallback.
func newestRelease(data *releasesData) string {
	var tag string
	var newest time.Time
	for _, r := range data.entries() {
		published, err := time.Parse(time.RFC3339, r.PublishedAt)
		if err != nil || r.TagName == "" {
			continue
		}
		if tag == "" || published.After(newest) {
			tag, newest = r.TagName, published
		}
	}
	return tag
}

// ProgressFunc is called during download/extract operations.
// pct is the percentage (0–100), or -1 for status-only messages (e.g. "Extracting...").
type ProgressFunc func(pct int, msg string)

// Download downloads and extracts the product release for the current platform.
func Download(version, destDir string, onProgress ProgressFunc) error {
	return Default.Download(version, destDir, onProgress)
}

// Download downloads and extracts the product release for the current platform.
func (c *Client) Download(version, destDir string, onProgress ProgressFunc) error {
	data, err := c.fetchReleasesData()
	if err != nil {
		return err
	}

	found, assetName, err := c.findPlaneAsset(data, version)
	if err != nil {
		return err
	}

	if onProgress != nil {
		onProgress(-1, fmt.Sprintf("Downloading "+product.Name+" v%s for %s/%s", version, runtime.GOOS, runtime.GOARCH))
	}

	zipPath := filepath.Join(os.TempDir(), assetName)
	if err := downloadFile(found.DownloadURL, zipPath, func(received, total int64) {
		if total > 0 && onProgress != nil {
			pct := int(float64(received) / float64(total) * 100)
			onProgress(pct, fmt.Sprintf("Downloading "+product.Name+" v%s", version))
		}
	}); err != nil {
		return err
	}
	defer func() { _ = os.Remove(zipPath) }()

	if onProgress != nil {
		onProgress(-1, "Extracting...")
	}
	return extractInto(zipPath, destDir)
}

// gatewayStem is the name a 2.x release gives its Data Plane pack.
const gatewayStem = product.Slug + "-gateway"

// planeAsset is the pack a release ships for one plane.
type planeAsset struct {
	asset *releaseAsset
	name  string
}

// planeAssets returns the packs version ships for the current platform, by plane. The product's own
// name carries a different pack depending on the release: from 2.0 the Data Plane is named
// thunderid-gateway and the product's name goes to the quickstart, while a 1.x release ships only
// the Data Plane, under the product's name. A gateway pack, for any platform, is what tells the two
// apart, so a 2.x release missing this platform's Data Plane never offers its quickstart as one.
func planeAssets(data *releasesData, version string) (map[plane.Plane]planeAsset, string, error) {
	productName, err := PlatformAssetName(product.Slug, version)
	if err != nil {
		return nil, "", err
	}
	gatewayName, err := PlatformAssetName(gatewayStem, version)
	if err != nil {
		return nil, "", err
	}
	assets := map[plane.Plane]planeAsset{}
	productAsset := findAsset(data, version, productName)
	if shipsGateway(data, version) {
		if gateway := findAsset(data, version, gatewayName); gateway != nil {
			assets[plane.Data] = planeAsset{gateway, gatewayName}
		}
		if productAsset != nil {
			assets[plane.Quickstart] = planeAsset{productAsset, productName}
		}
		return assets, gatewayName, nil
	}
	if productAsset != nil {
		assets[plane.Data] = planeAsset{productAsset, productName}
	}
	return assets, productName, nil
}

// shipsGateway reports whether the release for version lists a gateway pack for any platform.
func shipsGateway(data *releasesData, version string) bool {
	for _, r := range data.entries() {
		if r.TagName != "v"+version {
			continue
		}
		for _, a := range r.Assets {
			if strings.HasPrefix(a.Name, gatewayStem+"-") {
				return true
			}
		}
	}
	return false
}

// findPlaneAsset returns the asset of the requested plane for version. With no plane requested it
// takes the quickstart when the release ships one and the Data Plane otherwise.
func (c *Client) findPlaneAsset(data *releasesData, version string) (*releaseAsset, string, error) {
	assets, dataName, err := planeAssets(data, version)
	if err != nil {
		return nil, "", err
	}
	want := c.Plane
	if want == plane.Auto {
		want = plane.Quickstart
		if _, ok := assets[want]; !ok {
			want = plane.Data
		}
	}
	if found, ok := assets[want]; ok {
		return found.asset, found.name, nil
	}
	if len(assets) > 0 {
		var planes []string
		for _, p := range []plane.Plane{plane.Quickstart, plane.Data} {
			if _, ok := assets[p]; ok {
				planes = append(planes, string(p))
			}
		}
		return nil, "", fmt.Errorf("v%s has no %s for this platform; it ships: %s",
			version, want.Label(), strings.Join(planes, ", "))
	}
	return nil, "", fmt.Errorf("no release asset found for %s at %s%s", dataName, c.ManifestURL, c.availableVersions(data))
}

// extractInto extracts zipPath into destDir and removes a half-written destDir when the
// extraction fails, so a failed install does not leave a broken tree behind. Ownership
// comes from creating destDir here: anything that already exists at that path, including
// a dangling symlink or a directory another process created first, is not this run's to
// delete.
func extractInto(zipPath, destDir string) error {
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return err
	}
	createdByThisRun := false
	switch err := os.Mkdir(destDir, 0o755); {
	case err == nil:
		createdByThisRun = true
	case errors.Is(err, os.ErrExist):
		// Left in place; extractZip writes into it.
	default:
		return err
	}
	if err := extractZip(zipPath, destDir); err != nil {
		if createdByThisRun {
			_ = os.RemoveAll(destDir)
		}
		return err
	}
	return nil
}

// SampleAssetName returns the ZIP name for a sample app.
// Pattern: sample-app-{name}-{version}.zip
func SampleAssetName(sampleName, version string) (string, error) {
	return fmt.Sprintf("sample-app-%s-%s.zip", sampleName, version), nil
}

// DownloadSample downloads and extracts the named sample to destDir.
func DownloadSample(sampleName, version, destDir string, onProgress ProgressFunc) error {
	return Default.DownloadSample(sampleName, version, destDir, onProgress)
}

// DownloadSample downloads and extracts the named sample to destDir.
func (c *Client) DownloadSample(sampleName, version, destDir string, onProgress ProgressFunc) error {
	assetName, err := SampleAssetName(sampleName, version)
	if err != nil {
		return err
	}

	data, err := c.fetchReleasesData()
	if err != nil {
		return err
	}

	found := findAsset(data, version, assetName)
	if found == nil {
		return fmt.Errorf("no release asset found for %s at %s%s", assetName, c.ManifestURL, c.availableVersions(data))
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
	return extractInto(zipPath, destDir)
}

// findAsset returns the named asset belonging to the requested version, or nil.
//
// It matches on the version and never falls back to another release: returning the latest
// release's asset for a version the manifest does not carry would install something other than
// what was asked for, under the name that was asked for.
func findAsset(data *releasesData, version, assetName string) *releaseAsset {
	for _, r := range data.entries() {
		if r.TagName != "v"+version {
			continue
		}
		for i := range r.Assets {
			if r.Assets[i].Name == assetName {
				return &r.Assets[i]
			}
		}
	}
	return nil
}

// availableVersions renders the versions a manifest carries, for an error that would otherwise
// only say the asset was missing. A client that does not take prereleases leaves them out, so a
// stable CLI never advertises a milestone.
func (c *Client) availableVersions(data *releasesData) string {
	seen := map[string]bool{}
	var versions []string
	for _, r := range data.entries() {
		if r.TagName == "" || seen[r.TagName] || (r.IsPrerelease && !c.Prereleases) {
			continue
		}
		seen[r.TagName] = true
		versions = append(versions, r.TagName)
	}
	if len(versions) == 0 {
		return ""
	}
	return " (available: " + strings.Join(versions, ", ") + ")"
}

// downloadAttempts and downloadRetryDelay bound how often a transient failure is retried. They are
// variables so tests can retry without waiting.
var (
	downloadAttempts   = 3
	downloadRetryDelay = 2 * time.Second
)

// isTransientStatus reports whether a status is a server-side hiccup worth retrying, such as the
// 502 and 504 responses GitHub's release CDN returns now and then.
func isTransientStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

// getForDownload requests url, retrying connection failures and transient statuses with a growing
// delay. Any other status fails at once.
func getForDownload(url string) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= downloadAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt-1) * downloadRetryDelay)
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", product.Slug+"-cli")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		_ = resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d downloading %s", resp.StatusCode, url)
		if !isTransientStatus(resp.StatusCode) {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

func downloadFile(url, destPath string, onProgress func(received, total int64)) error {
	resp, err := getForDownload(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	f, err := os.CreateTemp(filepath.Dir(destPath), "."+filepath.Base(destPath)+"-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

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
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), destPath)
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
