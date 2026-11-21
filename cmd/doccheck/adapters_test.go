package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestRuntimeLaunchSource(t *testing.T) {
	cases := []struct {
		name   string
		source string
		reject bool
	}{
		{"shell", `import runner "os/exec"; var _ = runner.Command("sh", "-c", "echo")`, true},
		{"interpreter", `import "os/exec"; var _ = exec.Command("pwsh", "-File", "adapter.ps1")`, true},
		{"unreviewed native executable", `import "os/exec"; var _ = exec.Command("/bin/rg")`, true},
		{"os launch function value", `import host "os"; var _ = host.StartProcess`, true},
		{"dot import", `import . "os"; var _ = StartProcess`, true},
		{"native process launch", `import win "golang.org/x/sys/windows"; var _ = win.CreateProcessAsUser`, true},
		{"dynamic loader", `import "syscall"; var _ = syscall.NewLazyDLL`, true},
		{"raw syscall", `import "syscall"; var _ = syscall.RawSyscall6`, true},
		{"linker alias", "import _ \"unsafe\"\n//go:linkname launch os.StartProcess\nfunc launch()", true},
		{"native filesystem", `import "syscall"; var _ = syscall.MoveFile`, false},
		{"native process observation", `import "syscall"; var _ = syscall.Getpid`, false},
		{"Go filesystem", `import "os"; var _ = os.Stat`, false},
		{"plain text", `const label = "pwsh os/exec"; /* syscall.Syscall */`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := inspectRuntimeSource("adapter.go", "fixture", []byte("package adapter\n"+tc.source))
			if (len(findings) != 0) != tc.reject {
				t.Fatalf("reject = %t, findings = %+v", tc.reject, findings)
			}
			for _, finding := range findings {
				if finding.Rule != "RUNTIME_LAUNCH" || finding.Line < 2 {
					t.Fatalf("unexpected finding: %+v", finding)
				}
			}
		})
	}
}

func TestRuntimeLoaderExceptionOnlyAllowsFixedWindowsConfigAPIs(t *testing.T) {
	const config = "internal/config/source_windows.go"
	for _, test := range []struct {
		path, target, expression string
		allowed                  bool
	}{
		{config, "windows/amd64", `syscall.NewLazyDLL("advapi32.dll").NewProc("GetSecurityInfo")`, true},
		{config, "windows/arm64", `syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")`, true},
		{config, "windows/amd64", `syscall.NewLazyDLL("kernel32.dll").NewProc("CreateProcessW")`, false},
		{config, "windows/amd64", `syscall.NewLazyDLL(name).NewProc("GetSecurityInfo")`, false},
		{config, "windows/amd64", `syscall.NewLazyDLL("untrusted.dll").NewProc("GetSecurityInfo")`, false},
		{config, "linux/amd64", `syscall.NewLazyDLL("advapi32.dll").NewProc("GetSecurityInfo")`, false},
		{"other.go", "windows/amd64", `syscall.NewLazyDLL("advapi32.dll").NewProc("GetSecurityInfo")`, false},
	} {
		source := "package config\nimport \"syscall\"\nvar _ = " + test.expression
		if findings := inspectRuntimeSource(test.path, test.target, []byte(source)); (len(findings) == 0) != test.allowed {
			t.Fatalf("%s %s %s: unexpected findings %+v", test.path, test.target, test.expression, findings)
		}
	}
}

func TestRuntimeGraphUsesShippedSources(t *testing.T) {
	root := t.TempDir()
	fixtures := map[string]string{
		"go.mod": "module example.com/adapterfixture\n\ngo 1.26\n",
		"cmd/server/main.go": `package main
import _ "example.com/adapterfixture/internal/adapter"
func main() {}
`,
		"cmd/server/main_test.go": `package main
import "os/exec"
var _ = exec.Command("sh")
`,
		"cmd/mcpcheck/main.go": `package main
import "os/exec"
func main() { _ = exec.Command("pwsh") }
`,
		"internal/adapter/baseline.go": `package adapter
import "os"
var _ = os.Stat
`,
		"internal/adapter/launch_linux_arm64.go": `package adapter
import launcher "os/exec"
var _ = launcher.Command("bash")
`,
		"internal/adapter/custom.go": `//go:build custom

package adapter
import "os/exec"
var _ = exec.Command("python")
`,
	}
	for name, source := range fixtures {
		writeFixture(t, root, name, []byte(source))
	}
	t.Setenv("GOFLAGS", "-tags=custom")
	findings := checkRuntime(root)
	if len(findings) != 1 || findings[0].Path != "internal/adapter/launch_linux_arm64.go" ||
		findings[0].Rule != "RUNTIME_LAUNCH" || !strings.Contains(findings[0].Detail, "linux/arm64") {
		t.Fatalf("want the shipped Linux arm64 adapter finding, got %+v", findings)
	}
}

func TestRuntimeGraphFailure(t *testing.T) {
	findings := checkRuntimeTarget(t.TempDir(), runtimeTarget{"linux", "amd64"})
	if len(findings) != 1 || findings[0].Rule != "RUNTIME_GRAPH" {
		t.Fatalf("want fail-closed graph finding, got %+v", findings)
	}
}

const fixtureAdapterEvidence = `- **authorization**: Require effective authorization before dispatch and deny fallback for unauthorized requests.
- **bounded-resources**: Prove bounded resource behavior for the actual operation and failure paths.
- **gated-activation**: Keep unproved routes disabled or return a safe denial before activation.
- **native-platform**: Exercise the actual native backend on each applicable platform.
- **protected-effects**: Deny protected reads, mutations and disclosure across direct and delegated effects.
`

func adapterFixture(inventory string, families map[string]string, backlog string) (string, string) {
	var plan strings.Builder
	plan.WriteString(adapterInventoryStart + "\n" + inventory + adapterInventoryEnd + "\n")
	plan.WriteString(adapterContractStart + "\n" + fixtureAdapterEvidence)
	for i, key := range adapterFamilyKeys {
		owners := families[key]
		if owners == "" {
			owners = fmt.Sprintf("BL-%03d", 901+i)
			backlog += "| " + owners + " | Completed | Historical owner | Retained fixture |\n"
		}
		fmt.Fprintf(&plan, "<a id=\"adapter-acceptance-%s\"></a>\n<a id=\"bl-220-%s\"></a>\n\n### %s acceptance\n\n**Applicable owners** %s\n", key, key, key, owners)
	}
	plan.WriteString(adapterContractEnd + "\n")
	return plan.String(), backlog
}

func TestAdapterBindings(t *testing.T) {
	const inventory = "| Family | Public names | Owner | Milestone / state | Eligible capability / class | Adapter acceptance |\n" +
		"|---|---|---|---|---|---|\n" +
		"| Test | `future_tool` | BL-987 | Post-1.0 / CANDIDATE | Scoped / O | [Acceptance](#bl-220-native-tools) |\n" +
		"| Historical | `existing_tool` | BL-986 | 1.0 / IMPLEMENTED | Scoped / R | — |\n"
	const native = "[Adapter acceptance](docs/planning/tool-adapters.md#bl-220-native-tools)"
	const operations = "[Adapter acceptance](docs/planning/tool-adapters.md#bl-220-operations)"
	const public = "| BL-987 | **Planned** | New tool | " + native + ". |\n"
	const internal = "| BL-988 | **Planned** | Internal dispatch | " + operations + ". |\n"
	const rows = public + internal +
		"| BL-984 | **Planned** | Unrelated work | Separate contract |\n" +
		"| BL-986 | Completed | Existing tool | Historical acceptance |\n" +
		"| BL-989 | Completed | Historical internal work | Historical acceptance |\n"
	plan, backlog := adapterFixture(inventory, map[string]string{"native-tools": "BL-987", "operations": "BL-988–BL-989"}, rows)
	if findings := checkAdapterBindings(plan, backlog); len(findings) != 0 {
		t.Fatalf("valid binding rejected: %+v", findings)
	}
	cases := []struct {
		name    string
		plan    string
		backlog string
		detail  string
	}{
		{"existing wrong family", plan, strings.Replace(backlog, native, operations, 1), "link to #adapter-acceptance-native-tools for BL-987"},
		{"missing internal link", plan, strings.Replace(backlog, operations, "Reuse BL-220", 1), "link to #adapter-acceptance-operations for BL-988"},
		{"public row owner drift", strings.Replace(plan, "| BL-987 |", "| BL-988 |", 1), backlog, "different adapter applicability: BL-988"},
		{"public row family drift", strings.Replace(plan, "[Acceptance](#bl-220-native-tools)", "[Acceptance](#bl-220-operations)", 1), backlog, "different adapter applicability: BL-987"},
		{"missing canonical owner", plan, strings.Replace(backlog, internal, "", 1), "exactly one canonical backlog row: BL-988"},
		{"duplicate canonical owner", plan, backlog + public, "exactly one canonical backlog row: BL-987"},
		{"inactive planned public owner", plan, strings.Replace(backlog, "| BL-987 | **Planned** |", "| BL-987 | Completed |", 1), "needs an active backlog owner: BL-987"},
		{"missing applicability", strings.Replace(plan, "BL-988–BL-989", "BL-989", 1), backlog, "no matching applicability declaration: BL-988"},
		{"duplicate applicability", strings.Replace(plan, "BL-988–BL-989", "BL-987–BL-989", 1), backlog, "Multiple adapter applicability declarations for BL-987"},
		{"code example", plan, strings.Replace(backlog, native, "`"+native+"`", 1), "link to #adapter-acceptance-native-tools for BL-987"},
		{"active implementation still requires acceptance", strings.Replace(plan, "Post-1.0 / CANDIDATE", "Post-1.0 / IMPLEMENTED", 1),
			strings.Replace(strings.Replace(backlog, "**Planned**", "**In Progress**", 1), native, "Reuse BL-220", 1), "link to #adapter-acceptance-native-tools for BL-987"},
		{"missing security evidence", strings.Replace(plan, "**authorization**", "**unrelated**", 1), backlog, "Missing or incorrect adapter acceptance evidence: authorization"},
		{"weakened protected effects", strings.Replace(plan, "Deny protected reads, mutations and disclosure", "Log protected reads, mutations and disclosure", 1), backlog, "Missing or incorrect adapter acceptance evidence: protected-effects"},
		{"missing bounded resources", strings.Replace(plan, "**bounded-resources**", "**unrelated**", 1), backlog, "Missing or incorrect adapter acceptance evidence: bounded-resources"},
		{"weakened activation evidence", strings.Replace(plan, "Keep unproved routes disabled", "Enable unproved routes", 1), backlog, "Missing or incorrect adapter acceptance evidence: gated-activation"},
		{"missing native platform", strings.Replace(plan, "**native-platform**", "**unrelated**", 1), backlog, "Missing or incorrect adapter acceptance evidence: native-platform"},
		{"hidden contract", strings.Replace(plan, adapterContractStart, "<!-- archived\n"+adapterContractStart, 1), backlog, "Exactly one complete adapter acceptance contract is required"},
		{"missing semantic family", strings.Replace(plan, "<a id=\"adapter-acceptance-native-tools\"></a>", "### BL-220 native tools", 1), backlog, "Missing adapter applicability declaration for #adapter-acceptance-native-tools"},
		{"duplicate legacy anchor", plan + "<a id=\"bl-220-native-tools\"></a>\n", backlog, "Duplicate adapter acceptance anchor: #bl-220-native-tools"},
		{"misbound legacy anchor", strings.Replace(plan, "<a id=\"bl-220-native-tools\"></a>", "<a id=\"bl-220-native-tools-old\"></a>", 1), backlog, "Compatibility anchor points to a different adapter acceptance family"},
		{"missing inventory", strings.Replace(plan, adapterInventoryEnd, "", 1), backlog, "complete public tool inventory must be present"},
		{"duplicate tool", strings.Replace(plan, "existing_tool", "future_tool", 1), backlog, "Duplicate public tool name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := checkAdapterBindings(tc.plan, tc.backlog)
			for _, result := range findings {
				if result.Rule == "ADAPTER_BINDING" && strings.Contains(result.Detail, tc.detail) {
					return
				}
			}
			t.Fatalf("want %q, got %+v", tc.detail, findings)
		})
	}
	renamed := strings.ReplaceAll(plan, "### native-tools acceptance", "### First-party executables")
	renamed = strings.Replace(renamed, "[Acceptance](#bl-220-native-tools)", "[Acceptance](#adapter-acceptance-native-tools)", 1)
	if got := checkAdapterBindings(renamed, backlog); len(got) != 0 {
		t.Fatalf("semantic inventory link with a legacy backlog link and renamed heading rejected: %+v", got)
	}
}

func TestStagedPublicAdapterBindings(t *testing.T) {
	const inventory = "| Process | `fixture_start` | BL-987 / activation BL-151 | 1.0 / PLANNED | Scoped / O | [Acceptance](#bl-220-managed-execution) |\n"
	const binding = "[Adapter acceptance](docs/planning/tool-adapters.md#bl-220-managed-execution)"
	const transfer = "[Launch transfer to BL-151](docs/planning/implementation-contract.md#managed-process-activation)"
	const rows = "| BL-987 | Completed | Non-launching foundation | " + binding + ". " + transfer + ". BL-151 accepts real start evidence. |\n" +
		"| BL-151 | **Planned** | Activate | " + binding + ". Accept transferred launch obligations from BL-987. |\n" +
		"| BL-988 | **Planned** | Other same-family work | " + binding + ". |\n"
	plan, backlog := adapterFixture(inventory, map[string]string{"managed-execution": "BL-151, BL-987–BL-988"}, rows)
	if got := checkAdapterBindings(plan, backlog); len(got) != 0 {
		t.Fatalf("planned surface with a completed foundation was rejected: %+v", got)
	}
	cases := []struct {
		name, plan, backlog, detail string
	}{
		{"activation owner missing", strings.Replace(plan, " / activation BL-151", "", 1), backlog, "must match its BL-151 launch transfer"},
		{"wrong owner in same family", strings.Replace(plan, "/ activation BL-151", "/ activation BL-988", 1), backlog, "must match its BL-151 launch transfer"},
		{"foundation transfer missing", plan, strings.Replace(backlog, transfer, "No transfer", 1), "must match its BL-151 launch transfer"},
		{"premature implementation", strings.Replace(plan, "1.0 / PLANNED", "1.0 / IMPLEMENTED", 1), backlog, "cannot be implemented before activation"},
		{"stale planned state after activation", plan, strings.Replace(backlog, "| BL-151 | **Planned**", "| BL-151 | Completed", 1), "needs an active backlog owner"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkAdapterBindings(tc.plan, tc.backlog)
			expectBindingFinding(t, got, tc.detail)
		})
	}
}

func TestInternalLaunchAcceptanceTransfers(t *testing.T) {
	const inventory = "| Historical | `existing_tool` | BL-986 | 1.0 / IMPLEMENTED | Scoped / R | — |\n"
	const ops = "[Adapter acceptance](docs/planning/tool-adapters.md#bl-220-operations)"
	const process = "[Adapter acceptance](docs/planning/tool-adapters.md#bl-220-managed-execution)"
	const transfer = "[Launch transfer to BL-151](docs/planning/implementation-contract.md#managed-process-activation)"
	const receipt = "Accept transferred launch obligations from BL-097, BL-164, BL-252–BL-253."
	const rows = "| BL-097 | **Planned** | Decide dispatch | " + ops + ". " + transfer + ". BL-151 accepts native dispatch evidence. |\n" +
		"| BL-151 | **Planned** | Activate | " + process + ". " + receipt + " |\n" +
		"| BL-164 | Completed | Job limit tests | " + transfer + ". BL-151 accepts native exhaustion evidence. |\n" +
		"| BL-252 | **Planned** | Race checks | " + process + ". " + transfer + ". BL-151 accepts native race evidence. |\n" +
		"| BL-253 | **Planned** | CI checks | " + process + ". " + transfer + ". BL-151 accepts native CI evidence. |\n" +
		"| BL-986 | Completed | Existing tool | Historical acceptance |\n"
	plan, backlog := adapterFixture(inventory, map[string]string{"operations": "BL-097, BL-164", "managed-execution": "BL-151, BL-252–BL-253"}, rows)
	if got := checkAdapterBindings(plan, backlog); len(got) != 0 {
		t.Fatalf("complete acceptance rejected: %+v", got)
	}
	cases := []struct {
		name, old, replacement, detail string
	}{
		{"BL-097 donor link removed", transfer + ". BL-151 accepts native dispatch evidence.", "Prepare Go fixtures.", "no matching launch transfer from BL-097"},
		{"BL-164 omitted by BL-151", "BL-097, BL-164, BL-252", "BL-097, BL-252", "receipt omits launch donor BL-164"},
		{"wrong recipient", transfer, strings.Replace(transfer, "BL-151", "BL-152", 1), "consistent launch transfer to BL-151 from BL-097"},
		{"wrong destination", transfer, strings.Replace(transfer, "#managed-process-activation", "#closure", 1), "consistent launch transfer to BL-151 from BL-097"},
		{"duplicate donor link", transfer, transfer + " " + transfer, "consistent launch transfer to BL-151 from BL-097"},
		{"conflicting donor declarations", transfer, transfer + " " + strings.Replace(transfer, "BL-151", "BL-152", 1), "consistent launch transfer to BL-151 from BL-097"},
		{"contradictory named recipient", "BL-151 accepts native dispatch", "BL-152 accepts native dispatch", "consistent launch transfer to BL-151 from BL-097"},
		{"overlapping receipt range", "BL-252–BL-253.", "BL-252–BL-253, BL-253.", "duplicate launch donor in BL-151 receipt: BL-253"},
		{"malformed donor range", "BL-252–BL-253.", "BL-253–BL-252.", "invalid donor ID or range in BL-151 receipt"},
		{"duplicate receiver receipt", receipt, receipt + " " + receipt, "exactly one launch acceptance receipt"},
		{"wrong receiving row", "| BL-151 | **Planned** | Activate", "| BL-152 | **Planned** | Activate", "Only BL-151 may receive managed-process launch acceptance"},
		{"unknown canonical donor", "BL-252–BL-253.", "BL-252–BL-253, BL-999.", "unknown canonical donor: BL-999"},
		{"malformed donor owner ID", "| BL-097 |", "| BL-97 |", "Invalid canonical owner ID for launch acceptance: BL-97"},
		{"inactive donor", "| BL-097 | **Planned**", "| BL-097 | Rejected", "Invalid canonical launch donor: BL-097"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := strings.Replace(backlog, tc.old, tc.replacement, 1)
			if mutated == backlog {
				t.Fatal("fixture mutation did not change the backlog")
			}
			expectBindingFinding(t, checkAdapterBindings(plan, mutated), tc.detail)
		})
	}
	t.Run("unclassified donor", func(t *testing.T) {
		mutated := strings.Replace(plan, "**Applicable owners** BL-097, BL-164", "**Applicable owners** BL-097", 1)
		expectBindingFinding(t, checkAdapterBindings(mutated, backlog), "Invalid canonical launch donor: BL-164")
	})
}

func expectBindingFinding(t *testing.T, findings []finding, detail string) {
	t.Helper()
	for _, result := range findings {
		if result.Rule == "ADAPTER_BINDING" && strings.Contains(result.Detail, detail) {
			return
		}
	}
	t.Fatalf("want %q, got %+v", detail, findings)
}
