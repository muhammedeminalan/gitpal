// Package update finds gitpal's newest release on GitHub, picks the package for
// this platform and downloads it, verifying its SHA-256 before it is used.
//
// Nothing about the user is sent: the requests are anonymous reads of the public
// release list. A download that does not match its published checksum is deleted.
package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultAPI is the release list of gitpal's own repository.
const DefaultAPI = "https://api.github.com/repos/muhammedeminalan/gitpal"

// maxDownload caps a download, so a broken server cannot fill the disk.
const maxDownload = 512 << 20

var (
	// ErrNoChecksum means the release publishes no checksum for the package, so it
	// is not downloaded.
	ErrNoChecksum = errors.New("update: no checksum published for the package")
	// ErrChecksumMismatch means the downloaded file is not what was published.
	ErrChecksumMismatch = errors.New("update: checksum mismatch")
	// ErrNoPackage means the release has nothing for this platform.
	ErrNoPackage = errors.New("update: no package for this platform")
)

// Client talks to the release list.
type Client struct {
	// API is the repository's API address; DefaultAPI when empty.
	API  string
	HTTP *http.Client
}

// Asset is one downloadable file of a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Release is a published version.
type Release struct {
	Tag    string  `json:"tag_name"`
	Page   string  `json:"html_url"`
	Notes  string  `json:"body"`
	Assets []Asset `json:"assets"`
}

// Version is the release's version without the leading "v".
func (r Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }

func (r Release) asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// App returns the desktop app package for a platform: the .dmg on macOS, an
// archive holding gitpal-gui and gitpal elsewhere.
func (r Release) App(goos, goarch string) (Asset, bool) {
	v := r.Version()
	var name string
	switch goos {
	case "darwin":
		name = fmt.Sprintf("gitpal_%s_macos_%s.dmg", v, goarch)
	case "windows":
		name = fmt.Sprintf("gitpal-gui_%s_windows_%s.zip", v, goarch)
	default:
		name = fmt.Sprintf("gitpal-gui_%s_%s_%s.tar.gz", v, goos, goarch)
	}
	return r.asset(name)
}

// cliArchive returns the command-line tool's archive for a platform.
func (r Release) cliArchive(goos, goarch string) (Asset, bool) {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return r.asset(fmt.Sprintf("gitpal_%s_%s_%s%s", r.Version(), goos, goarch, ext))
}

var (
	semver = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)
	exact  = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)
)

// ReleaseTag returns the tag of a release build's version ("1.0.1" →
// "v1.0.1"). ok is false for anything else (a development build), which has
// no release of its own to download from.
func ReleaseTag(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if !exact.MatchString(v) {
		return "", false
	}
	return "v" + strings.TrimPrefix(v, "v"), true
}

// Newer reports whether latest is newer than current. known is false when
// current is not a release version (a development build), which cannot be
// compared. Text after the version number ("-3-gabc123-dirty", git describe's
// commits since the tag) counts as the same version.
func Newer(current, latest string) (newer, known bool) {
	c, ok := parse(current)
	if !ok {
		return false, false
	}
	l, ok := parse(latest)
	if !ok {
		return false, true
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i], true
		}
	}
	return false, true
}

func parse(v string) ([3]int, bool) {
	m := semver.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return [3]int{}, false
	}
	var out [3]int
	for i := range out {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

func (c Client) api() string {
	if c.API != "" {
		return strings.TrimRight(c.API, "/")
	}
	return DefaultAPI
}

func (c Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 10 * time.Minute}
}

// Latest returns the newest published release.
func (c Client) Latest(ctx context.Context) (Release, error) {
	return c.release(ctx, c.api()+"/releases/latest")
}

// ByTag returns the release of a tag ("v1.2.0").
func (c Client) ByTag(ctx context.Context, tag string) (Release, error) {
	return c.release(ctx, c.api()+"/releases/tags/"+tag)
}

func (c Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	// Anonim istek: kullanıcıya dair hiçbir bilgi gönderilmez.
	req.Header.Set("User-Agent", "gitpal-updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("update: %s: HTTP %d", req.URL.Path, resp.StatusCode)
	}
	return resp, nil
}

func (c Client) release(ctx context.Context, url string) (Release, error) {
	resp, err := c.get(ctx, url)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	var r Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&r); err != nil {
		return Release{}, fmt.Errorf("update: %w", err)
	}
	if r.Tag == "" {
		return Release{}, errors.New("update: release without a tag")
	}
	return r, nil
}

// DownloadVerified downloads an asset into dir and checks it against the
// "<name>.sha256" file the release publishes next to it.
func (c Client) DownloadVerified(ctx context.Context, rel Release, a Asset, dir string) (string, error) {
	sumAsset, ok := rel.asset(a.Name + ".sha256")
	if !ok {
		return "", ErrNoChecksum
	}
	want, err := c.checksum(ctx, sumAsset, a.Name)
	if err != nil {
		return "", err
	}
	return c.download(ctx, a, dir, want)
}

// checksum reads the hash for name from a sha256sum-style file.
func (c Client) checksum(ctx context.Context, file Asset, name string) (string, error) {
	resp, err := c.get(ctx, file.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == 64 {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", ErrNoChecksum
}

// download writes to a ".part" file while hashing, and renames it into place
// only when the hash matches.
func (c Client) download(ctx context.Context, a Asset, dir, want string) (string, error) {
	resp, err := c.get(ctx, a.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // indirme klasörü (Downloads) herkese açık izinlerle olağan
		return "", fmt.Errorf("update: %w", err)
	}
	final := filepath.Join(dir, filepath.Base(a.Name))
	part := final + ".part"
	f, err := os.Create(part) //nolint:gosec // ad, sürüm listesindeki dosya adının yalnızca son parçası
	if err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxDownload+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	switch {
	case err != nil:
		_ = os.Remove(part)
		return "", fmt.Errorf("update: %w", err)
	case n > maxDownload:
		_ = os.Remove(part)
		return "", errors.New("update: download too large")
	case hex.EncodeToString(h.Sum(nil)) != want:
		_ = os.Remove(part)
		return "", ErrChecksumMismatch
	}
	if err := os.Rename(part, final); err != nil {
		_ = os.Remove(part)
		return "", fmt.Errorf("update: %w", err)
	}
	return final, nil
}

// FetchCLI downloads the command-line tool of a release for a platform,
// verified against the release's checksums.txt, and puts the gitpal program
// (gitpal.exe on Windows) into dest. It returns the program's path.
func (c Client) FetchCLI(ctx context.Context, rel Release, goos, goarch, dest string) (string, error) {
	archive, ok := rel.cliArchive(goos, goarch)
	if !ok {
		return "", ErrNoPackage
	}
	sums, ok := rel.asset("checksums.txt")
	if !ok {
		return "", ErrNoChecksum
	}
	want, err := c.checksum(ctx, sums, archive.Name)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "gitpal-cli-*")
	if err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	defer os.RemoveAll(tmp)
	path, err := c.download(ctx, archive, tmp, want)
	if err != nil {
		return "", err
	}
	return extractCLI(path, goos, dest)
}

// extractCLI takes only the gitpal program at the root of the archive; nothing
// else is written, so no entry can land outside dest.
func extractCLI(archive, goos, dest string) (string, error) {
	name := "gitpal"
	if goos == "windows" {
		name += ".exe"
	}
	var (
		body io.Reader
		done func()
	)
	if strings.HasSuffix(archive, ".zip") {
		zr, err := zip.OpenReader(archive)
		if err != nil {
			return "", fmt.Errorf("update: %w", err)
		}
		defer zr.Close()
		for _, f := range zr.File {
			if f.Name == name {
				rc, err := f.Open()
				if err != nil {
					return "", fmt.Errorf("update: %w", err)
				}
				body, done = rc, func() { _ = rc.Close() }
				break
			}
		}
	} else {
		f, err := os.Open(archive) //nolint:gosec // az önce indirip doğruladığımız dosya
		if err != nil {
			return "", fmt.Errorf("update: %w", err)
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", fmt.Errorf("update: %w", err)
		}
		tr := tar.NewReader(gz)
		for {
			h, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return "", fmt.Errorf("update: %w", err)
			}
			if h.Name == name && h.Typeflag == tar.TypeReg {
				body, done = tr, func() {}
				break
			}
		}
	}
	if body == nil {
		return "", fmt.Errorf("update: %s not in %s", name, filepath.Base(archive))
	}
	defer done()
	if err := os.MkdirAll(dest, 0o755); err != nil { //nolint:gosec // program klasörü; başkalarının okuyabilmesi olağan
		return "", fmt.Errorf("update: %w", err)
	}
	out := filepath.Join(dest, name)
	tmp := out + ".new"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec // çalıştırılabilir program
	if err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	_, err = io.Copy(f, io.LimitReader(body, maxDownload))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, out)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("update: %w", err)
	}
	return out, nil
}
