package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func documentWithRoot(t *testing.T, root string) string {
	t.Helper()
	value, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return "[files]\ndefault_id = 'work'\nwork.path = " + string(value) + "\n"
}

func TestDecodeDefaultsAndValueSnapshots(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg, err := Decode([]byte(documentWithRoot(t, root)), Direct)
	if err != nil {
		t.Fatal(err)
	}
	f := cfg.Files()
	s := cfg.Security()
	p := cfg.Server()
	roots := f.Roots()
	if len(roots) != 1 || roots[0].ID != "work" || roots[0].Path != filepath.Clean(root) ||
		!roots[0].Access.Equal(cfg.Profile().Policy().Grants()) || roots[0].Limits != DefaultConfig().Files().Limits() || f.DefaultID() != "work" ||
		cfg.Profile().ID() != ProfileRead || s.AllowHiddenFiles() ||
		s.AllowUNCPaths() || f.Roots()[0].FollowSymlinks || cfg.Role() != Direct {
		t.Fatal("default filesystem or runtime policy drift")
	}
	if f.MaxFileSize() != defaultFileBytes || f.MaxWriteSize() != defaultFileBytes ||
		f.MaxListEntries() != 1000 || f.MaxDeleteEntries() != 1000 {
		t.Fatal("default filesystem limits drift")
	}
	if p.Name() != "flashgate" || p.Version() != "0.1.0-dev" || p.MaxMessageSize() != defaultWireBytes ||
		p.MaxArgumentSize() != 12*1024*1024 || p.MaxResponseSize() != defaultWireBytes {
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
		{"missing root", "[files]\ndefault_id = 'work'\n", CategoryMissingRoot},
		{"missing default", "[files.work]\npath = 'ignored'\n", CategoryMissingRoot},
		{"empty default", strings.Replace(root, "'work'", "''", 1), CategoryMissingRoot},
		{"unknown default", strings.Replace(root, "'work'", "'unknown'", 1), CategoryInvalidRoot},
		{"case alias default", strings.Replace(root, "'work'", "'Work'", 1), CategoryInvalidRoot},
		{"default type", strings.Replace(root, "'work'", "42", 1), CategoryInvalidConfig},
		{"empty root", documentWithRoot(t, ""), CategoryInvalidRoot},
		{"whitespace", documentWithRoot(t, "   "), CategoryInvalidRoot},
		{"relative root", documentWithRoot(t, "other"), CategoryInvalidRoot},
		{"CWD", documentWithRoot(t, "."), CategoryInvalidRoot},
		{"root type", "[files]\ndefault_id = 'work'\nwork.path = 42\n", CategoryInvalidConfig},
		{"duplicate root path", root + "work.path = 'x'\n", CategoryInvalidConfig},
		{"duplicate root table", root + "[files.work]\npath = 'x'\n", CategoryInvalidConfig},
		{"duplicate default", root + "default_id = 'work'\n", CategoryInvalidConfig},
		{"missing secondary path", root + "[files.archive]\n", CategoryInvalidRoot},
		{"relative secondary path", root + "archive.path = 'subdir'\n", CategoryInvalidRoot},
		{"unknown root policy", root + "work.read_only = true\n", CategoryInvalidConfig},
		{"unknown root table", root + "[files.work.policy]\n", CategoryInvalidConfig},
		{"legacy group", strings.Replace(root, "[files]", "[filesystem]", 1), CategoryInvalidConfig},
		{"legacy group with named roots", root + "[filesystem]\n", CategoryInvalidConfig},
		{"legacy CWD opt-in", root + "allow_cwd_root = true\n", CategoryInvalidConfig},
		{"case alias", root + "[profiles]\nactive = 'read'\nActive = 'write'\n", CategoryInvalidConfig},
		{"case alias in inline table", "files = { Default_ID = \"work\" }\n", CategoryInvalidConfig},
		{"case alias in table header", "[Files]\ndefault_id = \"work\"\n", CategoryInvalidConfig},
		{"case collision", root + "Work.path = 'x'\n", CategoryInvalidConfig},
		{"empty unknown domain", root + "[capabilities]\n", CategoryInvalidConfig},
		{"unknown policy", root + "[security]\nadmin = true\n", CategoryInvalidConfig},
		{"unknown domain", root + "[capabilities]\nwrite = true\n", CategoryInvalidConfig},
		{"duplicate table", root + "[files]\nmax_write_size = 1\n", CategoryInvalidConfig},
		{"invalid boolean", root + "[security]\nfollow_symlinks = \"true\"\n", CategoryInvalidConfig},
		{"retired read-only key", root + "read_only = true\n", CategoryInvalidConfig},
		{"zero limit", root + "max_write_size = 0\n", CategoryInvalidConfig},
		{"fractional limit", root + "max_write_size = 1.0\n", CategoryInvalidConfig},
		{"negative limit", root + "[protocol]\nmax_message_size = -1\n", CategoryInvalidConfig},
		{"integer overflow", root + "[protocol]\nmax_argument_size = 9223372036854775808\n", CategoryInvalidConfig},
		{"probe limit overflow", root + "max_file_size = 9223372036854775807\n", CategoryInvalidConfig},
		{"future mode", root + "[runtime]\nmode = \"service\"\n", CategoryUnsupportedRuntime},
		{"unsupported sink", root + "[logging]\nsink = \"journald\"\n", CategoryInvalidConfig},
		{"unsupported level", root + "[logging]\nlevel = \"trace\"\n", CategoryInvalidConfig},
		{"unsupported format", root + "[logging]\nformat = \"yaml\"\n", CategoryInvalidConfig},
		{"record cap", root + "[logging]\nmax_record_size = 2\n", CategoryInvalidConfig},
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

func TestDecodeNamedRootDefinitions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	archive := root + string(filepath.Separator) + "archive" + string(filepath.Separator)
	document := fmt.Sprintf("[files]\ndefault_id = 'work'\n[files.work]\npath = %q\n[files.a_7]\npath = %q\n", root, archive)
	cfg, err := Decode([]byte(document), Direct)
	if err != nil {
		t.Fatal(err)
	}
	read := cfg.Profile().Policy().Grants()
	want := []RootDefinition{{ID: "a_7", Path: filepath.Clean(archive), Access: read, Limits: DefaultConfig().Files().Limits()}, {ID: "work", Path: filepath.Clean(root), Access: read, Limits: DefaultConfig().Files().Limits()}}
	equal := func(a, b RootDefinition) bool {
		return a.ID == b.ID && a.Path == b.Path && a.Limits == b.Limits && a.Access.Equal(b.Access)
	}
	snapshot := cfg.Files().Roots()
	if !slices.EqualFunc(snapshot, want, equal) {
		t.Fatalf("unexpected definitions: %v", snapshot)
	}
	ids := snapshot[0].Access.IDs()
	ids[0] = capability.FilesWrite
	snapshot[0].ID, snapshot[1].Path, snapshot[1].Access = "changed", "", capability.Set{}
	if !slices.EqualFunc(cfg.Files().Roots(), want, equal) || !cfg.Files().Roots()[0].Access.Contains(capability.FilesRead) {
		t.Fatal("definition changes escaped the snapshot")
	}
	quotedPath, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "Upper", "x-y", "é", "a b", "1root", "_root", strings.Repeat("x", 65)} {
		t.Run(id, func(t *testing.T) {
			doc := documentWithRoot(t, root) + fmt.Sprintf("%q.path = %s\n", id, quotedPath)
			if _, err := Decode([]byte(doc), Direct); err == nil {
				t.Fatal("accepted an invalid root ID")
			}
		})
	}
	var bounded strings.Builder
	bounded.WriteString(documentWithRoot(t, root))
	for i := range MaxRoots - 1 {
		fmt.Fprintf(&bounded, "r%d.path = %s\n", i, quotedPath)
	}
	if cfg, err := Decode([]byte(bounded.String()), Direct); err != nil || len(cfg.Files().Roots()) != MaxRoots {
		t.Fatalf("root limit rejected: %v", err)
	}
	fmt.Fprintf(&bounded, "excess.path = %s\n", quotedPath)
	_, err = Decode([]byte(bounded.String()), Direct)
	assertCategory(t, err, CategoryInvalidConfig)
}

func TestDecodeFlatNamespaceConflicts(t *testing.T) {
	t.Parallel()
	root := documentWithRoot(t, t.TempDir())
	for _, extra := range []string{
		"[files.roots.archive]\npath = 'retired'\n",
		"[files.roots]\npath = 'reserved'\n",
		"roots = {archive = {path = 'retired'}}\n",
		"[profiles.definitions.other]\ngrants = []\n",
		"[profiles.definitions]\ngrants = []\n",
		"[profiles]\ndefinitions = {other = {grants = []}}\n",
		"[[files.other]]\npath = 'array'\n",
		"[[profiles.other]]\ngrants = []\n",
		"[files.other]\naccess = []\nrights = []\n",
		"[files.other]\ngrants = []\n",
		"[files.other]\ncapabilities = []\n",
		"[profiles]\nunknown_option = true\n",
		"[profiles]\nactive = 'read'\n[profiles.active]\ngrants = []\n",
		"[profiles]\ninclude = []\n[profiles.include]\ngrants = []\n",
		"[profiles]\nunknown = 2025-01-01\n",
	} {
		t.Run(extra, func(t *testing.T) {
			_, err := Decode([]byte(root+extra), Direct)
			assertCategory(t, err, CategoryInvalidConfig)
		})
	}
	for _, reserved := range []string{"default_id", "max_file_size", "max_write_size", "max_scan_size",
		"max_result_size", "max_temp_size", "max_list_entries", "max_delete_entries"} {
		t.Run("files."+reserved, func(t *testing.T) {
			document := fmt.Sprintf("[files.work]\npath = %q\n[files.%s]\npath = %q\n", t.TempDir(), reserved, t.TempDir())
			_, err := Decode([]byte(document), Direct)
			assertCategory(t, err, CategoryInvalidConfig)
		})
	}
	for _, reserved := range []string{"active", "include", "max_risk"} {
		t.Run("profiles."+reserved, func(t *testing.T) {
			_, err := Decode([]byte(root+"[profiles."+reserved+"]\ngrants = []\n"), Direct)
			assertCategory(t, err, CategoryInvalidConfig)
		})
	}
}

func TestDecodeFlatInlineDefinitions(t *testing.T) {
	t.Parallel()
	document := fmt.Sprintf("files = {default_id = 'work', work = {path = %q, access = []}}\n"+
		"profiles = {active = 'custom', include = ['read'], custom = {grants = ['files.write']}}\n", t.TempDir())
	cfg, err := Decode([]byte(document), Direct)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile().ID() != "custom" || !cfg.Profile().Policy().Grants().Contains(capability.FilesWrite) ||
		len(cfg.Files().Roots()[0].Access.IDs()) != 0 {
		t.Fatal("inline tables did not preserve profile composition and explicit empty access")
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
		"max_frame_size = 67108865",
	} {
		_, err = Decode([]byte(documentWithRoot(t, t.TempDir())+"[endpoints]\n"+entry+"\n"), Proxy)
		assertCategory(t, err, CategoryInvalidConfig)
	}
}

func TestLoadUsesExactlyTheTrustedOverride(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "server.toml")
	document := documentWithRoot(t, root) + "[profiles]\nactive = 'write'\n"
	testconfig.Write(t, path, []byte(document))
	t.Setenv("MCP_ROOT", "/legacy")
	t.Setenv("MCP_READ_ONLY", "false")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	cfg, err := Load(Source{Role: Direct, Path: path})
	if err != nil || cfg.Files().Roots()[0].Path != filepath.Clean(root) || cfg.Profile().ID() != ProfileWrite {
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

func TestDefaultConfigSourceProtectionBindsOverlappingAndSeparateRoots(t *testing.T) {
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
		{base, nil},
		{dataRoot, nil},
	} {
		testconfig.Write(t, path, []byte(documentWithRoot(t, test.root)))
		cfg, err := load(Source{Role: Direct}, resolve)
		if err != nil {
			t.Fatal(err)
		}
		guard, err := security.NewPathGuard(cfg.Files().Roots()[0].Path)
		if err != nil {
			t.Fatal(err)
		}
		if _, got := cfg.ProtectDataRoots([]*security.PathGuard{guard}); !errors.Is(got, test.want) {
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
	if _, err := cfg.ProtectDataRoots([]*security.PathGuard{guard}); !errors.Is(err, security.ErrProtectedPath) {
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
	if _, err := unbound.ProtectDataRoots([]*security.PathGuard{guard}); !errors.Is(err, security.ErrProtectedPath) {
		t.Fatalf("unbound source identity did not cause a safe denial: %v", err)
	}
	if err := os.Rename(source, source+".moved"); err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.ProtectDataRoots([]*security.PathGuard{guard}); !errors.Is(err, security.ErrProtectedPath) {
		t.Fatalf("missing source did not cause a safe denial: %v", err)
	}
}

func TestLinkedSourceStillAppliesDocumentSizeLimit(t *testing.T) {
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
	assertCategory(t, err, CategoryInvalidConfig)
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
