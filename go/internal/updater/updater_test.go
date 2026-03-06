package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// test constants to satisfy SonarQube S1192 (duplicated literals)
const (
	testTagV200        = "v2.0.0"
	testTagV300        = "v3.0.0"
	testTempPrefix     = "ai-review-new"
	testBinaryContent  = "new-binary"
	testContentFmt     = "content: got %q, want %q"
	testUnexpectedErr  = "unexpected error: %v"
	testBinContent     = "binary-content"
	testZipBinContent  = "zip-binary-content"
	testArchivePath    = "/archive.tar.gz"
	testBinName        = "ai-review"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// buildTarGz creates an in-memory .tar.gz containing a single file named
// entryName with the given content.
func buildTarGz(t *testing.T, entryName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	data := []byte(content)
	hdr := &tar.Header{
		Name: entryName,
		Mode: 0755,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

// buildZip creates an in-memory .zip containing a single file.
func buildZip(t *testing.T, entryName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(entryName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	return buf.Bytes()
}

// releaseServer creates a mock GitHub Releases API server returning the given
// assets list. tagName is the release tag (e.g. "v1.2.3").
func releaseServer(t *testing.T, tagName string, assets []map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type asset struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}
		resp := struct {
			TagName string  `json:"tag_name"`
			Assets  []asset `json:"assets"`
		}{TagName: tagName}
		for _, a := range assets {
			resp.Assets = append(resp.Assets, asset{
				Name:               a["name"],
				BrowserDownloadURL: a["url"],
			})
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
}

// ─── FetchLatest ─────────────────────────────────────────────────────────────

// ─── FetchLatest (uses package-level releaseAPIURL var) ──────────────────────

func TestFetchLatest_Success(t *testing.T) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	assetName := fmt.Sprintf("ai-review_%s_%s.tar.gz", goos, goarch)
	assetURL := "http://example.com/download/" + assetName

	srv := releaseServer(t, testTagV200, []map[string]string{
		{"name": assetName, "url": assetURL},
	})
	defer srv.Close()

	old := releaseAPIURL
	releaseAPIURL = srv.URL
	defer func() { releaseAPIURL = old }()

	rel, err := FetchLatest()
	if err != nil {
		t.Fatalf("FetchLatest: %v", err)
	}
	if rel.Tag != testTagV200 {
		t.Errorf("tag: got %q, want %q", rel.Tag, testTagV200)
	}
}

func TestFetchLatest_NoMatchingAsset(t *testing.T) {
	srv := releaseServer(t, "v1.0.0", []map[string]string{
		{"name": "ai-review_unknownos_unknownarch.tar.gz", "url": "http://x.com/a.tar.gz"},
	})
	defer srv.Close()

	old := releaseAPIURL
	releaseAPIURL = srv.URL
	defer func() { releaseAPIURL = old }()

	_, err := FetchLatest()
	if err == nil {
		t.Fatal("expected error for no matching asset")
	}
}

func TestFetchLatest_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	old := releaseAPIURL
	releaseAPIURL = srv.URL
	defer func() { releaseAPIURL = old }()

	_, err := FetchLatest()
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestFetchLatest_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json")) //nolint:errcheck
	}))
	defer srv.Close()

	old := releaseAPIURL
	releaseAPIURL = srv.URL
	defer func() { releaseAPIURL = old }()

	_, err := FetchLatest()
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

// ─── ReplaceCurrentBinary ────────────────────────────────────────────────────

func TestReplaceCurrentBinary_DownloadError(t *testing.T) {
	// Use a server that immediately closes to force a download error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusGone)
	}))
	srv.Close() // closed right away so the HTTP client gets a connection error

	err := ReplaceCurrentBinary(srv.URL + testArchivePath)
	if err == nil {
		t.Fatal("expected download error")
	}
}

// ─── replaceUnixBinary ────────────────────────────────────────────────────────

func TestReplaceUnixBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("replaceUnixBinary is Unix-only")
	}

	dir := t.TempDir()
	exePath := filepath.Join(dir, testBinName)
	newBinPath := filepath.Join(dir, testTempPrefix)

	if err := os.WriteFile(exePath, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBinPath, []byte(testBinaryContent), 0755); err != nil {
		t.Fatal(err)
	}

	if err := replaceUnixBinary(exePath, newBinPath); err != nil {
		t.Fatalf("replaceUnixBinary: %v", err)
	}

	data, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(data) != testBinaryContent {
		t.Errorf(testContentFmt, data, testBinaryContent)
	}

	// Verify the replaced binary is executable (regression: CreateTemp
	// creates 0600 files; without explicit chmod the binary loses +x).
	info, err := os.Stat(exePath)
	if err != nil {
		t.Fatalf("stat replaced binary: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("replaced binary not executable: mode %o", info.Mode().Perm())
	}
}

// TestReplaceUnixBinary_SourcePerm0600 reproduces the real-world bug where
// os.CreateTemp creates the new binary with 0600 permissions. Without the
// explicit os.Chmod in replaceUnixBinary, the installed binary would inherit
// the 0600 mode via os.OpenFile (which ignores the mode arg for existing
// files) and become non-executable after os.Rename.
func TestReplaceUnixBinary_SourcePerm0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("replaceUnixBinary is Unix-only")
	}

	dir := t.TempDir()
	exePath := filepath.Join(dir, testBinName)
	newBinPath := filepath.Join(dir, testTempPrefix)

	// Old binary is executable (simulates the currently-installed binary).
	if err := os.WriteFile(exePath, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	// New binary has 0600 — exactly what os.CreateTemp produces.
	if err := os.WriteFile(newBinPath, []byte(testBinaryContent), 0600); err != nil {
		t.Fatal(err)
	}

	if err := replaceUnixBinary(exePath, newBinPath); err != nil {
		t.Fatalf("replaceUnixBinary: %v", err)
	}

	data, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(data) != testBinaryContent {
		t.Errorf(testContentFmt, data, testBinaryContent)
	}

	info, err := os.Stat(exePath)
	if err != nil {
		t.Fatalf("stat replaced binary: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("replaced binary should be 0755, got %o", info.Mode().Perm())
	}
}

// legacy helper kept for existing tests below — delegates to FetchLatest.
func fetchLatestFromURL(apiURL string) (*LatestRelease, error) {
	old := releaseAPIURL
	releaseAPIURL = apiURL
	defer func() { releaseAPIURL = old }()
	return FetchLatest()
}

func TestFetchLatestFromURL_Success(t *testing.T) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	assetName := fmt.Sprintf("ai-review_%s_%s.tar.gz", goos, goarch)
	assetURL := "http://example.com/ai-review.tar.gz"

	srv := releaseServer(t, testTagV300, []map[string]string{
		{"name": assetName, "url": assetURL},
	})
	defer srv.Close()

	rel, err := fetchLatestFromURL(srv.URL)
	if err != nil {
		t.Fatalf(testUnexpectedErr, err)
	}
	if rel.Tag != testTagV300 {
		t.Errorf("tag: got %q, want %q", rel.Tag, testTagV300)
	}
	if rel.DownloadURL != assetURL {
		t.Errorf("download URL: got %q, want %q", rel.DownloadURL, assetURL)
	}
}

func TestFetchLatestFromURL_NoMatchingAsset(t *testing.T) {
	srv := releaseServer(t, "v1.0.0", []map[string]string{
		{"name": "ai-review_unknownos_unknownarch.tar.gz", "url": "http://x.com/a.tar.gz"},
	})
	defer srv.Close()

	_, err := fetchLatestFromURL(srv.URL)
	if err == nil {
		t.Fatal("expected error for no matching asset")
	}
}

func TestFetchLatestFromURL_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchLatestFromURL(srv.URL)
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestFetchLatestFromURL_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json")) //nolint:errcheck
	}))
	defer srv.Close()

	_, err := fetchLatestFromURL(srv.URL)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

// ─── extractFromTarGz ────────────────────────────────────────────────────────

func TestExtractFromTarGz_Found(t *testing.T) {
	binaryName := binaryNameForOS()
	archive := buildTarGz(t, binaryName, testBinContent)

	archivePath := filepath.Join(t.TempDir(), "test.tar.gz")
	if err := os.WriteFile(archivePath, archive, 0600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := extractFromTarGz(archivePath, binaryName, &out); err != nil {
		t.Fatalf("extractFromTarGz: %v", err)
	}
	if out.String() != testBinContent {
		t.Errorf(testContentFmt, out.String(), testBinContent)
	}
}

func TestExtractFromTarGz_NotFound(t *testing.T) {
	archive := buildTarGz(t, "other-file", "data")
	archivePath := filepath.Join(t.TempDir(), "test.tar.gz")
	os.WriteFile(archivePath, archive, 0600) //nolint:errcheck

	var out bytes.Buffer
	err := extractFromTarGz(archivePath, testBinName, &out)
	if err == nil {
		t.Fatal("expected error when binary not in archive")
	}
}

func TestExtractFromTarGz_BadFile(t *testing.T) {
	var out bytes.Buffer
	err := extractFromTarGz("/nonexistent/path.tar.gz", testBinName, &out)
	if err == nil {
		t.Fatal("expected error for missing archive file")
	}
}

func TestExtractFromTarGz_BadGzip(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "bad.tar.gz")
	os.WriteFile(archivePath, []byte("not-gzip"), 0600) //nolint:errcheck

	var out bytes.Buffer
	err := extractFromTarGz(archivePath, testBinName, &out)
	if err == nil {
		t.Fatal("expected error for invalid gzip")
	}
}

// ─── extractFromZip ──────────────────────────────────────────────────────────

func TestExtractFromZip_Found(t *testing.T) {
	binaryName := binaryNameForOS()
	archive := buildZip(t, binaryName, testZipBinContent)

	archivePath := filepath.Join(t.TempDir(), "test.zip")
	if err := os.WriteFile(archivePath, archive, 0600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := extractFromZip(archivePath, binaryName, &out); err != nil {
		t.Fatalf("extractFromZip: %v", err)
	}
	if out.String() != testZipBinContent {
		t.Errorf(testContentFmt, out.String(), testZipBinContent)
	}
}

func TestExtractFromZip_NotFound(t *testing.T) {
	archive := buildZip(t, "other-file", "data")
	archivePath := filepath.Join(t.TempDir(), "test.zip")
	os.WriteFile(archivePath, archive, 0600) //nolint:errcheck

	var out bytes.Buffer
	err := extractFromZip(archivePath, testBinName, &out)
	if err == nil {
		t.Fatal("expected error when binary not in zip")
	}
}

func TestExtractFromZip_BadFile(t *testing.T) {
	var out bytes.Buffer
	err := extractFromZip("/nonexistent/path.zip", testBinName, &out)
	if err == nil {
		t.Fatal("expected error for missing zip file")
	}
}

// ─── binaryNameForOS ─────────────────────────────────────────────────────────

func TestBinaryNameForOS(t *testing.T) {
	name := binaryNameForOS()
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(name, ".exe") {
			t.Errorf("Windows binary name should end with .exe; got %q", name)
		}
	} else {
		if strings.HasSuffix(name, ".exe") {
			t.Errorf("Non-Windows binary name should not end with .exe; got %q", name)
		}
	}
}

// ─── ReplaceCurrentBinary (happy path with mock HTTP server) ─────────────────

func TestReplaceCurrentBinary_TarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses Unix binary replacement")
	}

	// Build a real tar.gz archive containing the expected binary name.
	binName := binaryNameForOS()
	archiveData := buildTarGz(t, binName, "updated-binary-content")

	// Serve the archive via httptest.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(archiveData) //nolint:errcheck
	}))
	defer srv.Close()

	// Create a fake "current executable" so ReplaceCurrentBinary can replace it.
	dir := t.TempDir()
	fakeExe := filepath.Join(dir, testBinName)
	if err := os.WriteFile(fakeExe, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	// Patch os.Executable by creating a symlink and using that path isn't
	// feasible, but ReplaceCurrentBinary calls os.Executable() directly.
	// Instead, we create a thin wrapper that replaces the binary at a known path.
	// We'll test the core logic by calling the internal pieces that
	// ReplaceCurrentBinary orchestrates — download, extract, replace.
	//
	// Actually we CAN test ReplaceCurrentBinary end-to-end by building a
	// temporary test binary and running it. But a simpler approach: create a
	// small Go program, compile it to a temp dir, then point the test at it.
	//
	// Simplest workable approach: compile a trivial binary, run
	// ReplaceCurrentBinary pointing at our httptest server. os.Executable()
	// will return the test binary itself, which is fine — we just need the
	// function to succeed.

	// The download URL must NOT end in ".zip" to hit the tar.gz path.
	downloadURL := srv.URL + testArchivePath

	err := ReplaceCurrentBinary(downloadURL)
	// This will try to replace the test binary itself. It may or may not
	// succeed depending on OS protections, but it exercises all the code paths.
	// On macOS/Linux it should succeed.
	if err != nil {
		// If it fails, it should be at the replace step, not earlier.
		// Accept the error only if it's about replacing the binary (permission).
		if !strings.Contains(err.Error(), "replace binary") &&
			!strings.Contains(err.Error(), "text file busy") &&
			!strings.Contains(err.Error(), "permission denied") &&
			!strings.Contains(err.Error(), "operation not permitted") {
			t.Fatalf(testUnexpectedErr, err)
		}
	}
}

func TestReplaceCurrentBinary_Zip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses Unix binary replacement")
	}

	binName := binaryNameForOS()
	archiveData := buildZip(t, binName, "updated-zip-binary")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(archiveData) //nolint:errcheck
	}))
	defer srv.Close()

	// URL must end in ".zip" to hit the zip extraction path.
	downloadURL := srv.URL + "/archive.zip"

	err := ReplaceCurrentBinary(downloadURL)
	if err != nil {
		if !strings.Contains(err.Error(), "replace binary") &&
			!strings.Contains(err.Error(), "text file busy") &&
			!strings.Contains(err.Error(), "permission denied") &&
			!strings.Contains(err.Error(), "operation not permitted") {
			t.Fatalf(testUnexpectedErr, err)
		}
	}
}

func TestReplaceCurrentBinary_BadArchiveContent(t *testing.T) {
	// Serve something that isn't a valid tar.gz.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-a-real-archive")) //nolint:errcheck
	}))
	defer srv.Close()

	err := ReplaceCurrentBinary(srv.URL + testArchivePath)
	if err == nil {
		t.Fatal("expected error for invalid archive content")
	}
}

func TestReplaceCurrentBinary_BinaryNotInArchive(t *testing.T) {
	// Build a tar.gz that does NOT contain the expected binary name.
	archiveData := buildTarGz(t, "wrong-name", "some-content")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archiveData) //nolint:errcheck
	}))
	defer srv.Close()

	err := ReplaceCurrentBinary(srv.URL + testArchivePath)
	if err == nil {
		t.Fatal("expected error when binary not found in archive")
	}
	if !strings.Contains(err.Error(), "not found in archive") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestReplaceCurrentBinary_BadZipContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-a-zip")) //nolint:errcheck
	}))
	defer srv.Close()

	err := ReplaceCurrentBinary(srv.URL + "/archive.zip")
	if err == nil {
		t.Fatal("expected error for invalid zip content")
	}
}

// ─── replaceUnixBinary edge cases ────────────────────────────────────────────

func TestReplaceUnixBinary_BadSourcePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only")
	}

	dir := t.TempDir()
	exePath := filepath.Join(dir, testBinName)
	if err := os.WriteFile(exePath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	err := replaceUnixBinary(exePath, "/nonexistent/path/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestReplaceUnixBinary_BadExeDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only")
	}

	dir := t.TempDir()
	newBinPath := filepath.Join(dir, testBinaryContent)
	if err := os.WriteFile(newBinPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	// exePath in a nonexistent directory — CreateTemp should fail.
	err := replaceUnixBinary("/nonexistent/dir/ai-review", newBinPath)
	if err == nil {
		t.Fatal("expected error for nonexistent exe directory")
	}
	if !strings.Contains(err.Error(), "create staging file") {
		t.Errorf(testUnexpectedErr, err)
	}
}

// ─── replaceWindowsBinary ────────────────────────────────────────────────────

func TestReplaceWindowsBinary_WritesBatchFile(t *testing.T) {
	dir := t.TempDir()
	exePath := filepath.Join(dir, "ai-review.exe")
	newBinPath := filepath.Join(dir, testTempPrefix)

	// Create dummy files so the function has something to reference.
	os.WriteFile(exePath, []byte("old"), 0755)   //nolint:errcheck
	os.WriteFile(newBinPath, []byte("new"), 0755) //nolint:errcheck

	if err := replaceWindowsBinary(exePath, newBinPath); err != nil {
		t.Fatalf("replaceWindowsBinary: %v", err)
	}

	batPath := exePath + ".update.bat"
	data, err := os.ReadFile(batPath)
	if err != nil {
		t.Fatalf("batch file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, newBinPath) {
		t.Errorf("batch file missing new binary path; got: %q", content)
	}
	if !strings.Contains(content, exePath) {
		t.Errorf("batch file missing exe path; got: %q", content)
	}
}
