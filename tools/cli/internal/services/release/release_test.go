// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package release_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/tools/cli/internal/services/release"
)

func TestPlatformAssetName_Format(t *testing.T) {
	name, err := release.PlatformAssetName("1.2.3")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(name, "thunderid-gateway-1.2.3-"), "got %q", name)
	assert.True(t, strings.HasSuffix(name, ".zip"), "got %q", name)
}

func TestPlatformAssetName_ContainsPlatformAndArch(t *testing.T) {
	name, err := release.PlatformAssetName("0.5.0")
	require.NoError(t, err)

	// Must contain one of the known platform names.
	platforms := []string{"macos", "linux", "win"}
	found := false
	for _, p := range platforms {
		if strings.Contains(name, p) {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a known platform in %q", name)

	// Must contain one of the known arch names.
	arches := []string{"x64", "arm64"}
	found = false
	for _, a := range arches {
		if strings.Contains(name, a) {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a known arch in %q", name)
}

func TestSampleAssetName_Format(t *testing.T) {
	name, err := release.SampleAssetName("wayfinder", "1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "sample-app-wayfinder-1.2.3.zip", name)
}

func TestSampleAssetName_DifferentSamples(t *testing.T) {
	a, errA := release.SampleAssetName("wayfinder", "1.0.0")
	b, errB := release.SampleAssetName("agentid", "1.0.0")
	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.NotEqual(t, a, b, "different sample names should produce different asset names")
}

// packNames returns this platform's Data Plane pack name for version, and the name releases
// published before it (v1.1.0 and older) gave the pack. Later releases give the quickstart pack
// that second name.
func packNames(t *testing.T, version string) (gateway, legacy string) {
	t.Helper()
	gateway, err := release.PlatformAssetName(version)
	require.NoError(t, err)
	return gateway, "thunderid-" + strings.TrimPrefix(gateway, "thunderid-gateway-")
}

// markedZip builds a release zip holding pack.txt with the zip's own name, so a test can tell
// from the install which asset was downloaded.
func markedZip(t *testing.T, name string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	top := strings.TrimSuffix(name, ".zip") + "/"
	_, err := zw.Create(top)
	require.NoError(t, err)
	w, err := zw.Create(top + "pack.txt")
	require.NoError(t, err)
	_, err = w.Write([]byte(name))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// serveRelease serves a manifest listing one release, v<version>, with the named assets, and
// returns a client that reads it.
func serveRelease(t *testing.T, version string, assets ...string) *release.Client {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	listed := make([]map[string]string, 0, len(assets))
	for _, name := range assets {
		listed = append(listed, map[string]string{"name": name, "downloadUrl": srv.URL + "/" + name})
		body := markedZip(t, name)
		mux.HandleFunc("/"+name, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
	}
	entry := map[string]any{"tagName": "v" + version, "isLatest": true, "assets": listed}
	manifest, err := json.Marshal(map[string]any{"latestRelease": entry, "releases": []any{entry}})
	require.NoError(t, err)
	mux.HandleFunc("/releases.json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(manifest) })

	return &release.Client{ManifestURL: srv.URL + "/releases.json"}
}

// installedPack returns the name of the zip that was extracted into destDir.
func installedPack(t *testing.T, destDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(destDir, "pack.txt"))
	require.NoError(t, err)
	return string(data)
}

// A release that names the Data Plane pack thunderid-gateway-* also lists the quickstart pack as
// thunderid-<version>-*, which must not be what gets installed.
func TestDownload_InstallsTheGatewayPack(t *testing.T) {
	gateway, quickstart := packNames(t, "1.2.0")
	client := serveRelease(t, "1.2.0", quickstart, gateway, "sample-app-wayfinder-1.2.0.zip")
	dest := filepath.Join(t.TempDir(), "install")

	require.NoError(t, client.Download("1.2.0", dest, nil))
	assert.Equal(t, gateway, installedPack(t, dest))
}

// v1.1.0 and older list the Data Plane pack only as thunderid-<version>-*. They are the only
// releases published, so the CLI has to keep installing them.
func TestDownload_InstallsTheDataPlanePackOfAReleaseWithoutTheGatewayName(t *testing.T) {
	_, legacy := packNames(t, "1.1.0")
	client := serveRelease(t, "1.1.0", legacy, "sample-app-wayfinder-1.1.0.zip")
	dest := filepath.Join(t.TempDir(), "install")

	require.NoError(t, client.Download("1.1.0", dest, nil))
	assert.Equal(t, legacy, installedPack(t, dest))
}

// Once a release carries the gateway name, thunderid-<version>-* is the quickstart pack. A release
// missing this platform's gateway pack has to fail rather than install the quickstart pack, which
// runs a different product under the name of the one asked for.
func TestDownload_DoesNotInstallTheQuickstartPackWhenTheGatewayPackIsMissing(t *testing.T) {
	gateway, quickstart := packNames(t, "1.2.0")
	client := serveRelease(t, "1.2.0", quickstart, "thunderid-gateway-1.2.0-other-x64.zip")
	dest := filepath.Join(t.TempDir(), "install")

	err := client.Download("1.2.0", dest, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), gateway)
	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr), "nothing should have been installed")
}

// The error names the asset the CLI looked for: the older name for a release that lists no
// gateway asset, and the gateway name otherwise, including for a version the manifest lacks.
func TestDownload_FailsWhenNeitherNameIsListed(t *testing.T) {
	_, legacy := packNames(t, "1.1.0")
	unlisted, _ := packNames(t, "9.9.9")
	client := serveRelease(t, "1.1.0", "sample-app-wayfinder-1.1.0.zip")

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "release without the gateway name", version: "1.1.0", want: legacy},
		{name: "version not in the manifest", version: "9.9.9", want: unlisted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "install")

			err := client.Download(tt.version, dest, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no release asset found for "+tt.want+" ")
			_, statErr := os.Stat(dest)
			assert.True(t, os.IsNotExist(statErr), "nothing should have been installed")
		})
	}
}
