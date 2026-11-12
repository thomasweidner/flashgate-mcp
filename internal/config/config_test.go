package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func documentWithRoot(t *testing.T, root string) string {
	t.Helper()
	value, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return "[filesystem]\nroot = " + string(value) + "\n"
}

func TestDecodeDefaultsAndValueSnapshots(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg, err := Decode([]byte(documentWithRoot(t, root)), Direct)
	if err != nil {
		t.Fatal(err)
	}
	f := cfg.Filesystem()
	s := cfg.Security()
	p := cfg.Server()
	if f.RootPath() != root || f.ReadOnly() || f.AllowCWDRoot() || s.AllowHiddenFiles() ||
		s.AllowUNCPaths() || s.FollowSymlinks() || cfg.Role() != Direct {
		t.Fatal("default filesystem or runtime policy drift")
	}
	if f.MaxFileSize() != defaultFileBytes || f.MaxWriteBytes() != defaultFileBytes ||
		f.MaxCopyBytes() != defaultFileBytes || f.MaxListEntries() != 1000 || f.MaxDeleteEntries() != 1000 {
		t.Fatal("default filesystem limits drift")
	}
	if p.Name() != "flashgate" || p.Version() != "0.1.0-dev" || p.MaxMessageBytes() != defaultWireBytes ||
		p.MaxArgumentBytes() != 12*1024*1024 || p.MaxResponseBytes() != defaultWireBytes {
		t.Fatal("default protocol settings drift")
	}
	logging := cfg.Logging()
	logging.Level = LogDebug
	endpoints := cfg.Endpoints()
	endpoints.Fallback = DirectIfAbsent
	if cfg.Logging().Level != LogOff || cfg.Endpoints().Fallback != NoFallback {
		t.Fatal("changing a snapshot changed the effective configuration")
	}
	if err := DefaultConfig().Validate(); !errors.Is(err, NewError(CategoryMissingRoot, nil)) {
		t.Fatalf("unbound default configuration was usable: %v", err)
	}
}

func TestDecodeRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()
	root := documentWithRoot(t, t.TempDir())
	cases := []struct {
		name, document string
		category       ErrorCategory
	}{
		{"missing root", "[filesystem]\nread_only = true\n", CategoryMissingRoot},
		{"empty root", "[filesystem]\nroot = \"\"\n", CategoryInvalidRoot},
		{"whitespace", "[filesystem]\nroot = \"   \"\n", CategoryInvalidRoot},
		{"relative root", "[filesystem]\nroot = \"other\"\n", CategoryInvalidRoot},
		{"CWD without opt-in", "[filesystem]\nroot = \".\"\n", CategoryInvalidRoot},
		{"root type", "[filesystem]\nroot = 42\n", CategoryInvalidConfig},
		{"duplicate root", root + "root = \"x\"\n", CategoryInvalidConfig},
		{"case alias", root + "read_only = true\nRead_Only = false\n", CategoryInvalidConfig},
		{"case alias in inline table", "filesystem = { ROOT = \"x\" }\n", CategoryInvalidConfig},
		{"case alias in table header", "[Filesystem]\nroot = \"x\"\n", CategoryInvalidConfig},
		{"empty unknown domain", root + "[capabilities]\n", CategoryInvalidConfig},
		{"unknown policy", root + "[security]\nadmin = true\n", CategoryInvalidConfig},
		{"unknown domain", root + "[capabilities]\nwrite = true\n", CategoryInvalidConfig},
		{"duplicate table", root + "[filesystem]\nread_only = true\n", CategoryInvalidConfig},
		{"invalid boolean", root + "read_only = \"true\"\n", CategoryInvalidConfig},
		{"zero limit", root + "max_write_bytes = 0\n", CategoryInvalidConfig},
		{"fractional limit", root + "max_write_bytes = 1.0\n", CategoryInvalidConfig},
		{"negative limit", root + "[protocol]\nmax_message_bytes = -1\n", CategoryInvalidConfig},
		{"integer overflow", root + "[protocol]\nmax_argument_bytes = 9223372036854775808\n", CategoryInvalidConfig},
		{"probe limit overflow", root + "max_file_size = 9223372036854775807\n", CategoryInvalidConfig},
		{"future mode", root + "[runtime]\nmode = \"service\"\n", CategoryUnsupportedRuntime},
		{"unsupported sink", root + "[logging]\nsink = \"journald\"\n", CategoryInvalidConfig},
		{"unsupported level", root + "[logging]\nlevel = \"trace\"\n", CategoryInvalidConfig},
		{"unsupported format", root + "[logging]\nformat = \"yaml\"\n", CategoryInvalidConfig},
		{"record cap", root + "[logging]\nmax_record_bytes = 2\n", CategoryInvalidConfig},
		{"record count", root + "[logging]\nmax_records = 1000001\n", CategoryInvalidConfig},
		{"stdio endpoint", root + "[endpoints]\nwindows_pipe_name = \"pipe\"\n", CategoryInvalidConfig},
		{"invalid UTF8", root + "#\xff\n", CategoryInvalidConfig},
		{"oversized", strings.Repeat("#\n", MaxDocumentBytes), CategoryInvalidConfig},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode([]byte(test.document), Direct)
			assertCategory(t, err, test.category)
		})
	}
}

func TestCanonicalTOMLKeysInCollections(t *testing.T) {
	t.Parallel()
	for _, document := range []string{
		`groups = [[{ item = { leaf = [{ key = "value" }] } }]]`,
		"[[groups]]\nitems = [{ nested.key = true }]\n",
		`items = [{ "quoted_key" = [{ inner = 1 }] }]`,
	} {
		if err := checkTOMLKeys([]byte(document)); err != nil {
			t.Errorf("canonical collection rejected: %v", err)
		}
	}
	for _, document := range []string{
		`groups = [[{ item = { leaf = [{ Key = "value" }] } }]]`,
		"[[groups]]\nitems = [{ nested.Key = true }]\n",
		`items = [{ "quoted_key" = [{ inner = 1, Inner = 2 }] }]`,
	} {
		if err := checkTOMLKeys([]byte(document)); err == nil {
			t.Error("case alias in a collection was accepted")
		}
	}
}

func TestDecodeManagedEndpointPolicy(t *testing.T) {
	t.Parallel()
	document := documentWithRoot(t, t.TempDir()) + "[runtime]\nmode = \"auto\"\n[endpoints]\n" +
		"windows_pipe_name = \"FlashGate.local\"\nlinux_socket_path = \"/run/flashgate.sock\"\n" +
		"fallback = \"direct-if-absent\"\nconnect_timeout_ms = 123\n"
	cfg, err := Decode([]byte(document), Auto)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Endpoints()
	if cfg.Role() != Auto || got.WindowsPipeName != "FlashGate.local" || got.LinuxSocketPath != "/run/flashgate.sock" ||
		got.Fallback != DirectIfAbsent || got.ConnectTimeout != 123*time.Millisecond || got.IPCVersion != 1 {
		t.Fatalf("managed configuration was not preserved: %+v", got)
	}
	_, err = Decode([]byte(document), Direct)
	assertCategory(t, err, CategoryUnsupportedRuntime)
	for _, entry := range []string{
		"windows_pipe_name = \"remote/name\"",
		"linux_socket_path = \"/run/../unsafe.sock\"",
		"linux_socket_path = \"relative.sock\"",
		"fallback = \"direct-if-absent\"",
		"connect_timeout_ms = 3600001",
		"request_timeout_ms = 0",
		"shutdown_timeout_ms = -1",
		"ipc_version = 2",
		"max_frame_bytes = 67108865",
	} {
		_, err = Decode([]byte(documentWithRoot(t, t.TempDir())+"[endpoints]\n"+entry+"\n"), Proxy)
		assertCategory(t, err, CategoryInvalidConfig)
	}
}

func TestLoadUsesExactlyTheTrustedOverride(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "server.toml")
	document := documentWithRoot(t, root) + "read_only = true\n"
	testconfig.Write(t, path, []byte(document))
	t.Setenv("MCP_ROOT", "/legacy")
	t.Setenv("MCP_READ_ONLY", "false")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	cfg, err := Load(Source{Role: Direct, Path: path})
	if err != nil || cfg.Filesystem().RootPath() != root || !cfg.Filesystem().ReadOnly() {
		t.Fatalf("explicit TOML source was not honored: %v", err)
	}
	_, err = Load(Source{Role: Direct, Path: filepath.Join(filepath.Dir(path), "absent.toml")})
	assertCategory(t, err, CategoryMissingConfig)
	for _, path := range []string{"relative.toml", filepath.Dir(path), filepath.Dir(path) + string(filepath.Separator) + ".." + string(filepath.Separator) + "other.toml"} {
		_, err := Load(Source{Role: Direct, Path: path})
		assertCategory(t, err, CategoryUntrustedConfig)
	}
	_, err = Load(Source{Role: "http", Path: path})
	assertCategory(t, err, CategoryUnsupportedRuntime)
	after, err := os.ReadFile(path)
	if err != nil || string(after) != document {
		t.Fatal("loading changed the source file")
	}
}

func TestDefaultConfigSourceMustBeOutsideTheDataRoot(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	homeConfig := filepath.Join(base, ".flashgate")
	dataRoot := filepath.Join(base, "data")
	for _, dir := range []string{homeConfig, dataRoot} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(homeConfig, defaultFileName)
	resolve := func(role Role) (string, error) {
		if role != Direct {
			t.Fatal("incorrect default source role")
		}
		return path, nil
	}
	for _, test := range []struct {
		root string
		want error
	}{
		{base, security.ErrProtectedPath},
		{dataRoot, nil},
	} {
		testconfig.Write(t, path, []byte(documentWithRoot(t, test.root)))
		cfg, err := load(Source{Role: Direct}, resolve)
		if err != nil {
			t.Fatal(err)
		}
		guard, err := security.NewPathGuard(cfg.Filesystem().RootPath())
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.ValidateDataRoot(guard); !errors.Is(got, test.want) {
			t.Errorf("default source isolation = %v, want %v", got, test.want)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal("removing the default configuration fixture failed")
		}
	}
}

func TestSourceIdentityChangeDeniesDataRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), defaultFileName)
	document := []byte(documentWithRoot(t, root))
	testconfig.Write(t, source, document)
	cfg, err := Load(Source{Role: Direct, Path: source})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, filepath.Join(root, "moved.toml")); err != nil {
		t.Fatal(err)
	}
	testconfig.Write(t, source, document)
	guard, err := security.NewPathGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateDataRoot(guard); !errors.Is(err, security.ErrProtectedPath) {
		t.Fatalf("replacement source did not cause a safe denial: %v", err)
	}
}

func TestDataRootRejectsUnprovableSourceIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), defaultFileName)
	document := []byte(documentWithRoot(t, root))
	testconfig.Write(t, source, document)
	guard, err := security.NewPathGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(Source{Role: Direct, Path: source})
	if err != nil {
		t.Fatal(err)
	}
	unbound := cfg
	unbound.sourceInfo = nil
	if err := unbound.ValidateDataRoot(guard); !errors.Is(err, security.ErrProtectedPath) {
		t.Fatalf("unbound source identity did not cause a safe denial: %v", err)
	}
	if err := os.Rename(source, source+".moved"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateDataRoot(guard); !errors.Is(err, security.ErrProtectedPath) {
		t.Fatalf("missing source did not cause a safe denial: %v", err)
	}
}

func TestLoadRejectsLinkedAndOversizedSources(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	path := filepath.Join(base, "source.toml")
	testconfig.Write(t, path, []byte(strings.Repeat("#\n", MaxDocumentBytes)))
	_, err := Load(Source{Role: Direct, Path: path})
	assertCategory(t, err, CategoryInvalidConfig)
	if err := os.Link(path, filepath.Join(base, "hardlink.toml")); err != nil {
		t.Fatal(err)
	}
	_, err = Load(Source{Role: Direct, Path: path})
	assertCategory(t, err, CategoryUntrustedConfig)
}

func TestLoadRejectsSymlinkParentsAndLeaves(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "trusted")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "config.toml")
	testconfig.Write(t, file, []byte(documentWithRoot(t, dir)))
	for _, tc := range []struct{ target, link, child string }{
		{file, filepath.Join(base, "leaf.toml"), ""},
		{dir, filepath.Join(base, "parent"), "config.toml"},
	} {
		if err := os.Symlink(tc.target, tc.link); err != nil {
			t.Skip("native symlink creation is unavailable")
		}
		_, err := Load(Source{Role: Direct, Path: filepath.Join(tc.link, tc.child)})
		assertCategory(t, err, CategoryUntrustedConfig)
	}
}

func TestDefaultPathUsesOSIdentity(t *testing.T) {
	fake := t.TempDir()
	t.Setenv("HOME", fake)
	t.Setenv("USERPROFILE", fake)
	t.Setenv("ProgramData", fake)
	for _, role := range []Role{Direct, Proxy, Auto, UserHost, SystemService} {
		path, err := defaultPath(role)
		if err != nil || !filepath.IsAbs(path) || filepath.Base(path) != defaultFileName || strings.HasPrefix(path, fake) {
			t.Fatalf("role %s used an untrusted default: %v", role, err)
		}
	}
}

func TestConfigErrorSupportsErrorsIsAndAs(t *testing.T) {
	t.Parallel()
	cause := errors.New("private path")
	err := NewError(CategoryInvalidRoot, cause)
	var categorized *ConfigError
	if !errors.Is(err, cause) || !errors.Is(err, NewError(CategoryInvalidRoot, nil)) ||
		!errors.As(err, &categorized) || err.Error() != string(CategoryInvalidRoot) {
		t.Fatalf("configuration error contract drifted: %v", err)
	}
}

func assertCategory(t *testing.T, err error, expected ErrorCategory) {
	t.Helper()
	if got, ok := CategoryOf(err); !ok || got != expected {
		t.Fatalf("wanted %s, got %v", expected, err)
	}
}
