package fsapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func directFixture(t *testing.T, profileName string) (*roots.Registry, config.EffectiveProfile, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	document := "[files]\ndefault_id='work'\nwork.path=" + strconv.Quote(dir) +
		"\nwork.access=['files.read','files.write']\n" +
		"reader.path=" + strconv.Quote(dir) + "\n" +
		"[profiles]\nactive=" + strconv.Quote(profileName) + "\n"
	testconfig.Write(t, path, []byte(document))
	cfg, err := config.Load(config.Source{Role: config.Direct, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := roots.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return registry, cfg.Profile(), dir
}

func TestDirectFilesystemEffectsAndDeniedSources(t *testing.T) {
	r, profile, dir := directFixture(t, "write")
	service, err := NewDirect(r, profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewDirect(r, config.EffectiveProfile{}); !errors.Is(err, execution.ErrMismatch) {
		t.Fatal(err)
	}
	if _, err := service.ForRequest(t.Context(), "payload"); !errors.Is(err, execution.ErrMismatch) {
		t.Fatal(err)
	}
	f, err := service.ForRequest(t.Context(), execution.MCP20251125)
	if err != nil {
		t.Fatal(err)
	}
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	scope := fs.Operand{RootID: "work", Path: "new"}
	created, err := f.Mkdir(scope)
	check(err)
	if !created {
		t.Fatal("mkdir did not create the directory")
	}
	a := fs.Operand{RootID: "work", Path: "new/a.txt"}
	b := fs.Operand{RootID: "work", Path: "new/b.txt"}
	c := fs.Operand{RootID: "work", Path: "new/c.txt"}
	check(f.Write(a, []byte("ordinary"), false))
	check(f.Copy(a, b, false))
	check(f.Move(b, c, false))
	got, err := f.Read(c, 32)
	check(err)
	if string(got) != "ordinary" {
		t.Fatal("copy/move/read changed the native value")
	}
	metadata, err := f.Stat(c)
	check(err)
	original, err := r.Stat(c)
	check(err)
	if metadata != original {
		t.Fatal("stat result differs from the native domain")
	}
	listed, err := f.List(scope)
	check(err)
	if len(listed) != 2 {
		t.Fatalf("list length = %d", len(listed))
	}
	check(f.Delete(c, false))
	_, err = os.Stat(filepath.Join(dir, "new", "c.txt"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("delete effect: %v", err)
	}

	configOperand := fs.Operand{RootID: "work", Path: "settings.toml"}
	for _, protected := range []fs.Operand{configOperand, {RootID: "work", Path: "alias.toml"}} {
		if protected.Path != configOperand.Path {
			check(os.Link(filepath.Join(dir, configOperand.Path), filepath.Join(dir, protected.Path)))
		}
		got, err := f.Read(protected, 4096)
		if err == nil || len(got) != 0 {
			t.Fatal("protected source disclosed data")
		}
		if err := f.Copy(protected, c, true); err == nil {
			t.Fatal("protected copy source reached publication")
		}
	}
	if err := f.Copy(a, fs.Operand{RootID: "reader", Path: c.Path}, false); !errors.Is(err, fs.ErrCrossRootOperation) {
		t.Fatal(err)
	}
	if err := f.Write(fs.Operand{RootID: "reader", Path: c.Path}, []byte("cannot-write"), false); !errors.Is(err, fs.ErrRootAccessDenied) {
		t.Fatal("read-only root did not deny a write", err)
	}
	if data, err := f.Read(fs.Operand{RootID: "unknown", Path: a.Path}, 32); len(data) != 0 || !errors.Is(err, fs.ErrUnknownRoot) {
		t.Fatal("unknown resource returned data", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new", "c.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("denied copy changed the destination")
	}
	got, err = f.Read(a, 32)
	check(err)
	if string(got) != "ordinary" {
		t.Fatal("ordinary sibling denied after a protected read")
	}
}

func TestDeniedReceiptAndCanceledRequestHaveNoEffect(t *testing.T) {
	r, profile, dir := directFixture(t, "write")
	service, err := NewDirect(r, profile)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f, err := service.ForRequest(ctx, execution.MCP20251125)
	if err != nil {
		t.Fatal(err)
	}
	target := fs.Operand{RootID: "work", Path: "result.txt", AdmitEffect: func() error { return fs.ErrLimitExceeded }}
	if err := f.Write(target, []byte("x"), false); !errors.Is(err, fs.ErrLimitExceeded) {
		t.Fatal(err)
	}
	target.AdmitEffect = func() error { cancel(); return nil }
	if err := f.Write(target, []byte("x"), false); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if data, err := f.Read(target, 32); len(data) != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled read delivered data", err)
	}
	if _, err := os.Stat(filepath.Join(dir, target.Path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("denied effect touched target", err)
	}
}

type simulatedBackend func(context.Context, execution.Invocation[command]) (execution.Completion[result], error)

func (f simulatedBackend) Execute(ctx context.Context, call execution.Invocation[command]) (execution.Completion[result], error) {
	return f(ctx, call)
}

func TestTypedReadParityAndMismatchedOperands(t *testing.T) {
	r, profile, dir := directFixture(t, "write")
	testconfig.Write(t, filepath.Join(dir, "ordinary.txt"), []byte("ordinary"))
	const generation = "test-generation"
	runtime, err := execution.NewRuntime(generation)
	if err != nil {
		t.Fatal(err)
	}
	session, native, err := execution.BindDirectSTDIO(runtime)
	if err != nil {
		t.Fatal(err)
	}
	req, err := session.Request(execution.MCP20251125, 4096)
	if err != nil {
		t.Fatal(err)
	}
	operations, err := fs.BindOperations(profile.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	routes, err := bindRoutes(r, operations)
	if err != nil {
		t.Fatal(err)
	}
	local := currentProcess{roots: r, operations: operations, identity: native, generation: generation}
	direct := execution.Registration[command, result]{ID: execution.CurrentProcess, Identity: native, Backend: local}
	identity := execution.Identity{Account: "test-account", Binding: "fake-worker"}
	delegate := execution.Registration[command, result]{ID: execution.UserWorker, Identity: identity, Backend: simulatedBackend(func(ctx context.Context, call execution.Invocation[command]) (execution.Completion[result], error) {
		bind := call.Binding()
		if err := bind.Verify(ctx, execution.UserWorker, identity, generation); err != nil {
			return execution.Completion[result]{}, err
		}
		if !local.matches(bind, call.Command()) {
			return execution.Completion[result]{}, execution.ErrMismatch
		}
		data, err := local.execute(call.Command(), nil)
		return execution.Complete(bind, data), err
	})}
	read := command{name: readFile, path: operand{"work", "ordinary.txt"}, maxBytes: 4096}
	for _, reg := range []execution.Registration[command, result]{direct, delegate} {
		d, err := execution.NewDispatcher(runtime, profile, reg.ID, routes, reg)
		if err != nil {
			t.Fatal(err)
		}
		value, err := d.Dispatch(t.Context(), req, read, nil)
		if err != nil || string(value.content) != "ordinary" {
			t.Fatalf("%s: expected allowed parity, err = %v", reg.ID, err)
		}
		protected := read
		protected.path.path = "settings.toml"
		if value, err := d.Dispatch(t.Context(), req, protected, nil); len(value.content) != 0 || err == nil {
			t.Fatalf("%s: protected source returned data", reg.ID)
		}
	}
	forged := local
	forged.identity.Binding = "unrelated-process"
	wrongNative, err := execution.NewDispatcher(runtime, profile, direct.ID, routes,
		execution.Registration[command, result]{ID: direct.ID, Identity: forged.identity, Backend: forged})
	if err != nil {
		t.Fatal(err)
	}
	if value, err := wrongNative.Dispatch(t.Context(), req, read, nil); len(value.content) != 0 || !errors.Is(err, execution.ErrMismatch) {
		t.Fatalf("native identity mismatch disclosed data: %v", err)
	}
	if value, err := wrongNative.Dispatch(t.Context(), req, command{name: "fixture.unknown"}, nil); len(value.content) != 0 || !errors.Is(err, execution.ErrDenied) {
		t.Fatalf("undeclared typed operation reached execution: %v", err)
	}
	for _, invalid := range []string{"role", "root", "path", "variant"} {
		misbound := routes
		want := execution.ErrMismatch
		if invalid == "role" {
			want = execution.ErrDenied
		}
		if invalid == "variant" {
			misbound.Key = func(cmd command) execution.OperationKey {
				return execution.OperationKey{Name: cmd.name, Variant: execution.DefaultVariant}
			}
		}
		misbound.Resolve = func(ctx context.Context, cmd command) ([]execution.Resource, error) {
			resources, err := routes.Resolve(ctx, cmd)
			if err != nil {
				return nil, err
			}
			switch invalid {
			case "role":
				resources[0], resources[1] = resources[1], resources[0]
			case "root":
				resources[0].ID = "reader"
			case "path":
				resources[1].Relative = "different.txt"
			}
			return resources, nil
		}
		d, err := execution.NewDispatcher(runtime, profile, direct.ID, misbound, direct)
		if err != nil {
			t.Fatal(err)
		}
		copy := command{name: copyPath, path: read.path, target: operand{"work", "copy.txt"}, overwrite: true}
		value, err := d.Dispatch(t.Context(), req, copy, nil)
		if len(value.content) != 0 || !errors.Is(err, want) || strings.Contains(err.Error(), dir) {
			t.Fatalf("%s: %v", invalid, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "copy.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("mismatched copy reached the native effect")
	}
}
