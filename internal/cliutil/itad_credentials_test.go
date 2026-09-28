package cliutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestITADCredentialRoundTrip proves the IsThereAnyDeal key is stored in
// credentials.toml and read back, and that it survives as a sibling of the
// primary api_key rather than replacing it.
func TestITADCredentialRoundTrip(t *testing.T) {
	restore, err := SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()

	// Seed a primary credential so we can prove the ITAD write preserves it.
	if err := SaveCredentials(&Credentials{RawgApiKey: "rawg-secret"}); err != nil {
		t.Fatalf("seed rawg credential: %v", err)
	}
	if err := SaveITADCredential("itad-secret"); err != nil {
		t.Fatalf("SaveITADCredential: %v", err)
	}

	key, ok, err := LoadITADCredential()
	if err != nil || !ok || key != "itad-secret" {
		t.Fatalf("LoadITADCredential = (%q, %v, %v), want itad-secret", key, ok, err)
	}

	creds, ok, err := LoadCredentials()
	if err != nil || !ok {
		t.Fatalf("LoadCredentials: ok=%v err=%v", ok, err)
	}
	if creds.RawgApiKey != "rawg-secret" {
		t.Fatalf("sibling api_key = %q, want rawg-secret (must be preserved)", creds.RawgApiKey)
	}

	path, err := CredentialsFilePath()
	if err != nil {
		t.Fatalf("CredentialsFilePath: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credentials perm = %o, want 600", perm)
	}

	// Clearing the ITAD key leaves the primary credential intact.
	if err := ClearITADCredential(); err != nil {
		t.Fatalf("ClearITADCredential: %v", err)
	}
	if _, ok, err := LoadITADCredential(); err != nil || ok {
		t.Fatalf("ITAD credential still present after clear: ok=%v err=%v", ok, err)
	}
	if creds, ok, err := LoadCredentials(); err != nil || !ok || creds.RawgApiKey != "rawg-secret" {
		t.Fatalf("primary credential lost after ITAD clear: %+v ok=%v err=%v", creds, ok, err)
	}
}

// TestClearITADCredentialOnlyFieldRemovesFile proves clearing the only stored
// field removes the file instead of leaving an empty credentials.toml behind.
func TestClearITADCredentialOnlyFieldRemovesFile(t *testing.T) {
	restore, err := SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()

	if err := SaveITADCredential("only-itad"); err != nil {
		t.Fatalf("SaveITADCredential: %v", err)
	}
	path, err := CredentialsFilePath()
	if err != nil {
		t.Fatalf("CredentialsFilePath: %v", err)
	}
	if err := ClearITADCredential(); err != nil {
		t.Fatalf("ClearITADCredential: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("credentials file should be removed, stat err = %v", err)
	}
}

// TestSaveITADRefusesRefusedCredentialsFile proves an unreadable (over-permissive)
// credentials file is never overwritten: doing so would silently discard the
// credential it holds instead of asking the user to fix its permissions.
func TestSaveITADRefusesRefusedCredentialsFile(t *testing.T) {
	restore, err := SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()

	path, err := CredentialsFilePath()
	if err != nil {
		t.Fatalf("CredentialsFilePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("api_key = \"rawg-secret\"\n"), 0o644); err != nil {
		t.Fatalf("seed world-readable credentials: %v", err)
	}

	if err := SaveITADCredential("itad-secret"); err == nil {
		t.Fatal("SaveITADCredential must refuse a world-readable credentials file, not overwrite it")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	if !strings.Contains(string(data), "rawg-secret") {
		t.Fatalf("refused file was overwritten and its credential lost: %q", string(data))
	}
}
