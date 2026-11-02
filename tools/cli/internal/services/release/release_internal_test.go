// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package release

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A download cut off mid-transfer used to leave its partial file on disk — tens of
// megabytes nothing ever cleans up, and a truncated artifact a later run could pick up.
func TestDownloadFile_RemovesTruncatedFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		// Drop the connection so the client sees a truncated body.
		conn, _, err := http.NewResponseController(w).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "release.zip")
	err := downloadFile(srv.URL, dest, nil)

	require.Error(t, err, "a truncated body must fail the download")
	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr), "the partial download should have been removed")
	assert.Empty(t, dirEntryNames(t, filepath.Dir(dest)), "the temporary download file should be gone too")
}

// dirEntryNames lists dir so a test can assert nothing was left behind.
func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestDownloadFile_PreservesExistingFileAfterFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1024")
		_, _ = w.Write([]byte("partial"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "release.zip")
	require.NoError(t, os.WriteFile(dest, []byte("existing"), 0o644))

	require.Error(t, downloadFile(srv.URL, dest, nil))
	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "existing", string(data))
	assert.Equal(t, []string{"release.zip"}, dirEntryNames(t, filepath.Dir(dest)),
		"only the pre-existing file should remain")
}

func TestDownloadFile_KeepsCompleteFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("complete"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "release.zip")
	require.NoError(t, downloadFile(srv.URL, dest, nil))

	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "complete", string(data))
}

// unreachableURL returns the address of a server that is no longer listening, so a
// request to it fails at dial time without touching the network.
func unreachableURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()
	return url
}

func jsonServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The primary release host being down must not stop a startup: the GitHub API carries the
// same metadata under different field names.
func TestFetchReleasesData_FallsBackWhenPrimaryUnreachable(t *testing.T) {
	fallback := jsonServer(t, `{
		"tag_name": "v1.2.3",
		"assets": [{"name": "thunderid-1.2.3-linux-x64.zip", "browser_download_url": "https://example.test/a.zip"}]
	}`)

	data, err := fetchReleasesDataFrom(unreachableURL(t), fallback.URL)
	require.NoError(t, err)

	assert.Equal(t, "v1.2.3", data.LatestRelease.TagName)
	assert.True(t, data.LatestRelease.IsLatest)
	require.Len(t, data.LatestRelease.Assets, 1)
	assert.Equal(t, "thunderid-1.2.3-linux-x64.zip", data.LatestRelease.Assets[0].Name)
	assert.Equal(t, "https://example.test/a.zip", data.LatestRelease.Assets[0].DownloadURL,
		"browser_download_url must be mapped onto the asset download URL")
	assert.Equal(t, []releaseEntry{data.LatestRelease}, data.Releases,
		"the fallback release must also be listed as a release")
}

// An HTTP error from the primary host is a failure like any other and must fall back too.
func TestFetchReleasesData_FallsBackOnPrimaryHTTPError(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer primary.Close()
	fallback := jsonServer(t, `{"tag_name": "v9.9.9"}`)

	data, err := fetchReleasesDataFrom(primary.URL, fallback.URL)
	require.NoError(t, err)
	assert.Equal(t, "v9.9.9", data.LatestRelease.TagName)
}

func TestFetchReleasesData_UsesPrimaryWhenItWorks(t *testing.T) {
	primary := jsonServer(t, `{"latestRelease": {"tagName": "v2.0.0", "isLatest": true}}`)
	fallbackHits := 0
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fallbackHits++
		_, _ = w.Write([]byte(`{"tag_name": "v0.0.0"}`))
	}))
	defer fallback.Close()

	data, err := fetchReleasesDataFrom(primary.URL, fallback.URL)
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", data.LatestRelease.TagName)
	assert.Zero(t, fallbackHits, "the fallback must not be hit when the primary answers")
}

// Blaming only the fallback hides which host actually started the failure, so both errors
// have to survive into the message.
func TestFetchReleasesData_ReportsBothHostsWhenAllFail(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer fallback.Close()

	data, err := fetchReleasesDataFrom(primary.URL, fallback.URL)

	require.Error(t, err)
	assert.Nil(t, data)
	msg := err.Error()
	assert.Contains(t, msg, primary.URL, "the primary host must be named")
	assert.Contains(t, msg, fallback.URL, "the fallback host must be named")
	assert.Contains(t, msg, "500", "the primary status must survive")
	assert.Contains(t, msg, "403", "the fallback status must survive")
}

// A fallback that answers but omits the tag is a bad response, not a working one.
func TestFetchReleasesData_RejectsFallbackWithoutTag(t *testing.T) {
	fallback := jsonServer(t, `{"assets": []}`)

	data, err := fetchReleasesDataFrom(unreachableURL(t), fallback.URL)
	require.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "tag_name")
}

// A failed extraction used to leave a half-written install directory behind.
func TestExtractInto_RemovesDirectoryItCreated(t *testing.T) {
	badZip := filepath.Join(t.TempDir(), "broken.zip")
	require.NoError(t, os.WriteFile(badZip, []byte("not a zip"), 0o644))
	dest := filepath.Join(t.TempDir(), "v1.0.0")

	require.Error(t, extractInto(badZip, dest))

	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr), "the half-written install should have been removed")
}

// os.Stat reports a dangling symlink as missing, which used to make this run
// claim ownership of it and delete it on failure.
func TestExtractInto_KeepsDanglingSymlink(t *testing.T) {
	badZip := filepath.Join(t.TempDir(), "broken.zip")
	require.NoError(t, os.WriteFile(badZip, []byte("not a zip"), 0o644))
	dest := filepath.Join(t.TempDir(), "v1.0.0")

	// Point at an existing directory first, so Windows makes a directory symlink,
	// then remove the target to leave the link dangling.
	target := filepath.Join(t.TempDir(), "target")
	require.NoError(t, os.Mkdir(target, 0o755))
	if err := os.Symlink(target, dest); err != nil {
		t.Skipf("symlinks unavailable on this machine: %v", err) // Windows without the privilege
	}
	require.NoError(t, os.RemoveAll(target))

	require.Error(t, extractInto(badZip, dest))

	info, err := os.Lstat(dest)
	require.NoError(t, err, "a pre-existing symlink must be left alone")
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "it must still be the symlink")
	link, err := os.Readlink(dest)
	require.NoError(t, err)
	assert.Equal(t, target, link, "it must still point where it did")
}

// A directory that was already there is not this run's to delete.
func TestExtractInto_KeepsPreexistingDirectory(t *testing.T) {
	badZip := filepath.Join(t.TempDir(), "broken.zip")
	require.NoError(t, os.WriteFile(badZip, []byte("not a zip"), 0o644))
	dest := t.TempDir()
	keep := filepath.Join(dest, "keep.txt")
	require.NoError(t, os.WriteFile(keep, []byte("x"), 0o644))

	require.Error(t, extractInto(badZip, dest))

	_, err := os.Stat(keep)
	assert.NoError(t, err, "a pre-existing install must be left alone")
}

// findAsset used to fall back to the latest release's asset when the requested version was not
// in the manifest. Nothing could request a specific version, so it never mattered; with
// --product-version it would install the latest under the name of the version that was asked
// for, which is worse than failing.
func TestFindAsset_DoesNotSubstituteAnotherRelease(t *testing.T) {
	data := &releasesData{
		LatestRelease: releaseEntry{
			TagName: "v2.0.0",
			Assets:  []releaseAsset{{Name: "thunderid-2.0.0-linux-x64.zip", DownloadURL: "https://example.com/2.0.0"}},
		},
		Releases: []releaseEntry{
			{
				TagName: "v1.0.0",
				Assets:  []releaseAsset{{Name: "thunderid-1.0.0-linux-x64.zip", DownloadURL: "https://example.com/1.0.0"}},
			},
		},
	}

	found := findAsset(data, "1.0.0", "thunderid-1.0.0-linux-x64.zip")
	require.NotNil(t, found, "an asset that exists should be found")
	assert.Equal(t, "https://example.com/1.0.0", found.DownloadURL)

	assert.Nil(t, findAsset(data, "9.9.9", "thunderid-9.9.9-linux-x64.zip"),
		"a version the manifest does not carry must not resolve to another release")
}

// The latest release is only listed under latestRelease in the manifest, so asking for it by
// version has to match there too.
func TestFindAsset_MatchesTheLatestReleaseByVersion(t *testing.T) {
	data := &releasesData{
		LatestRelease: releaseEntry{
			TagName: "v2.0.0",
			Assets:  []releaseAsset{{Name: "thunderid-2.0.0-linux-x64.zip", DownloadURL: "https://example.com/2.0.0"}},
		},
	}

	found := findAsset(data, "2.0.0", "thunderid-2.0.0-linux-x64.zip")
	require.NotNil(t, found)
	assert.Equal(t, "https://example.com/2.0.0", found.DownloadURL)
}

// A custom source is authoritative: reaching for the public GitHub API behind the operator's back
// would fetch something they did not point at.
func TestFetchReleasesData_NoFallbackWhenDisabled(t *testing.T) {
	_, err := fetchReleasesDataFrom("http://127.0.0.1:1/releases.json", "")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "fallback", "an empty fallback must not be attempted")
}
