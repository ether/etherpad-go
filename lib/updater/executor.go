package updater

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.uber.org/zap"
)

// ExecResult is the outcome of an apply attempt's file operations. The
// orchestrator turns OK into a pending-verification state + exit(75), and a
// failure into a rollback.
type ExecResult struct {
	OK         bool
	BackupPath string
	Reason     string
}

// Executor downloads, verifies and atomically swaps in the new release binary.
type Executor struct {
	checker  *VersionChecker
	exePath  string
	verifier SignatureVerifier
	logger   *zap.SugaredLogger
}

// NewExecutor builds an executor targeting exePath (the running executable).
func NewExecutor(checker *VersionChecker, exePath string, verifier SignatureVerifier, logger *zap.SugaredLogger) *Executor {
	return &Executor{checker: checker, exePath: exePath, verifier: verifier, logger: logger}
}

func execFail(reason string) ExecResult { return ExecResult{Reason: reason} }

// Apply performs the full download → verify → replace sequence. On success the
// previous binary is preserved at the returned BackupPath for rollback.
func (e *Executor) Apply(target ReleaseInfo) ExecResult {
	rel, err := e.checker.fetchReleaseByTag(target.Tag)
	if err != nil {
		return execFail("fetch-release-failed")
	}
	asset, ok := matchAsset(rel.Assets, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return execFail("no-matching-asset")
	}
	sumAsset, ok := findMeta(rel.Assets, isChecksumsAsset)
	if !ok {
		return execFail("no-checksums-asset")
	}

	dir := filepath.Dir(e.exePath)
	tmpNew := filepath.Join(dir, ".ep-update-new")
	if err := e.downloadToFile(asset.BrowserDownloadURL, tmpNew); err != nil {
		return execFail("download-failed")
	}
	// If we don't end up renaming tmpNew into place, clean it up.
	defer os.Remove(tmpNew)

	sums, err := e.downloadBytes(sumAsset.BrowserDownloadURL)
	if err != nil {
		return execFail("checksums-download-failed")
	}

	if e.verifier.Require {
		// Only ".sig" (raw ed25519) is verifiable by SignatureVerifier. Do not
		// select ".asc"/".minisig" here, or asset ordering could pick a format
		// we cannot verify and fail an otherwise-valid update.
		sigAsset, ok := findMeta(rel.Assets, isEd25519SignatureAsset)
		if !ok {
			return execFail("no-signature-asset")
		}
		sig, err := e.downloadBytes(sigAsset.BrowserDownloadURL)
		if err != nil {
			return execFail("signature-download-failed")
		}
		if err := e.verifier.Verify(sums, decodeMaybeBase64(sig)); err != nil {
			return execFail("signature-verification-failed")
		}
	}

	want, ok := parseChecksums(string(sums))[asset.Name]
	if !ok {
		return execFail("checksum-missing-for-asset")
	}
	if err := verifyFileChecksum(tmpNew, want); err != nil {
		return execFail("checksum-mismatch")
	}

	backup, err := atomicReplace(e.exePath, tmpNew)
	if err != nil {
		return execFail("replace-failed")
	}
	return ExecResult{OK: true, BackupPath: backup}
}

// Rollback restores the previous binary saved during Apply.
func (e *Executor) Rollback(backupPath string) error {
	return restoreBackup(e.exePath, backupPath)
}

func (e *Executor) downloadToFile(url, dest string) error {
	resp, err := e.checker.HTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (e *Executor) downloadBytes(url string) ([]byte, error) {
	resp, err := e.checker.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// ---- asset selection / verification helpers (pure, unit-tested) ----

func osAliases(goos string) []string {
	switch goos {
	case "darwin":
		return []string{"darwin", "macos", "osx"}
	default:
		return []string{goos}
	}
}

func archAliases(goarch string) []string {
	switch goarch {
	case "amd64":
		return []string{"amd64", "x86_64", "x64"}
	case "arm64":
		return []string{"arm64", "aarch64"}
	case "386":
		return []string{"386", "i386", "x86"}
	default:
		return []string{goarch}
	}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func isChecksumsAsset(name string) bool {
	return strings.Contains(name, "checksums") || strings.HasSuffix(name, ".sha256")
}

// isSignatureAsset matches any signature artifact, used to exclude such files
// from binary asset selection.
func isSignatureAsset(name string) bool {
	return strings.HasSuffix(name, ".sig") || strings.HasSuffix(name, ".minisig") || strings.HasSuffix(name, ".asc")
}

// isEd25519SignatureAsset matches only the signature format SignatureVerifier
// can actually check (raw ed25519 over the checksums file, ".sig").
func isEd25519SignatureAsset(name string) bool {
	return strings.HasSuffix(name, ".sig")
}

func isMetaAsset(name string) bool {
	return isChecksumsAsset(name) || isSignatureAsset(name)
}

// matchAsset picks the release binary asset for the running platform.
func matchAsset(assets []ghAsset, goos, goarch string) (ghAsset, bool) {
	oss := osAliases(goos)
	arches := archAliases(goarch)
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if isMetaAsset(name) {
			continue
		}
		if containsAny(name, oss) && containsAny(name, arches) {
			return a, true
		}
	}
	return ghAsset{}, false
}

func findMeta(assets []ghAsset, pred func(name string) bool) (ghAsset, bool) {
	for _, a := range assets {
		if pred(strings.ToLower(a.Name)) {
			return a, true
		}
	}
	return ghAsset{}, false
}

// parseChecksums parses "sha256sum"-style lines ("<hex>  <filename>", an
// optional "*" binary marker is tolerated) into a filename -> hex map.
func parseChecksums(content string) map[string]string {
	out := map[string]string{}
	for line := range strings.SplitSeq(content, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		sum := fields[0]
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		// Use the basename so a path prefix in the checksums file still matches.
		name = filepath.Base(name)
		out[name] = strings.ToLower(sum)
	}
	return out
}

func verifyFileChecksum(path, wantHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, wantHex) {
		return fmt.Errorf("checksum mismatch: got %s want %s", got, wantHex)
	}
	return nil
}

func decodeMaybeBase64(b []byte) []byte {
	trimmed := strings.TrimSpace(string(b))
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		return decoded
	}
	return b
}

// atomicReplace swaps newPath into exePath, preserving the previous binary at
// exePath+".bak". It works while the current binary is running: the running
// executable is moved aside (allowed on Windows and Unix) and the new file is
// renamed into its place. On failure the original is restored.
func atomicReplace(exePath, newPath string) (string, error) {
	backup := exePath + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(exePath, backup); err != nil {
		return "", err
	}
	if err := os.Rename(newPath, exePath); err != nil {
		_ = os.Rename(backup, exePath) // undo
		return "", err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(exePath, 0o755)
	}
	return backup, nil
}

// restoreBackup reverses atomicReplace: the (failed) current binary is moved
// aside and the backup is renamed back into place.
func restoreBackup(exePath, backupPath string) error {
	if backupPath == "" {
		backupPath = exePath + ".bak"
	}
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("no backup to restore: %w", err)
	}
	tmp := exePath + ".rollback-tmp"
	_ = os.Remove(tmp)
	if err := os.Rename(exePath, tmp); err != nil {
		return err
	}
	if err := os.Rename(backupPath, exePath); err != nil {
		_ = os.Rename(tmp, exePath) // undo
		return err
	}
	_ = os.Remove(tmp)
	if runtime.GOOS != "windows" {
		_ = os.Chmod(exePath, 0o755)
	}
	return nil
}
