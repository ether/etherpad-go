package updater

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchAsset(t *testing.T) {
	assets := []ghAsset{
		{Name: "checksums.txt"},
		{Name: "etherpad-go_linux_amd64"},
		{Name: "etherpad-go_windows_amd64.exe"},
		{Name: "etherpad-go_darwin_arm64"},
		{Name: "etherpad-go_linux_amd64.sig"},
	}
	if a, ok := matchAsset(assets, "linux", "amd64"); !ok || a.Name != "etherpad-go_linux_amd64" {
		t.Errorf("linux/amd64 -> %+v ok=%v", a, ok)
	}
	if a, ok := matchAsset(assets, "windows", "amd64"); !ok || a.Name != "etherpad-go_windows_amd64.exe" {
		t.Errorf("windows/amd64 -> %+v ok=%v", a, ok)
	}
	// darwin/arm64 with aarch64 alias name
	aliased := []ghAsset{{Name: "etherpad-go_macos_aarch64"}}
	if a, ok := matchAsset(aliased, "darwin", "arm64"); !ok || a.Name != "etherpad-go_macos_aarch64" {
		t.Errorf("darwin/arm64 alias -> %+v ok=%v", a, ok)
	}
	if _, ok := matchAsset(assets, "freebsd", "riscv64"); ok {
		t.Error("expected no match for unsupported platform")
	}
}

func TestSignatureAssetSelectionPrefersVerifiable(t *testing.T) {
	// .asc is listed first but is not ed25519-verifiable; selection must pick
	// the .sig so a valid update is not failed by asset ordering.
	assets := []ghAsset{
		{Name: "checksums.txt"},
		{Name: "checksums.txt.asc"},
		{Name: "checksums.txt.sig"},
	}
	sig, ok := findMeta(assets, isEd25519SignatureAsset)
	if !ok || sig.Name != "checksums.txt.sig" {
		t.Errorf("expected to select .sig, got %+v ok=%v", sig, ok)
	}
	// .asc / .minisig are still excluded from binary asset selection.
	if !isMetaAsset("checksums.txt.asc") || !isMetaAsset("x.minisig") {
		t.Error("non-ed25519 signature artifacts must still be excluded from binary matching")
	}
}

func TestParseChecksums(t *testing.T) {
	content := "abc123  etherpad-go_linux_amd64\n" +
		"DEF456 *etherpad-go_windows_amd64.exe\n" +
		"\n" +
		"# comment line ignored\n"
	m := parseChecksums(content)
	if m["etherpad-go_linux_amd64"] != "abc123" {
		t.Errorf("linux sum: %q", m["etherpad-go_linux_amd64"])
	}
	if m["etherpad-go_windows_amd64.exe"] != "def456" {
		t.Errorf("windows sum (lowercased, * stripped): %q", m["etherpad-go_windows_amd64.exe"])
	}
}

func TestVerifyFileChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	data := []byte("hello etherpad")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if err := verifyFileChecksum(path, hex.EncodeToString(sum[:])); err != nil {
		t.Errorf("matching checksum should pass: %v", err)
	}
	if err := verifyFileChecksum(path, "deadbeef"); err == nil {
		t.Error("wrong checksum should fail")
	}
}

func TestAtomicReplaceAndRestore(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad")
	if err := os.WriteFile(exe, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBin := filepath.Join(dir, ".ep-update-new")
	if err := os.WriteFile(newBin, []byte("NEW"), 0o755); err != nil {
		t.Fatal(err)
	}

	backup, err := atomicReplace(exe, newBin)
	if err != nil {
		t.Fatalf("atomicReplace: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "NEW" {
		t.Errorf("exe should be NEW, got %q", got)
	}
	if got, _ := os.ReadFile(backup); string(got) != "OLD" {
		t.Errorf("backup should be OLD, got %q", got)
	}

	if err := restoreBackup(exe, backup); err != nil {
		t.Fatalf("restoreBackup: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "OLD" {
		t.Errorf("exe should be restored to OLD, got %q", got)
	}
}

func TestSignatureVerifier(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	msg := []byte("checksums-file-contents")
	sig := ed25519.Sign(priv, msg)
	v := SignatureVerifier{Require: true, PublicKey: base64.StdEncoding.EncodeToString(pub)}

	if err := v.Verify(msg, sig); err != nil {
		t.Errorf("valid signature should pass: %v", err)
	}
	if err := v.Verify([]byte("tampered"), sig); err == nil {
		t.Error("tampered message should fail")
	}
	// not required -> always ok
	if err := (SignatureVerifier{Require: false}).Verify(msg, nil); err != nil {
		t.Errorf("non-required verify should pass: %v", err)
	}
	// required but no key -> error
	if err := (SignatureVerifier{Require: true}).Verify(msg, sig); err == nil {
		t.Error("required verify without key should fail")
	}
}

func TestLockAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update.lock")

	ok, err := acquireLock(path)
	if err != nil || !ok {
		t.Fatalf("first acquire: ok=%v err=%v", ok, err)
	}
	// Same-process re-acquire is blocked while the lock is held.
	if held := lockHeld(path); !held {
		t.Error("lock should be reported held")
	}
	releaseLock(path)
	if held := lockHeld(path); held {
		t.Error("lock should be free after release")
	}
}

func TestDrainerImmediate(t *testing.T) {
	var accepting = true
	d := NewDrainer(0, func(b bool) { accepting = b }, nil)
	if res := d.Start(); res != DrainCompleted {
		t.Errorf("zero-second drain should complete, got %v", res)
	}
	if !accepting {
		t.Error("accepting should be restored to true after drain")
	}
}

func TestDrainerCancel(t *testing.T) {
	d := NewDrainer(60, func(bool) {}, nil)
	go d.Cancel()
	if res := d.Start(); res != DrainCancelled {
		t.Errorf("cancelled drain should report cancelled, got %v", res)
	}
}
