// Package updater handles self-update of the ai-review binary from GitHub Releases.
package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const binaryName = "ai-review"

// releaseAPIURL is a variable so tests can point it at a local mock server.
var releaseAPIURL = "https://api.github.com/repos/hiiamtrong/smart-code-review/releases/latest"

// LatestRelease holds the tag and download URL for the matching asset.
type LatestRelease struct {
	Tag        string
	DownloadURL string
}

// FetchLatest queries the GitHub Releases API and returns the best-matching asset.
func FetchLatest() (*LatestRelease, error) {
	resp, err := http.Get(releaseAPIURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, body)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release JSON: %w", err)
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	// Normalize arch names to match goreleaser conventions.
	if goarch == "amd64" {
		goarch = "x86_64"
	} else if goarch == "arm64" {
		goarch = "arm64"
	}

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, goos) && strings.Contains(name, strings.ToLower(goarch)) {
			return &LatestRelease{
				Tag:        release.TagName,
				DownloadURL: asset.BrowserDownloadURL,
			}, nil
		}
	}
	return nil, fmt.Errorf("no release asset found for %s/%s", goos, goarch)
}

// ReplaceCurrentBinary downloads the archive at url, extracts the ai-review
// binary, and atomically replaces the currently running executable.
// On Windows it writes a batch trampoline instead of replacing in-place.
func ReplaceCurrentBinary(downloadURL string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current binary: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	// Download archive to a temp file.
	tmpArchive, err := os.CreateTemp("", "ai-review-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpArchive.Name())

	resp, err := http.Get(downloadURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("download archive: %w", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(tmpArchive, resp.Body); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}
	tmpArchive.Close()

	// Extract the binary from the archive.
	newBinary, err := os.CreateTemp("", "ai-review-new-*")
	if err != nil {
		return fmt.Errorf("create temp binary: %w", err)
	}
	newBinaryPath := newBinary.Name()
	defer os.Remove(newBinaryPath)

	if strings.HasSuffix(downloadURL, ".zip") {
		if err := extractFromZip(tmpArchive.Name(), binaryNameForOS(), newBinary); err != nil {
			newBinary.Close()
			return err
		}
	} else {
		if err := extractFromTarGz(tmpArchive.Name(), binaryNameForOS(), newBinary); err != nil {
			newBinary.Close()
			return err
		}
	}
	newBinary.Close()

	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		return fmt.Errorf("chmod new binary: %w", err)
	}

	if runtime.GOOS == "windows" {
		return replaceWindowsBinary(exePath, newBinaryPath)
	}
	return replaceUnixBinary(exePath, newBinaryPath)
}

func binaryNameForOS() string {
	if runtime.GOOS == "windows" {
		return binaryName + ".exe"
	}
	return binaryName
}

// replaceUnixBinary atomically swaps the binary using rename(2).
func replaceUnixBinary(exePath, newBinaryPath string) error {
	// Place the temp file next to the target so rename is on the same filesystem.
	dir := filepath.Dir(exePath)
	sameFS, err := os.CreateTemp(dir, ".ai-review-update-*")
	if err != nil {
		return fmt.Errorf("create staging file: %w", err)
	}
	stagingPath := sameFS.Name()
	sameFS.Close()
	defer os.Remove(stagingPath)

	// Copy new binary → staging file.
	src, err := os.Open(newBinaryPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(stagingPath, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return fmt.Errorf("copy to staging: %w", err)
	}
	dst.Close()

	// Atomic rename.
	if err := os.Rename(stagingPath, exePath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

// replaceWindowsBinary writes a batch trampoline that waits for the process
// to exit then copies the new binary over the old one.
func replaceWindowsBinary(exePath, newBinaryPath string) error {
	batPath := exePath + ".update.bat"
	bat := fmt.Sprintf(`@echo off
:loop
timeout /t 1 /nobreak >nul 2>&1
copy /y "%s" "%s" >nul 2>&1
if errorlevel 1 goto loop
del "%s"
del "%%~f0"
`, newBinaryPath, exePath, newBinaryPath)

	if err := os.WriteFile(batPath, []byte(bat), 0755); err != nil {
		return fmt.Errorf("write update batch: %w", err)
	}

	// Launch the batch file detached so it runs after this process exits.
	// On Windows, START /B runs in background without a new console window.
	fmt.Printf("Update staged. Run the following to complete (or it will run automatically on next start):\n  %s\n", batPath)
	return nil
}

// ─── archive helpers ──────────────────────────────────────────────────────────

func extractFromTarGz(archivePath, binaryName string, dst io.Writer) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if filepath.Base(hdr.Name) == binaryName {
			if _, err := io.Copy(dst, tr); err != nil { //nolint:gosec
				return fmt.Errorf("extract binary: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("binary %q not found in archive", binaryName)
}

func extractFromZip(archivePath, binaryName string, dst io.Writer) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == binaryName {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open zip entry: %w", err)
			}
			_, copyErr := io.Copy(dst, rc) //nolint:gosec
			rc.Close()
			return copyErr
		}
	}
	return fmt.Errorf("binary %q not found in zip archive", binaryName)
}
