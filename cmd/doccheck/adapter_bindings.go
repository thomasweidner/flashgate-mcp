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
	launchReceiver      = "BL-151"
	launchTransferLink  = "[Launch transfer to BL-151](docs/planning/implementation-contract.md#managed-process-activation)"
	launchReceiptPrefix = "Accept transferred launch obligations from "
)

var adapterOwnerID = regexp.MustCompile(`^BL-[0-9]{3}$`)

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
	planLines := documentLines(plan, false)
	var findings []finding
	add := func(path string, line int, detail string) {
		findings = append(findings, finding{Path: path, Line: line, Rule: "ADAPTER_BINDING", Detail: detail})
	}
	families, expected := successorApplicability(planLines, func(line int, detail string) {
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
			if fragment, local := strings.CutPrefix(target, adapterPlan); local && families[fragment] {
				owner.targets = append(owner.targets, fragment)
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
	findings = append(findings, checkPublicAdapterInventory(planLines, owners, duplicate, expected)...)
	return findings
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
func checkPublicAdapterInventory(lines []string, owners map[string]adapterOwner, duplicate map[string]bool, expected map[string]string) []finding {
	var findings []finding
	add := func(line int, detail string) {
		findings = append(findings, finding{Path: adapterPlan, Line: line, Rule: "ADAPTER_BINDING", Detail: detail})
	}
	inInventory := false
	rowCount := 0
	for i, line := range lines {
		switch {
		case line == "## 20. Public tool inventory by target":
			inInventory = true
		case strings.HasPrefix(line, "## "):
			inInventory = false
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
		if expected[id] == "" || len(links) != 1 || links[0] != expected[id] {
			add(i+1, "Public tool row and canonical owner have different adapter applicability: "+id)
		}
	}
	if rowCount == 0 {
		add(0, "Public tool inventory must be present.")
	}
	return findings
}

func successorApplicability(lines []string, add func(int, string)) (map[string]bool, map[string]string) {
	families := map[string]bool{}
	expected := map[string]string{}
	inSuccessors := false
	current := ""
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			inSuccessors = line == "## BL-220 successor acceptance"
			current = ""
		}
		if !inSuccessors {
			continue
		}
		if strings.HasPrefix(line, "### ") {
			current = ""
			if !strings.HasPrefix(line, "### BL-220 ") {
				continue
			}
			for anchor := range headingAnchors(line) {
				current = "#" + anchor
				if _, exists := families[current]; exists {
					add(i+1, "Duplicate adapter acceptance family: "+current)
				}
				families[current] = false
			}
			continue
		}
		owners, declaration := strings.CutPrefix(line, "**Applicable owners** ")
		if !declaration {
			continue
		}
		if current == "" || families[current] {
			add(i+1, "Adapter applicability must be declared once inside a successor family.")
			continue
		}
		families[current] = true
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
	if len(families) == 0 || len(expected) == 0 {
		add(0, "BL-220 successor families and applicable owners must be present.")
	}
	missing := make([]string, 0)
	for family, declared := range families {
		if !declared {
			missing = append(missing, family)
		}
	}
	slices.Sort(missing)
	for _, family := range missing {
		add(0, "Missing adapter applicability declaration for "+family)
	}
	return families, expected
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
