package main

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const adapterPlan = "docs/planning/tool-adapters.md"

const (
	launchReceiver        = "BL-151"
	launchTransferLink    = "[Launch transfer to BL-151](docs/planning/implementation-contract.md#managed-process-activation)"
	launchReceiptPrefix   = "Accept transferred launch obligations from "
	adapterInventoryStart = "<!-- public-tool-inventory:v1 -->"
	adapterInventoryEnd   = "<!-- /public-tool-inventory -->"
	adapterContractStart  = "<!-- adapter-acceptance-contract:v1 -->"
	adapterContractEnd    = "<!-- /adapter-acceptance-contract -->"
	adapterFamilyPrefix   = "adapter-acceptance-"
)

var adapterOwnerID = regexp.MustCompile(`^BL-[0-9]{3}$`)
var adapterAnchor = regexp.MustCompile(`^<a id="([a-z0-9-]+)"></a>$`)
var publicToolName = regexp.MustCompile("^`([a-z][a-z0-9_]*)`$")

var adapterFamilyKeys = []string{
	"filesystem", "search", "operations", "context-and-policy", "managed-execution",
	"system-and-platform", "host-and-transport", "native-tools", "provider-boundary",
}

var adapterEvidence = map[string]string{
	"authorization":     "Require effective authorization before dispatch and deny fallback for unauthorized requests.",
	"bounded-resources": "Prove bounded resource behavior for the actual operation and failure paths.",
	"gated-activation":  "Keep unproved routes disabled or return a safe denial before activation.",
	"native-platform":   "Exercise the actual native backend on each applicable platform.",
	"protected-effects": "Deny protected reads, mutations and disclosure across direct and delegated effects.",
}

type adapterOwner struct {
	line       int
	status     string
	targets    []string
	acceptance string
}

func (o adapterOwner) active() bool {
	return o.status == "Planned" || o.status == "In Progress" || o.status == "Blocked"
}

func (o adapterOwner) launchTransfer() bool {
	return strings.Contains(o.acceptance, launchTransferLink)
}

func checkAdapterBindings(plan, backlog string) []finding {
	planLines := adapterPlanLines(plan)
	var findings []finding
	add := func(path string, line int, detail string) {
		findings = append(findings, finding{Path: path, Line: line, Rule: "ADAPTER_BINDING", Detail: detail})
	}
	links, expected := successorApplicability(planLines, func(line int, detail string) {
		add(adapterPlan, line, detail)
	})
	owners := map[string]adapterOwner{}
	duplicate := map[string]bool{}
	for i, line := range documentLines(backlog, false) {
		fields := strings.SplitN(line, "|", 5)
		if len(fields) != 5 || fields[0] != "" {
			continue
		}
		id := strings.TrimSpace(fields[1])
		acceptance := stripInlineCode(fields[4])
		mentionsLaunch := strings.Contains(strings.ToLower(acceptance), "[launch transfer") || strings.Contains(acceptance, launchReceiptPrefix)
		if !adapterOwnerID.MatchString(id) {
			if strings.HasPrefix(id, "BL-") && mentionsLaunch {
				add("BACKLOG.md", i+1, "Invalid canonical owner ID for launch acceptance: "+id)
			}
			continue
		}
		if _, exists := owners[id]; exists {
			duplicate[id] = true
			if mentionsLaunch {
				add("BACKLOG.md", i+1, "Launch transfer or receipt in a duplicate backlog row: "+id)
			}
			continue
		}
		owner := adapterOwner{
			line:       i + 1,
			status:     strings.Trim(strings.TrimSpace(fields[2]), "*"),
			acceptance: acceptance,
		}
		for _, target := range inlineDestinations(acceptance) {
			if fragment, local := strings.CutPrefix(target, adapterPlan); local && links[fragment] != "" {
				owner.targets = append(owner.targets, links[fragment])
			}
		}
		owners[id] = owner
		if owner.active() && len(owner.targets) != 0 && expected[id] == "" {
			add("BACKLOG.md", i+1, "Adapter acceptance has no matching applicability declaration: "+id)
		}
	}

	ids := make([]string, 0, len(expected))
	for id := range expected {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		owner, exists := owners[id]
		switch {
		case !exists || duplicate[id]:
			add("BACKLOG.md", owner.line, "Adapter applicability needs exactly one canonical backlog row: "+id)
		case owner.status == "Completed":
			// Retained historical rows do not acquire a new acceptance duty.
		case !owner.active():
			add("BACKLOG.md", owner.line, "Adapter applicability needs an active or completed owner: "+id)
		case len(owner.targets) != 1 || owner.targets[0] != expected[id]:
			add("BACKLOG.md", owner.line, "Expected exactly one adapter acceptance link to "+expected[id]+" for "+id)
		}
	}
	checkLaunchTransfers(owners, duplicate, expected, func(line int, detail string) {
		add("BACKLOG.md", line, detail)
	})
	findings = append(findings, checkPublicAdapterInventory(planLines, owners, duplicate, expected, links)...)
	return findings
}

func adapterPlanLines(plan string) []string {
	lines := documentLines(plan, false)
	inComment := false
	for i, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
		lines[i] = line
		if inComment {
			lines[i] = ""
			inComment = !strings.Contains(line, "-->")
		} else if _, after, opens := strings.Cut(line, "<!--"); opens && !strings.Contains(after, "-->") {
			lines[i] = ""
			inComment = true
		}
	}
	return lines
}

// The canonical receiver lists the accepted donors. Compare both directions
// independently of the public inventory so internal work cannot be orphaned.
func checkLaunchTransfers(owners map[string]adapterOwner, duplicate map[string]bool, expected map[string]string, add func(int, string)) {
	receiver, exists := owners[launchReceiver]
	receiverValid := exists && !duplicate[launchReceiver] && expected[launchReceiver] != "" &&
		(receiver.active() || receiver.status == "Completed")
	var received map[string]bool
	if exists || expected[launchReceiver] != "" {
		if !receiverValid {
			add(receiver.line, "Launch acceptance requires one active or completed BL-151 owner with adapter applicability.")
		} else {
			var err error
			received, err = launchReceipt(receiver.acceptance)
			if err != nil {
				add(receiver.line, err.Error())
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(owners)) {
		owner := owners[id]
		if id != launchReceiver && strings.Contains(owner.acceptance, launchReceiptPrefix) {
			add(owner.line, "Only BL-151 may receive managed-process launch acceptance: "+id)
		}
		markers := strings.Count(strings.ToLower(owner.acceptance), "[launch transfer")
		if markers == 0 && !received[id] {
			continue
		}
		if id == launchReceiver || duplicate[id] || expected[id] == "" || (!owner.active() && owner.status != "Completed") {
			add(owner.line, "Invalid canonical launch donor: "+id)
			continue
		}
		_, after, linked := strings.Cut(owner.acceptance, launchTransferLink)
		switch {
		case markers == 0:
			add(owner.line, "BL-151 receipt has no matching launch transfer from "+id)
		case markers != 1 || !linked || !strings.HasPrefix(after, ". BL-151 accepts "):
			add(owner.line, "Expected one consistent launch transfer to BL-151 from "+id)
		case !receiverValid:
			add(owner.line, "Launch transfer has no valid BL-151 receiver: "+id)
		case received != nil && !received[id]:
			add(owner.line, "BL-151 receipt omits launch donor "+id)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(received)) {
		if _, exists := owners[id]; !exists {
			add(receiver.line, "BL-151 receipt names an unknown canonical donor: "+id)
		}
	}
}

func launchReceipt(acceptance string) (map[string]bool, error) {
	_, rest, _ := strings.Cut(acceptance, launchReceiptPrefix)
	list, _, terminated := strings.Cut(rest, ".")
	if strings.Count(acceptance, launchReceiptPrefix) != 1 || !terminated {
		return nil, fmt.Errorf("BL-151 needs exactly one launch acceptance receipt")
	}
	donors := map[string]bool{}
	for _, span := range strings.Split(list, ",") {
		first, last, valid := adapterOwnerBounds(span)
		if !valid {
			return nil, fmt.Errorf("invalid donor ID or range in BL-151 receipt: %s", strings.TrimSpace(span))
		}
		for n := first; n <= last; n++ {
			id := fmt.Sprintf("BL-%03d", n)
			if donors[id] {
				return nil, fmt.Errorf("duplicate launch donor in BL-151 receipt: %s", id)
			}
			donors[id] = true
		}
	}
	return donors, nil
}

// The inventory declares the applicable policy for the public surface.
// Compare that classification with the independently declared owner scope.
func checkPublicAdapterInventory(lines []string, owners map[string]adapterOwner, duplicate map[string]bool, expected, familyLinks map[string]string) []finding {
	var findings []finding
	add := func(line int, detail string) {
		findings = append(findings, finding{Path: adapterPlan, Line: line, Rule: "ADAPTER_BINDING", Detail: detail})
	}
	inInventory := false
	starts, ends, rowCount := 0, 0, 0
	names := map[string]bool{}
	for i, line := range lines {
		switch line {
		case adapterInventoryStart:
			starts++
			inInventory = true
			continue
		case adapterInventoryEnd:
			ends++
			if !inInventory {
				add(i+1, "Public tool inventory closes before it opens.")
			}
			inInventory = false
			continue
		}
		if !inInventory || !strings.HasPrefix(line, "|") {
			continue
		}
		if strings.HasPrefix(line, "| Family |") || strings.HasPrefix(line, "|---") {
			continue
		}
		rowCount++
		fields := strings.Split(line, "|")
		if len(fields) != 8 {
			add(i+1, "Invalid public tool inventory row.")
			continue
		}
		for _, name := range strings.Split(strings.TrimSpace(fields[2]), ", ") {
			match := publicToolName.FindStringSubmatch(name)
			if match == nil {
				add(i+1, "Invalid public tool name in inventory: "+name)
			} else if names[match[1]] {
				add(i+1, "Duplicate public tool name in inventory: "+match[1])
			} else {
				names[match[1]] = true
			}
		}
		id, activationID, staged := strings.Cut(strings.TrimSpace(fields[3]), " / activation ")
		owner, exists := owners[id]
		if !exists || duplicate[id] {
			add(i+1, "Public tool family needs exactly one canonical backlog owner: "+id)
			continue
		}
		_, state, _ := strings.Cut(strings.TrimSpace(fields[4]), " / ")
		if state != "IMPLEMENTED" && state != "PLANNED" && state != "CANDIDATE" {
			add(i+1, "Invalid public tool inventory state.")
			continue
		}
		if owner.launchTransfer() != staged || (staged && activationID != launchReceiver) {
			add(i+1, "Public tool row must match its BL-151 launch transfer: "+id)
			continue
		}
		if staged {
			activation, exists := owners[activationID]
			if !exists || duplicate[activationID] || id == activationID ||
				expected[id] == "" || expected[id] != expected[activationID] ||
				(!owner.active() && owner.status != "Completed") {
				add(i+1, "Invalid staged activation owner for public tool row: "+id)
				continue
			}
			if state == "IMPLEMENTED" && (owner.status != "Completed" ||
				(activation.status != "In Progress" && activation.status != "Completed")) {
				add(i+1, "Staged public tool cannot be implemented before activation: "+id)
				continue
			}
			// A completed foundation may retain a planned public surface.
			owner = activation
		}
		acceptance := strings.TrimSpace(fields[6])
		if owner.status == "Completed" {
			switch {
			case state != "IMPLEMENTED":
				add(i+1, "Planned public tool family needs an active backlog owner: "+id)
				continue
			case acceptance == "—":
				continue
			}
		} else if !owner.active() {
			add(i+1, "Public tool family cannot use a rejected or inactive owner: "+id)
			continue
		}
		links := inlineDestinations(stripInlineCode(acceptance))
		if expected[id] == "" || len(links) != 1 || familyLinks[links[0]] != expected[id] {
			add(i+1, "Public tool row and canonical owner have different adapter applicability: "+id)
		}
	}
	if starts != 1 || ends != 1 || inInventory || rowCount == 0 {
		add(0, "Exactly one complete public tool inventory must be present.")
	}
	return findings
}

func successorApplicability(lines []string, add func(int, string)) (map[string]string, map[string]string) {
	links := map[string]string{}
	expected := map[string]string{}
	declared := map[string]bool{}
	anchors := map[string]int{}
	for _, line := range lines {
		for _, anchor := range explicitAnchor.FindAllStringSubmatch(line, -1) {
			anchors[anchor[1]]++
		}
	}
	inContract := false
	starts, ends := 0, 0
	evidence := map[string]string{}
	current := ""
	for i, line := range lines {
		switch line {
		case adapterContractStart:
			starts++
			inContract = true
			current = ""
			continue
		case adapterContractEnd:
			ends++
			if !inContract {
				add(i+1, "Adapter acceptance contract closes before it opens.")
			}
			inContract = false
			continue
		}
		if !inContract {
			continue
		}
		if value, ok := strings.CutPrefix(line, "- **"); ok && current == "" {
			key, outcome, valid := strings.Cut(value, "**: ")
			if valid && adapterEvidence[key] != "" {
				if _, duplicate := evidence[key]; duplicate {
					add(i+1, "Duplicate adapter acceptance evidence: "+key)
				}
				evidence[key] = outcome
				continue
			}
		}
		if anchor := adapterAnchor.FindStringSubmatch(line); anchor != nil {
			id := anchor[1]
			link := "#" + id
			if anchors[id] != 1 || links[link] != "" {
				add(i+1, "Duplicate adapter acceptance anchor: "+link)
				continue
			}
			if key, family := strings.CutPrefix(id, adapterFamilyPrefix); family {
				current = ""
				if !slices.Contains(adapterFamilyKeys, key) {
					add(i+1, "Unknown adapter acceptance family: "+key)
					continue
				}
				current = link
				links[link] = current
				continue
			}
			if current == "" || declared[current] {
				add(i+1, "Compatibility anchor must precede applicability in its adapter family.")
				continue
			}
			if legacyFamily, legacy := strings.CutPrefix(id, "bl-220-"); legacy && current != "#"+adapterFamilyPrefix+legacyFamily {
				add(i+1, "Compatibility anchor points to a different adapter acceptance family: "+link)
				continue
			}
			links[link] = current
			continue
		}
		owners, declaration := strings.CutPrefix(line, "**Applicable owners** ")
		if !declaration {
			continue
		}
		if current == "" || declared[current] {
			add(i+1, "Adapter applicability must be declared once inside a successor family.")
			continue
		}
		declared[current] = true
		for _, span := range strings.Split(owners, ",") {
			from, to, valid := adapterOwnerBounds(span)
			if !valid {
				add(i+1, "Invalid BL ID or inclusive range in adapter applicability: "+span)
				continue
			}
			for n := from; n <= to; n++ {
				id := fmt.Sprintf("BL-%03d", n)
				if expected[id] != "" {
					add(i+1, "Multiple adapter applicability declarations for "+id)
					continue
				}
				expected[id] = current
			}
		}
	}
	if starts != 1 || ends != 1 || inContract {
		add(0, "Exactly one complete adapter acceptance contract is required.")
	}
	for _, key := range slices.Sorted(maps.Keys(adapterEvidence)) {
		if evidence[key] != adapterEvidence[key] {
			add(0, "Missing or incorrect adapter acceptance evidence: "+key)
		}
	}
	for _, key := range adapterFamilyKeys {
		family := "#" + adapterFamilyPrefix + key
		if !declared[family] {
			add(0, "Missing adapter applicability declaration for "+family)
		}
	}
	return links, expected
}

func adapterOwnerBounds(span string) (int, int, bool) {
	first, last, ranged := strings.Cut(strings.TrimSpace(span), "–")
	if !ranged {
		last = first
	}
	if !adapterOwnerID.MatchString(first) || !adapterOwnerID.MatchString(last) {
		return 0, 0, false
	}
	from, _ := strconv.Atoi(first[3:])
	to, _ := strconv.Atoi(last[3:])
	return from, to, from != 0 && from <= to
}
