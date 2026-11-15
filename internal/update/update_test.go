package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		newer, known    bool
	}{
		{"1.0.1", "v1.0.2", true, true},
		{"v1.0.1", "1.0.1", false, true},
		{"1.2.0", "1.10.0", true, true},
		{"2.0.0", "1.9.9", false, true},
		// git describe çıktısı: etiketten sonraki commit'ler aynı sürüm sayılır.
		{"v1.0.1-3-g858f660-dirty", "v1.0.1", false, true},
		{"v1.0.1-3-g858f660", "v1.0.2", true, true},
		// Geliştirme derlemesi: sürümü bilinmez, kıyaslanamaz.
		{"dev", "v1.0.1", false, false},
		{"858f660", "v1.0.1", false, false},
	} {
		newer, known := Newer(tc.current, tc.latest)
		assert.Equal(t, tc.newer, newer, "%s → %s", tc.current, tc.latest)
		assert.Equal(t, tc.known, known, "%s", tc.current)
	}
}

func sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// fakeGitHub, gerçek API'nin biçiminde bir sürüm ve dosyaları sunar.
func fakeGitHub(t *testing.T, tag string, files map[string][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/o/r/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q,"html_url":"https://example.com/rel","body":"notlar","assets":[`, tag)
		first := true
		for name := range files {
			if !first {
				fmt.Fprint(w, ",")
			}
			first = false
			fmt.Fprintf(w, `{"name":%q,"browser_download_url":"%s/dl/%s","size":%d}`, name, srv.URL, name, len(files[name]))
		}
		fmt.Fprint(w, `]}`)
	})
	mux.HandleFunc(fmt.Sprintf("/repos/o/r/releases/tags/%s", tag), func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/repos/o/r/releases/latest"
		mux.ServeHTTP(w, r)
	})
	mux.HandleFunc("/dl/", func(w http.ResponseWriter, r *http.Request) {
		b, ok := files[filepath.Base(r.URL.Path)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestLatestAndDownloadTheAppForThisPlatform(t *testing.T) {
	dmg := []byte("dmg içeriği")
	srv := fakeGitHub(t, "v1.2.0", map[string][]byte{
		"gitpal_1.2.0_macos_arm64.dmg":        dmg,
		"gitpal_1.2.0_macos_arm64.dmg.sha256": []byte(sum(dmg) + "  gitpal_1.2.0_macos_arm64.dmg\n"),
		"gitpal-gui_1.2.0_windows_amd64.zip":  []byte("zip"),
	})
	c := Client{API: srv.URL + "/repos/o/r", HTTP: srv.Client()}
	rel, err := c.Latest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "v1.2.0", rel.Tag)
	assert.Equal(t, "notlar", rel.Notes)
	assert.Equal(t, "https://example.com/rel", rel.Page)

	app, ok := rel.App("darwin", "arm64")
	require.True(t, ok)
	assert.Equal(t, "gitpal_1.2.0_macos_arm64.dmg", app.Name)
	_, ok = rel.App("darwin", "amd64")
	assert.False(t, ok, "Intel Mac için paket yok")

	dir := t.TempDir()
	path, err := c.DownloadVerified(context.Background(), rel, app, dir)
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, dmg, got)
	assert.Equal(t, filepath.Join(dir, "gitpal_1.2.0_macos_arm64.dmg"), path)

	// Sağlama dosyası olmayan paket indirilmez.
	win, ok := rel.App("windows", "amd64")
	require.True(t, ok)
	_, err = c.DownloadVerified(context.Background(), rel, win, dir)
	assert.ErrorIs(t, err, ErrNoChecksum)
}

func TestTamperedDownloadIsRejectedAndRemoved(t *testing.T) {
	srv := fakeGitHub(t, "v1.2.0", map[string][]byte{
		"gitpal_1.2.0_macos_arm64.dmg":        []byte("değiştirilmiş"),
		"gitpal_1.2.0_macos_arm64.dmg.sha256": []byte(sum([]byte("asıl")) + "  gitpal_1.2.0_macos_arm64.dmg\n"),
	})
	c := Client{API: srv.URL + "/repos/o/r", HTTP: srv.Client()}
	rel, err := c.Latest(context.Background())
	require.NoError(t, err)
	app, _ := rel.App("darwin", "arm64")
	dir := t.TempDir()
	_, err = c.DownloadVerified(context.Background(), rel, app, dir)
	assert.ErrorIs(t, err, ErrChecksumMismatch)
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries, "doğrulanmayan dosya diskte kalmaz")
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestFetchCLIFromTheReleaseArchive(t *testing.T) {
	archive := tarGz(t, map[string]string{"README.md": "x", "gitpal": "#!/bin/sh\necho gitpal\n"})
	srv := fakeGitHub(t, "v1.2.0", map[string][]byte{
		"gitpal_1.2.0_linux_amd64.tar.gz": archive,
		"checksums.txt":                   []byte(sum(archive) + "  gitpal_1.2.0_linux_amd64.tar.gz\n" + sum([]byte("x")) + "  other.zip\n"),
	})
	c := Client{API: srv.URL + "/repos/o/r", HTTP: srv.Client()}
	rel, err := c.ByTag(context.Background(), "v1.2.0")
	require.NoError(t, err)
	dest := t.TempDir()
	path, err := c.FetchCLI(context.Background(), rel, "linux", "amd64", dest)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dest, "gitpal"), path)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(b), "echo gitpal")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100, "çalıştırılabilir")
}

func TestFetchCLIFromAZipOnWindows(t *testing.T) {
	archive := zipOf(t, map[string]string{"gitpal.exe": "MZ"})
	srv := fakeGitHub(t, "v1.2.0", map[string][]byte{
		"gitpal_1.2.0_windows_amd64.zip": archive,
		"checksums.txt":                  []byte(sum(archive) + "  gitpal_1.2.0_windows_amd64.zip\n"),
	})
	c := Client{API: srv.URL + "/repos/o/r", HTTP: srv.Client()}
	rel, err := c.Latest(context.Background())
	require.NoError(t, err)
	path, err := c.FetchCLI(context.Background(), rel, "windows", "amd64", t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "gitpal.exe", filepath.Base(path))
}

func TestArchiveCannotEscapeTheDestination(t *testing.T) {
	archive := tarGz(t, map[string]string{"../../gitpal": "kötü"})
	dest := t.TempDir()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	require.NoError(t, os.WriteFile(p, archive, 0o600))
	_, err := extractCLI(p, "linux", dest)
	assert.Error(t, err, "yalnızca arşivin kökündeki gitpal alınır")
}

func TestReleaseTagOnlyForExactVersions(t *testing.T) {
	tag, ok := ReleaseTag("1.0.1")
	assert.True(t, ok)
	assert.Equal(t, "v1.0.1", tag)
	tag, ok = ReleaseTag("v2.3.4")
	assert.True(t, ok)
	assert.Equal(t, "v2.3.4", tag)
	for _, v := range []string{"dev", "v1.0.1-3-g858f660", "858f660", "1.0"} {
		_, ok = ReleaseTag(v)
		assert.False(t, ok, v)
	}
}
