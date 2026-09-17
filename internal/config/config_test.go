package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.Filesystem().RootPath() != defaultRootPath {
		t.Fatalf("expected root path %q, got %q", defaultRootPath, cfg.Filesystem().RootPath())
	}

	if !cfg.Filesystem().ReadOnly() {
		t.Fatal("expected safe read-only mode by default")
	}

	if cfg.Profile() != ProfileSafeRead {
		t.Fatalf("expected default profile %q, got %q", ProfileSafeRead, cfg.Profile())
	}

	if got := cfg.RiskPolicy(); len(got) != 1 || got[0] != RiskStandard {
		t.Fatalf("expected default standard risk policy, got %v", got)
	}

	if cfg.Filesystem().AllowCWDRoot() {
		t.Fatal("expected CWD root development option to be disabled by default")
	}

	if cfg.Filesystem().MaxFileSize() != defaultMaxFileSize {
		t.Fatalf("expected max file size %d, got %d", defaultMaxFileSize, cfg.Filesystem().MaxFileSize())
	}

	if cfg.Filesystem().MaxWriteBytes() != defaultMaxWriteBytes {
		t.Fatalf("expected max write bytes %d, got %d", defaultMaxWriteBytes, cfg.Filesystem().MaxWriteBytes())
	}

	if cfg.Filesystem().MaxListEntries() != defaultMaxListEntries {
		t.Fatalf("expected max list entries %d, got %d", defaultMaxListEntries, cfg.Filesystem().MaxListEntries())
	}

	if cfg.Filesystem().MaxCopyBytes() != defaultMaxCopyBytes {
		t.Fatalf("expected max copy bytes %d, got %d", defaultMaxCopyBytes, cfg.Filesystem().MaxCopyBytes())
	}

	if cfg.Filesystem().MaxDeleteEntries() != defaultMaxDeleteEntries {
		t.Fatalf("expected max delete entries %d, got %d", defaultMaxDeleteEntries, cfg.Filesystem().MaxDeleteEntries())
	}

	if cfg.Security().AllowHiddenFiles() {
		t.Fatal("expected hidden files to be denied by default")
	}

	if cfg.Security().AllowUNCPaths() {
		t.Fatal("expected UNC paths to be denied by default")
	}

	if cfg.Security().FollowSymlinks() {
		t.Fatal("expected symlink following to be disabled by default")
	}

	if cfg.Server().Name() != defaultServerName {
		t.Fatalf("expected server name %q, got %q", defaultServerName, cfg.Server().Name())
	}

	if cfg.Server().Version() != defaultVersion {
		t.Fatalf("expected version %q, got %q", defaultVersion, cfg.Server().Version())
	}

	if cfg.Server().Debug() {
		t.Fatal("expected debug mode to be disabled by default")
	}

	if cfg.Server().MaxMessageBytes() != defaultMaxMessageBytes {
		t.Fatalf("expected max message bytes %d, got %d", defaultMaxMessageBytes, cfg.Server().MaxMessageBytes())
	}

	if cfg.Server().MaxArgumentBytes() != defaultMaxArgumentBytes {
		t.Fatalf("expected max argument bytes %d, got %d", defaultMaxArgumentBytes, cfg.Server().MaxArgumentBytes())
	}

	if cfg.Server().MaxResponseBytes() != defaultMaxResponseBytes {
		t.Fatalf("expected max response bytes %d, got %d", defaultMaxResponseBytes, cfg.Server().MaxResponseBytes())
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv(envRootPath, root)
	t.Setenv(envReadOnly, "true")
	t.Setenv(envMaxFileSize, "1048576")
	t.Setenv(envMaxWriteBytes, "2048")
	t.Setenv(envMaxListEntries, "25")
	t.Setenv(envMaxCopyBytes, "4096")
	t.Setenv(envMaxDeleteEntries, "30")
	t.Setenv(envAllowHiddenFiles, "true")
	t.Setenv(envAllowUNCPaths, "true")
	t.Setenv(envFollowSymlinks, "true")
	t.Setenv(envServerDebug, "true")
	t.Setenv(envMaxMessageBytes, "8192")
	t.Setenv(envMaxArgumentBytes, "4096")
	t.Setenv(envMaxResponseBytes, "16384")

	cfg, err := LoadFromEnvironment()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Filesystem().RootPath() != root {
		t.Fatalf("unexpected root path: %q", cfg.Filesystem().RootPath())
	}

	if !cfg.Filesystem().ReadOnly() {
		t.Fatal("expected read-only mode to be enabled")
	}

	if cfg.Filesystem().MaxFileSize() != 1048576 {
		t.Fatalf("expected max file size 1048576, got %d", cfg.Filesystem().MaxFileSize())
	}

	if cfg.Filesystem().MaxWriteBytes() != 2048 {
		t.Fatalf("expected max write bytes 2048, got %d", cfg.Filesystem().MaxWriteBytes())
	}

	if cfg.Filesystem().MaxListEntries() != 25 {
		t.Fatalf("expected max list entries 25, got %d", cfg.Filesystem().MaxListEntries())
	}

	if cfg.Filesystem().MaxCopyBytes() != 4096 {
		t.Fatalf("expected max copy bytes 4096, got %d", cfg.Filesystem().MaxCopyBytes())
	}

	if cfg.Filesystem().MaxDeleteEntries() != 30 {
		t.Fatalf("expected max delete entries 30, got %d", cfg.Filesystem().MaxDeleteEntries())
	}

	if !cfg.Security().AllowHiddenFiles() {
		t.Fatal("expected hidden files to be allowed")
	}

	if !cfg.Security().AllowUNCPaths() {
		t.Fatal("expected UNC paths to be allowed")
	}

	if !cfg.Security().FollowSymlinks() {
		t.Fatal("expected symlink following to be enabled")
	}

	if !cfg.Server().Debug() {
		t.Fatal("expected debug mode to be enabled")
	}

	if cfg.Server().MaxMessageBytes() != 8192 {
		t.Fatalf("expected max message bytes 8192, got %d", cfg.Server().MaxMessageBytes())
	}

	if cfg.Server().MaxArgumentBytes() != 4096 {
		t.Fatalf("expected max argument bytes 4096, got %d", cfg.Server().MaxArgumentBytes())
	}

	if cfg.Server().MaxResponseBytes() != 16384 {
		t.Fatalf("expected max response bytes 16384, got %d", cfg.Server().MaxResponseBytes())
	}
}

func TestLoadFromEnvironmentDefaultsToSafeReadProfile(t *testing.T) {
	setValidRoot(t)

	cfg, err := LoadFromEnvironment()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Profile() != ProfileSafeRead || !cfg.Filesystem().ReadOnly() {
		t.Fatalf("expected safe read-only default, got profile=%q readOnly=%t", cfg.Profile(), cfg.Filesystem().ReadOnly())
	}
}

func TestLoadFromEnvironmentActivatesFilesystemWriteProfileExplicitly(t *testing.T) {
	setValidRoot(t)
	t.Setenv(envProfile, string(ProfileFilesystemWrite))
	t.Setenv(envRiskPolicy, "high-risk,destructive")

	cfg, err := LoadFromEnvironment()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Profile() != ProfileFilesystemWrite || cfg.Filesystem().ReadOnly() {
		t.Fatalf("expected explicit write profile, got profile=%q readOnly=%t", cfg.Profile(), cfg.Filesystem().ReadOnly())
	}
	wantRisk := []RiskClass{RiskHigh, RiskDestructive}
	if got := cfg.RiskPolicy(); len(got) != len(wantRisk) || got[0] != wantRisk[0] || got[1] != wantRisk[1] {
		t.Fatalf("expected risk policy %v, got %v", wantRisk, got)
	}
}

func TestLoadFromEnvironmentSupportsLegacyReadOnlyFalseAsExplicitWriteActivation(t *testing.T) {
	setValidRoot(t)
	t.Setenv(envReadOnly, "false")

	cfg, err := LoadFromEnvironment()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Profile() != ProfileFilesystemWrite || cfg.Filesystem().ReadOnly() {
		t.Fatalf("expected legacy false to activate write profile, got profile=%q readOnly=%t", cfg.Profile(), cfg.Filesystem().ReadOnly())
	}
}

func TestLoadFromEnvironmentRejectsProfileAndLegacyConflict(t *testing.T) {
	setValidRoot(t)
	t.Setenv(envProfile, string(ProfileSafeRead))
	t.Setenv(envReadOnly, "false")

	_, err := LoadFromEnvironment()
	assertCategory(t, err, CategoryInvalidProfile)
}

func TestLoadFromEnvironmentRejectsInvalidProfileAndRiskPolicy(t *testing.T) {
	t.Run("profile", func(t *testing.T) {
		setValidRoot(t)
		t.Setenv(envProfile, "admin")
		_, err := LoadFromEnvironment()
		assertCategory(t, err, CategoryInvalidProfile)
	})

	t.Run("risk policy", func(t *testing.T) {
		setValidRoot(t)
		t.Setenv(envRiskPolicy, "standard,high-risk")
		_, err := LoadFromEnvironment()
		assertCategory(t, err, CategoryInvalidRiskPolicy)
	})
}

func TestLoadFromEnvironmentParsesFalseSecurityFlags(t *testing.T) {
	setValidRoot(t)
	t.Setenv(envAllowHiddenFiles, "false")
	t.Setenv(envAllowUNCPaths, "false")
	t.Setenv(envFollowSymlinks, "false")

	cfg, err := LoadFromEnvironment()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Security().AllowHiddenFiles() {
		t.Fatal("expected hidden files to be denied")
	}

	if cfg.Security().AllowUNCPaths() {
		t.Fatal("expected UNC paths to be denied")
	}

	if cfg.Security().FollowSymlinks() {
		t.Fatal("expected symlink following to be denied")
	}
}

func TestLoadFromEnvironmentRejectsInvalidReadOnly(t *testing.T) {
	setValidRoot(t)
	t.Setenv(envReadOnly, "not-a-bool")

	_, err := LoadFromEnvironment()
	assertCategory(t, err, CategoryInvalidProfile)
}

func TestLoadFromEnvironmentRejectsInvalidMaxFileSize(t *testing.T) {
	setValidRoot(t)
	t.Setenv(envMaxFileSize, "not-a-number")

	_, err := LoadFromEnvironment()
	if err == nil {
		t.Fatal("expected error for invalid max file size value")
	}
}

func TestLoadFromEnvironmentRejectsInvalidLimits(t *testing.T) {
	testCases := map[string]string{
		envMaxFileSize:      "0",
		envMaxWriteBytes:    "-1",
		envMaxListEntries:   "not-a-number",
		envMaxCopyBytes:     "0",
		envMaxDeleteEntries: "-1",
		envMaxMessageBytes:  "9223372036854775808",
		envMaxArgumentBytes: "0",
		envMaxResponseBytes: "-1",
	}

	for name, value := range testCases {
		name := name
		value := value

		t.Run(name, func(t *testing.T) {
			setValidRoot(t)
			t.Setenv(name, value)

			_, err := LoadFromEnvironment()
			if err == nil {
				t.Fatalf("expected error for invalid %s value", name)
			}
		})
	}
}

func TestLoadFromEnvironmentRejectsInvalidAllowHiddenFiles(t *testing.T) {
	setValidRoot(t)
	t.Setenv(envAllowHiddenFiles, "not-a-bool")

	_, err := LoadFromEnvironment()
	if err == nil {
		t.Fatal("expected error for invalid hidden files value")
	}
}

func TestLoadFromEnvironmentRejectsInvalidAllowUNCPaths(t *testing.T) {
	setValidRoot(t)
	t.Setenv(envAllowUNCPaths, "not-a-bool")

	_, err := LoadFromEnvironment()
	if err == nil {
		t.Fatal("expected error for invalid UNC paths value")
	}
}

func TestLoadFromEnvironmentRejectsInvalidFollowSymlinks(t *testing.T) {
	setValidRoot(t)
	t.Setenv(envFollowSymlinks, "not-a-bool")

	_, err := LoadFromEnvironment()
	if err == nil {
		t.Fatal("expected error for invalid symlink value")
	}
}

func TestLoadFromEnvironmentRejectsInvalidDebug(t *testing.T) {
	setValidRoot(t)
	t.Setenv(envServerDebug, "not-a-bool")

	_, err := LoadFromEnvironment()
	if err == nil {
		t.Fatal("expected error for invalid debug value")
	}
}

func TestValidateRejectsDefaultConfigWithoutRoot(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	assertCategory(t, cfg.Validate(), CategoryInvalidRoot)
}

func TestValidateRejectsEmptyRootPath(t *testing.T) {
	t.Parallel()

	cfg := Config{
		filesystem: FilesystemConfig{
			rootPath:    "",
			readOnly:    false,
			maxFileSize: defaultMaxFileSize,
		},
		security: SecurityConfig{},
		server:   ServerConfig{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty root path")
	}
}

func TestValidateRejectsZeroMaxFileSize(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.filesystem.rootPath = t.TempDir()
	cfg.filesystem.maxFileSize = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for zero max file size")
	}
}

func TestValidateRejectsNegativeMaxFileSize(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.filesystem.rootPath = t.TempDir()
	cfg.filesystem.maxFileSize = -1

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative max file size")
	}
}

func TestLoadFromEnvironmentDistinguishesMissingAndEmptyRoot(t *testing.T) {
	previous, existed := os.LookupEnv(envRootPath)
	if err := os.Unsetenv(envRootPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(envRootPath, previous)
		} else {
			_ = os.Unsetenv(envRootPath)
		}
	})

	_, err := LoadFromEnvironment()
	assertCategory(t, err, CategoryMissingRoot)

	t.Setenv(envRootPath, "")
	_, err = LoadFromEnvironment()
	assertCategory(t, err, CategoryInvalidRoot)
}

func TestLoadFromEnvironmentRejectsWhitespaceRoot(t *testing.T) {
	for _, root := range []string{" \t ", "\r\n"} {
		t.Run(root, func(t *testing.T) {
			t.Setenv(envRootPath, root)
			_, err := LoadFromEnvironment()
			assertCategory(t, err, CategoryInvalidRoot)
		})
	}
}

func TestLoadFromEnvironmentAcceptsAbsoluteRootForms(t *testing.T) {
	base := t.TempDir()
	tests := map[string]string{
		"absolute":           base,
		"embedded spaces":    filepath.Join(base, "root with spaces"),
		"trailing separator": base + string(filepath.Separator),
	}

	for name, root := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(envRootPath, root)
			cfg, err := LoadFromEnvironment()
			if err != nil {
				t.Fatalf("expected root to be accepted, got %v", err)
			}
			if cfg.Filesystem().RootPath() != root {
				t.Fatalf("expected root %q to remain unchanged, got %q", root, cfg.Filesystem().RootPath())
			}
		})
	}
}

func TestLoadFromEnvironmentRejectsRelativeRoots(t *testing.T) {
	tests := []string{
		"subdir",
		filepath.Join("subdir", "child"),
		"..",
		filepath.Join("subdir", ".."),
		"./",
		".\\",
		"C:relative",
	}

	for _, root := range tests {
		t.Run(root, func(t *testing.T) {
			t.Setenv(envRootPath, root)
			_, err := LoadFromEnvironment()
			assertCategory(t, err, CategoryInvalidRoot)
		})
	}
}

func TestLoadFromEnvironmentRequiresOptInForDotRoot(t *testing.T) {
	t.Setenv(envRootPath, ".")

	_, err := LoadFromEnvironment()
	assertCategory(t, err, CategoryInvalidRoot)

	t.Setenv(envAllowCWDRoot, "true")
	cfg, err := LoadFromEnvironment()
	if err != nil {
		t.Fatalf("expected explicit CWD root opt-in to succeed, got %v", err)
	}
	if !cfg.Filesystem().AllowCWDRoot() {
		t.Fatal("expected CWD root opt-in to be enabled")
	}
}

func TestLoadFromEnvironmentDevelopmentOptionDoesNotCreateRoot(t *testing.T) {
	previous, existed := os.LookupEnv(envRootPath)
	if err := os.Unsetenv(envRootPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(envRootPath, previous)
		} else {
			_ = os.Unsetenv(envRootPath)
		}
	})
	t.Setenv(envAllowCWDRoot, "true")

	_, err := LoadFromEnvironment()
	assertCategory(t, err, CategoryMissingRoot)
}

func TestLoadFromEnvironmentParsesStrictDevelopmentOption(t *testing.T) {
	t.Setenv(envRootPath, ".")

	for _, value := range []string{"TRUE", "False", "1", "yes", "t", "", " ", "\t", "\r\n"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(envAllowCWDRoot, value)
			_, err := LoadFromEnvironment()
			assertCategory(t, err, CategoryInvalidDevelopmentOption)
		})
	}

	t.Run("false", func(t *testing.T) {
		t.Setenv(envAllowCWDRoot, "false")
		_, err := LoadFromEnvironment()
		assertCategory(t, err, CategoryInvalidRoot)
	})
}

func TestConfigErrorSupportsErrorsIsAndAs(t *testing.T) {
	cause := errors.New("internal cause")
	err := NewError(CategoryRootNotAllowed, cause)

	if !errors.Is(err, &ConfigError{Category: CategoryRootNotAllowed}) {
		t.Fatal("expected errors.Is category match")
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected wrapped cause match")
	}
	assertCategory(t, err, CategoryRootNotAllowed)
	if err.Error() != string(CategoryRootNotAllowed) {
		t.Fatalf("expected safe category error text, got %q", err.Error())
	}
}

func setValidRoot(t *testing.T) {
	t.Helper()
	t.Setenv(envRootPath, t.TempDir())
}

func assertCategory(t *testing.T, err error, expected ErrorCategory) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected category %s, got nil", expected)
	}
	category, ok := CategoryOf(err)
	if !ok || category != expected {
		t.Fatalf("expected category %s, got %s (%v)", expected, category, err)
	}
}
