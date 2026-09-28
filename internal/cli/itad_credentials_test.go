package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/config"
	"github.com/spf13/cobra"
)

// TestResolveITADKeyPrecedence proves the stored credential is used when no env
// override is set, and that an explicit env var still wins.
func TestResolveITADKeyPrecedence(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	// No key anywhere -> empty, no error.
	key, err := resolveITADKey(&rootFlags{})
	if err != nil || key != "" {
		t.Fatalf("resolveITADKey with no key = (%q, %v), want empty", key, err)
	}

	// Stored credential is used.
	if err := cliutil.SaveITADCredential("stored-itad"); err != nil {
		t.Fatalf("SaveITADCredential: %v", err)
	}
	key, err = resolveITADKey(&rootFlags{})
	if err != nil || key != "stored-itad" {
		t.Fatalf("resolveITADKey stored = (%q, %v), want stored-itad", key, err)
	}

	// Env override wins over the stored credential.
	t.Setenv("ITAD_API_KEY", "env-itad")
	key, err = resolveITADKey(&rootFlags{})
	if err != nil || key != "env-itad" {
		t.Fatalf("resolveITADKey env = (%q, %v), want env-itad", key, err)
	}
}

// TestNewITADClientUsesStoredCredential proves prices/price-history authenticate
// from the stored credential with no ITAD_API_KEY in the environment.
func TestNewITADClientUsesStoredCredential(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	if err := cliutil.SaveITADCredential("stored-itad"); err != nil {
		t.Fatalf("SaveITADCredential: %v", err)
	}
	if _, err := newITADClient(&rootFlags{}, "US"); err != nil {
		t.Fatalf("newITADClient with stored credential: %v", err)
	}
}

// TestNewITADClientMissingKeyGuidance points the user at both the env var and
// the one-time auth command when no key is configured.
func TestNewITADClientMissingKeyGuidance(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	_, err = newITADClient(&rootFlags{}, "US")
	if err == nil {
		t.Fatal("expected an auth error with no ITAD key configured")
	}
	if !strings.Contains(err.Error(), "auth set-token --provider itad") {
		t.Fatalf("error should name the stored-credential path, got: %v", err)
	}
}

// TestRawgSavePreservesStoredITADKey guards the shared-file contract: saving
// the primary RAWG key must not drop a stored IsThereAnyDeal key.
func TestRawgSavePreservesStoredITADKey(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	if err := cliutil.SaveITADCredential("itad-secret"); err != nil {
		t.Fatalf("SaveITADCredential: %v", err)
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if err := cfg.SaveCredential("rawg-secret"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}

	creds, ok, err := cliutil.LoadCredentials()
	if err != nil || !ok {
		t.Fatalf("LoadCredentials: ok=%v err=%v", ok, err)
	}
	if creds.RawgApiKey != "rawg-secret" {
		t.Fatalf("RAWG key = %q, want rawg-secret", creds.RawgApiKey)
	}
	if creds.ITADApiKey != "itad-secret" {
		t.Fatalf("ITAD key = %q, want itad-secret (RAWG save must not drop it)", creds.ITADApiKey)
	}
}

// TestRawgLogoutPreservesStoredITADKey guards that clearing the RAWG credential
// rewrites the shared file instead of deleting a sibling ITAD key.
func TestRawgLogoutPreservesStoredITADKey(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	if err := cliutil.SaveCredentials(&cliutil.Credentials{RawgApiKey: "rawg-secret", ITADApiKey: "itad-secret"}); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}

	creds, ok, err := cliutil.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if !ok {
		t.Fatal("credentials file should remain for the stored ITAD key")
	}
	if creds.RawgApiKey != "" {
		t.Fatalf("RAWG key was not cleared: %q", creds.RawgApiKey)
	}
	if creds.ITADApiKey != "itad-secret" {
		t.Fatalf("ITAD key = %q, want itad-secret (RAWG logout must not drop it)", creds.ITADApiKey)
	}
}

// TestCredentialsFileITADKeyBeatsConfigFileKey proves a rotated key stored via
// auth set-token --provider itad (credentials file) is used even when the config
// file still carries an older itad_api_key.
func TestCredentialsFileITADKeyBeatsConfigFileKey(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	credsPath := filepath.Join(dir, "data", "credentials.toml")
	if err := os.MkdirAll(filepath.Dir(credsPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(credsPath, []byte("itad_api_key = \"itad-new\"\n"), 0o600); err != nil {
		t.Fatalf("seed credentials: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("base_url = \"https://api.rawg.io/api\"\nitad_api_key = \"itad-old\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.ITADApiKey != "itad-new" {
		t.Fatalf("ITAD key = %q, want itad-new (credentials file must beat a stale config value)", cfg.ITADApiKey)
	}
}

// TestResolveITADKeyEnvBypassesMalformedConfig proves a valid env key still
// works when the config file cannot be parsed.
func TestResolveITADKeyEnvBypassesMalformedConfig(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "env-itad")

	bad := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(bad, []byte("this is not = valid = toml\n"), 0o600); err != nil {
		t.Fatalf("seed malformed config: %v", err)
	}
	key, err := resolveITADKey(&rootFlags{configPath: bad})
	if err != nil {
		t.Fatalf("resolveITADKey must use the env key despite a malformed config: %v", err)
	}
	if key != "env-itad" {
		t.Fatalf("key = %q, want env-itad", key)
	}
}

// TestSaveITADTokenReportsSelectedHome proves the reported credentials_path is
// the file actually written, not the default home's path.
func TestSaveITADTokenReportsSelectedHome(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("base_url = \"https://api.rawg.io/api\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := saveITADToken(cmd, &rootFlags{configPath: cfgPath, asJSON: true}, "itad-x"); err != nil {
		t.Fatalf("saveITADToken: %v", err)
	}
	want := filepath.Join(dir, "data", "credentials.toml")
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("reported credentials_path missing %q: %s", want, buf.String())
	}
}

// TestExplicitConfigDoesNotLeakDefaultITADKey proves an explicit config with no
// sibling credentials never adopts the default home's ITAD key, so a later RAWG
// save cannot copy it into the selected home.
func TestExplicitConfigDoesNotLeakDefaultITADKey(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	if err := cliutil.SaveCredentials(&cliutil.Credentials{RawgApiKey: "rawg-default", ITADApiKey: "itad-default"}); err != nil {
		t.Fatalf("seed default credentials: %v", err)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("base_url = \"https://api.rawg.io/api\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.ITADApiKey != "" {
		t.Fatalf("explicit config adopted the default home's ITAD key: %q", cfg.ITADApiKey)
	}
	if err := cfg.SaveCredential("rawg-selected"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	creds, ok, err := cliutil.LoadCredentialsForConfig(cfgPath)
	if err != nil || !ok {
		t.Fatalf("selected credentials: ok=%v err=%v", ok, err)
	}
	if creds.RawgApiKey != "rawg-selected" {
		t.Fatalf("selected RAWG key = %q, want rawg-selected", creds.RawgApiKey)
	}
	if creds.ITADApiKey != "" {
		t.Fatalf("default home's ITAD key leaked into the selected home: %q", creds.ITADApiKey)
	}
}

// TestGameGoatConfigEnvSelectsSiblingCredentials proves a GAME_GOAT_CONFIG home
// resolves its own credentials file, so ITAD operations (clear/save) target that
// home and never the default one.
func TestGameGoatConfigEnvSelectsSiblingCredentials(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	if err := cliutil.SaveCredentials(&cliutil.Credentials{RawgApiKey: "rawg-default", ITADApiKey: "itad-default"}); err != nil {
		t.Fatalf("seed default credentials: %v", err)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	credsPath := filepath.Join(dir, "data", "credentials.toml")
	if err := os.MkdirAll(filepath.Dir(credsPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(credsPath, []byte("api_key = \"rawg-selected\"\nitad_api_key = \"itad-selected\"\n"), 0o600); err != nil {
		t.Fatalf("seed selected credentials: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("base_url = \"https://api.rawg.io/api\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	t.Setenv("GAME_GOAT_CONFIG", cfgPath)

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	got, err := cfg.CredentialsFilePath()
	if err != nil {
		t.Fatalf("CredentialsFilePath: %v", err)
	}
	if got != credsPath {
		t.Fatalf("credentials path = %q, want %q", got, credsPath)
	}
	if cfg.ITADApiKey != "itad-selected" {
		t.Fatalf("ITAD key = %q, want itad-selected", cfg.ITADApiKey)
	}

	if err := cliutil.ClearITADCredentialAt(got); err != nil {
		t.Fatalf("ClearITADCredentialAt: %v", err)
	}
	sel, ok, err := cliutil.LoadCredentialsForConfig(cfgPath)
	if err != nil || !ok {
		t.Fatalf("selected credentials: ok=%v err=%v", ok, err)
	}
	if sel.ITADApiKey != "" {
		t.Fatalf("selected home's ITAD key survived clear: %q", sel.ITADApiKey)
	}
	if sel.RawgApiKey != "rawg-selected" {
		t.Fatalf("selected home's RAWG key was lost: %q", sel.RawgApiKey)
	}
	def, _, err := cliutil.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if def.ITADApiKey != "itad-default" {
		t.Fatalf("default home's ITAD key was touched: %q", def.ITADApiKey)
	}
}

// TestLogoutWithExplicitConfigTargetsSiblingCredentials proves logout reads and
// writes the same selected home: with --config X, the remaining ITAD key is
// rewritten into X/data/credentials.toml and the RAWG key there is cleared,
// never the default home's file.
func TestLogoutWithExplicitConfigTargetsSiblingCredentials(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	credsPath := filepath.Join(dir, "data", "credentials.toml")
	if err := os.MkdirAll(filepath.Dir(credsPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(credsPath, []byte("api_key = \"rawg-secret\"\nitad_api_key = \"itad-secret\"\n"), 0o600); err != nil {
		t.Fatalf("seed sibling credentials: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("base_url = \"https://api.rawg.io/api\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}

	creds, ok, err := cliutil.LoadCredentialsForConfig(cfgPath)
	if err != nil || !ok {
		t.Fatalf("sibling credentials: ok=%v err=%v", ok, err)
	}
	if creds.RawgApiKey != "" {
		t.Fatalf("selected home's RAWG key survived logout: %q", creds.RawgApiKey)
	}
	if creds.ITADApiKey != "itad-secret" {
		t.Fatalf("ITAD key = %q, want itad-secret", creds.ITADApiKey)
	}

	defPath, err := cliutil.CredentialsFilePath()
	if err != nil {
		t.Fatalf("CredentialsFilePath: %v", err)
	}
	if _, err := os.Stat(defPath); !os.IsNotExist(err) {
		t.Fatalf("explicit-config logout wrote the default home's credentials file: %v", err)
	}
}

// TestLogoutAllWithExplicitConfigRemovesSiblingCredentials proves the
// no-ITAD removal path also targets the selected home, leaving no RAWG key on
// disk there and not touching the default home.
func TestLogoutAllWithExplicitConfigRemovesSiblingCredentials(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	defer restore()
	t.Setenv("ITAD_API_KEY", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	credsPath := filepath.Join(dir, "data", "credentials.toml")
	if err := os.MkdirAll(filepath.Dir(credsPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(credsPath, []byte("api_key = \"rawg-secret\"\n"), 0o600); err != nil {
		t.Fatalf("seed sibling credentials: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("base_url = \"https://api.rawg.io/api\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}
	if _, err := os.Stat(credsPath); !os.IsNotExist(err) {
		t.Fatalf("selected home's credentials file was not removed: %v", err)
	}
	defPath, err := cliutil.CredentialsFilePath()
	if err != nil {
		t.Fatalf("CredentialsFilePath: %v", err)
	}
	if _, err := os.Stat(defPath); !os.IsNotExist(err) {
		t.Fatalf("explicit-config logout created the default home's credentials file: %v", err)
	}
}
