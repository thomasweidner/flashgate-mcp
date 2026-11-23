package roots

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestWorkingDirectoryPolicyAndPathBinding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"inside/one/two/three", "inside/sibling", "excluded/deep", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(root, "active.toml")
	r, catalog := workingDirectoryRegistry(t, source, "mutation", "['commands.run','files.read','files.write']",
		rootTable(t, "work", root)+"policy='allowed'\nexcluded=['excluded']\n"+
			rootTable(t, "blocked", filepath.Join(root, "inside"))+"policy='allowed'\nallow_process_cwd=false\n"+
			rootTable(t, "exalias", filepath.Join(root, "excluded"))+"policy='allowed'\n"+
			rootTable(t, "overlap", filepath.Join(root, "inside"))+"allow_process_cwd=true\n"+
			"[policy.allowed]\nallow_process_cwd=true\n")
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	request := func(id config.RootID, path string) WorkingDirectoryOperand {
		return WorkingDirectoryOperand{RootID: id, RelativePath: path}
	}
	operation := cwdOperation(t, catalog, capability.RiskRead, capability.CommandsRun)
	read := cwdRequirement(t, catalog, capability.FilesRead)
	for _, operand := range []WorkingDirectoryOperand{request("work", "."), request("work", "inside/one/two/three"),
		request("work", "inside/sibling"), request("overlap", "one/two/three")} {
		directory, err := r.CheckWorkingDirectory(operand, operation, read)
		if err != nil || directory.Operand() != operand || directory.Revalidate() != nil {
			t.Fatalf("%+v did not bind to the exact authorized directory: %v", operand, err)
		}
	}
	for _, tc := range []struct {
		operand WorkingDirectoryOperand
		want    error
	}{
		{request("blocked", "one"), fs.ErrRootAccessDenied},
		{request("Work", "."), fs.ErrUnknownRoot},
		{request("", "."), fs.ErrUnknownRoot},
		{request("work", ""), ErrWorkingDirectoryDenied},
		{request("work", "  "), ErrWorkingDirectoryDenied},
		{request("work", "./"), ErrWorkingDirectoryDenied},
		{request("work", "missing"), ErrWorkingDirectoryDenied},
		{request("work", "active.toml"), ErrWorkingDirectoryDenied},
		{request("work", "../"), ErrWorkingDirectoryDenied},
		{request("work", "inside/../inside"), ErrWorkingDirectoryDenied},
		{request("work", root), ErrWorkingDirectoryDenied},
		{request("work", `\\server\share`), ErrWorkingDirectoryDenied},
		{request("work", ".hidden"), ErrWorkingDirectoryDenied},
		{request("work", "excluded/deep"), ErrWorkingDirectoryDenied},
		{request("exalias", "."), ErrWorkingDirectoryDenied},
	} {
		if err := r.PreflightProcessDispatch(tc.operand, operation, read, func(WorkingDirectory) bool {
			t.Errorf("%+v reached the enforcement check", tc.operand)
			return true
		}, func(WorkingDirectory) error {
			t.Errorf("%+v reached dispatch", tc.operand)
			return nil
		}); !errors.Is(err, tc.want) {
			t.Errorf("%+v returned %v, want %v", tc.operand, err, tc.want)
		}
	}
	if after, err := os.ReadFile(source); err != nil || !bytes.Equal(content, after) {
		t.Fatalf("policy evaluation changed the bootstrap document: %v", err)
	}
}

func TestWorkingDirectoryDispatchRequirements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	document := rootTable(t, "work", root) + "allow_process_cwd=true\naccess=['files.read','files.write']\n" +
		rootTable(t, "disabled", root) + "access=['files.write']\n" +
		rootTable(t, "partial", root) + "allow_process_cwd=true\naccess=['files.read']\n"
	registry := func(ceiling, grants string) (*Registry, capability.Catalog) {
		t.Helper()
		return workingDirectoryRegistry(t, filepath.Join(t.TempDir(), "active.toml"), ceiling, grants, document)
	}
	r, catalog := registry("mutation", "['commands.run','processes.manage','files.read','files.write']")
	operation := cwdOperation(t, catalog, capability.RiskRead, capability.CommandsRun)
	process := cwdOperation(t, catalog, capability.RiskRead, capability.ProcessesManage)
	read := cwdRequirement(t, catalog, capability.FilesRead)
	both := cwdRequirement(t, catalog, capability.FilesRead, capability.FilesWrite)
	write := cwdRequirement(t, catalog, capability.FilesWrite)
	withoutProcess, filesystem := registry("mutation", "['files.read','files.write']")
	readCeiling, limited := registry("read", "['commands.run','files.read']")
	withoutResource, commands := registry("mutation", "['commands.run']")
	const denied = "process dispatch reached the sink after denial"
	for _, tc := range []struct {
		name        string
		r           *Registry
		id          config.RootID
		operation   capability.Operation
		requirement capability.Requirement
	}{
		{"command grant without CWD", r, "disabled", operation, write},
		{"CWD without operation", r, "work", capability.Operation{}, read},
		{"file-write operation cannot dispatch", r, "work", cwdOperation(t, catalog, capability.RiskRead, capability.FilesWrite), write},
		{"CWD without command grant", withoutProcess, "work", cwdOperation(t, filesystem, capability.RiskRead, capability.CommandsRun), cwdRequirement(t, filesystem, capability.FilesRead)},
		{"operation from another snapshot", r, "work", cwdOperation(t, commands, capability.RiskRead, capability.CommandsRun), read},
		{"missing operand declaration", r, "work", operation, capability.Requirement{}},
		{"incomplete root access", r, "partial", process, both},
		{"foreign resource snapshot", r, "work", operation, cwdRequirement(t, commands, capability.FilesRead)},
		{"global command without resource grant", withoutResource, "work", cwdOperation(t, commands, capability.RiskRead, capability.CommandsRun), cwdRequirement(t, commands, capability.FilesRead)},
		{"risk ceiling", readCeiling, "work", cwdOperation(t, limited, capability.RiskRead, capability.CommandsRun), cwdRequirement(t, limited, capability.FilesRead)},
		{"declared extra right", r, "work", cwdOperation(t, catalog, capability.RiskRead, capability.CommandsRun, capability.SearchRun), read},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.r.PreflightProcessDispatch(WorkingDirectoryOperand{tc.id, "."}, tc.operation, tc.requirement,
				func(WorkingDirectory) bool { t.Error("authorization denial reached enforcement"); return true },
				func(WorkingDirectory) error { t.Error(denied); return nil })
			if !errors.Is(err, fs.ErrRootAccessDenied) {
				t.Fatalf("unexpected authorization outcome: %v", err)
			}
		})
	}
	selection := WorkingDirectoryOperand{"work", "."}
	sink := func(WorkingDirectory) error { t.Error(denied); return nil }
	supported := func(WorkingDirectory) bool { return true }
	for _, err := range []error{
		r.PreflightProcessDispatch(selection, operation, read, nil, sink),
		r.PreflightProcessDispatch(selection, operation, read, func(WorkingDirectory) bool { return false }, sink),
		r.PreflightProcessDispatch(selection, operation, read, supported, nil),
	} {
		if !errors.Is(err, ErrProcessDispatchUnavailable) {
			t.Errorf("missing enforcement or sink did not deny: %v", err)
		}
	}
	calls := 0
	marker := errors.New("dispatch result")
	err := r.PreflightProcessDispatch(selection, process, both, supported, func(directory WorkingDirectory) error {
		calls++
		if directory.Operand() != selection || directory.Revalidate() != nil {
			t.Error("dispatch received the wrong directory")
		}
		return marker
	})
	if calls != 1 || !errors.Is(err, marker) {
		t.Fatalf("non-executing dispatch returned %v after %d calls", err, calls)
	}
	if err := (WorkingDirectory{}).Revalidate(); !errors.Is(err, ErrWorkingDirectoryDenied) {
		t.Fatalf("unbound directory returned %v", err)
	}
}

func TestWorkingDirectoryRechecksObjectAfterEnforcementProbe(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "selected")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	r, catalog := workingDirectoryRegistry(t, filepath.Join(t.TempDir(), "active.toml"), "mutation", "['commands.run','files.read']",
		rootTable(t, "work", root)+"allow_process_cwd=true\n")
	operation := cwdOperation(t, catalog, capability.RiskRead, capability.CommandsRun)
	read := cwdRequirement(t, catalog, capability.FilesRead)
	err := r.PreflightProcessDispatch(WorkingDirectoryOperand{"work", "selected"}, operation, read,
		func(WorkingDirectory) bool {
			if err := os.Rename(dir, filepath.Join(root, "saved")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			return true
		}, func(WorkingDirectory) error { t.Error("directory swap reached dispatch"); return nil })
	if !errors.Is(err, ErrWorkingDirectoryDenied) {
		t.Fatalf("directory substitution returned %v", err)
	}
	if _, err := r.CheckWorkingDirectory(WorkingDirectoryOperand{"work", "saved"}, operation, read); err != nil {
		t.Fatalf("authorized sibling was rejected: %v", err)
	}
}

func workingDirectoryRegistry(t *testing.T, source, ceiling, grants, tables string) (*Registry, capability.Catalog) {
	t.Helper()
	testconfig.Write(t, source, []byte("[files]\ndefault_id='work'\n"+tables+
		"[profiles]\nactive='execution'\nmax_risk='"+ceiling+"'\n"+
		"[profiles.execution]\ngrants="+grants+"\n"))
	cfg, err := config.Load(config.Source{Role: config.Direct, Path: source})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return registry, cfg.Profile().Catalog()
}

func cwdOperation(t *testing.T, catalog capability.Catalog, floor capability.Risk, ids ...capability.ID) capability.Operation {
	t.Helper()
	operation, err := catalog.NewOperation(floor, ids...)
	if err != nil {
		t.Fatal(err)
	}
	return operation
}

func cwdRequirement(t *testing.T, catalog capability.Catalog, ids ...capability.ID) capability.Requirement {
	t.Helper()
	requirement, err := catalog.NewRequirement(ids...)
	if err != nil {
		t.Fatal(err)
	}
	return requirement
}
